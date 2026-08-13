// Package handlers 线路探测策略参数化（#9a）：
// 把探测间隔 / 失败阈值 / 容差 / 并发上限四项参数持久化到 SystemConfig，
// 并通过 linereg.Manager.LoadProbeConfig() 在启动时与更新后应用。
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/linereg"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// 探测参数在 SystemConfig 中的键名。
const (
	cfgKeyIntervalSec      = "probe_interval_sec"
	cfgKeyFailureThreshold = "probe_failure_threshold"
	cfgKeyToleranceMs      = "probe_tolerance_ms"
	cfgKeyMaxConcurrent    = "probe_max_concurrent"
)

// 探测参数默认值（与 linereg.DefaultInterval / selector 默认一致）。
const (
	defaultIntervalSec      = 60
	defaultFailureThreshold = 2
	defaultToleranceMs      = 50
	defaultMaxConcurrent    = 8
)

// 校验范围（含上下界）。
const (
	minIntervalSec      = 5
	maxIntervalSec      = 3600
	minFailureThreshold = 1
	maxFailureThreshold = 10
	minToleranceMs      = 0
	maxToleranceMs      = 5000
	minMaxConcurrent    = 1
	maxMaxConcurrent    = 64
)

// LineregHandler 线路探测策略处理器。
type LineregHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *linereg.Manager
}

// NewLineregHandler 创建线路探测策略处理器。
func NewLineregHandler(db *gorm.DB, log *logrus.Logger, mgr *linereg.Manager) *LineregHandler {
	return &LineregHandler{db: db, log: log, mgr: mgr}
}

// probeConfigResponse 前端回填用响应体。
type probeConfigResponse struct {
	IntervalSec      int `json:"interval_sec"`
	FailureThreshold int `json:"failure_threshold"`
	ToleranceMs      int `json:"tolerance_ms"`
	MaxConcurrent    int `json:"max_concurrent"`
}

// probeConfigRequest 前端提交的配置更新请求体。
type probeConfigRequest struct {
	IntervalSec      int `json:"interval_sec"`
	FailureThreshold int `json:"failure_threshold"`
	ToleranceMs      int `json:"tolerance_ms"`
	MaxConcurrent    int `json:"max_concurrent"`
}

// getConfigInt 从 SystemConfig 读取一个整型参数；缺失或解析失败时返回 def。
func getConfigInt(db *gorm.DB, key string, def int) int {
	var cfg model.SystemConfig
	if err := db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return def
	}
	v, err := strconv.Atoi(cfg.Value)
	if err != nil {
		return def
	}
	return v
}

// GetConfig 读取探测策略四项参数（无记录返回默认值）。
func (h *LineregHandler) GetConfig(c *gin.Context) {
	resp := probeConfigResponse{
		IntervalSec:      getConfigInt(h.db, cfgKeyIntervalSec, defaultIntervalSec),
		FailureThreshold: getConfigInt(h.db, cfgKeyFailureThreshold, defaultFailureThreshold),
		ToleranceMs:      getConfigInt(h.db, cfgKeyToleranceMs, defaultToleranceMs),
		MaxConcurrent:    getConfigInt(h.db, cfgKeyMaxConcurrent, defaultMaxConcurrent),
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

// setConfigUpsert 写入（或更新）一条 SystemConfig 整型参数。
func setConfigUpsert(db *gorm.DB, key string, value int) {
	var cfg model.SystemConfig
	if err := db.Where("key = ?", key).First(&cfg).Error; err == nil {
		cfg.Value = strconv.Itoa(value)
		db.Save(&cfg)
		return
	}
	db.Create(&model.SystemConfig{Key: key, Value: strconv.Itoa(value)})
}

// UpdateConfig 校验并写入探测策略四项参数，随后应用（透传到 selector）。
func (h *LineregHandler) UpdateConfig(c *gin.Context) {
	var req probeConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	// 范围校验
	if req.IntervalSec < minIntervalSec || req.IntervalSec > maxIntervalSec {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "interval_sec out of range"})
		return
	}
	if req.FailureThreshold < minFailureThreshold || req.FailureThreshold > maxFailureThreshold {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "failure_threshold out of range"})
		return
	}
	if req.ToleranceMs < minToleranceMs || req.ToleranceMs > maxToleranceMs {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "tolerance_ms out of range"})
		return
	}
	if req.MaxConcurrent < minMaxConcurrent || req.MaxConcurrent > maxMaxConcurrent {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "max_concurrent out of range"})
		return
	}
	// 持久化
	setConfigUpsert(h.db, cfgKeyIntervalSec, req.IntervalSec)
	setConfigUpsert(h.db, cfgKeyFailureThreshold, req.FailureThreshold)
	setConfigUpsert(h.db, cfgKeyToleranceMs, req.ToleranceMs)
	setConfigUpsert(h.db, cfgKeyMaxConcurrent, req.MaxConcurrent)
	// 立即应用（间隔需在下一轮生效，其余透传到 selector）
	h.mgr.SetInterval(time.Duration(req.IntervalSec) * time.Second)
	h.mgr.SetFailureThreshold(req.FailureThreshold)
	h.mgr.SetTolerance(time.Duration(req.ToleranceMs) * time.Millisecond)
	h.mgr.SetMaxConcurrent(req.MaxConcurrent)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已更新"})
}
