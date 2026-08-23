package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/waf"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== WAF 防火墙 =====

type WafHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewWafHandler(db *gorm.DB, log *logrus.Logger) *WafHandler {
	return &WafHandler{db: db, log: log}
}

func (h *WafHandler) List(c *gin.Context) {
	var configs []model.WafConfig
	h.db.Order("id desc").Find(&configs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *WafHandler) Create(c *gin.Context) {
	var cfg model.WafConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	logger.WriteLog("info", "waf", fmt.Sprintf("创建WAF配置 [%d] %s", cfg.ID, cfg.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *WafHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WafConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "waf", fmt.Sprintf("更新WAF配置 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WafHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.WafConfig{}, id)
	h.db.Where("waf_config_id = ?", id).Delete(&model.WafLog{})
	logger.WriteLog("info", "waf", fmt.Sprintf("删除WAF配置 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WafHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cfg model.WafConfig
	if err := h.db.First(&cfg, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "WAF 配置不存在"})
		return
	}
	if waf.Default == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "WAF 引擎未初始化"})
		return
	}
	if err := waf.Default.Start(uint(id)); err != nil {
		h.log.Errorf("[WAF] 启动失败: id=%d err=%v", id, err)
		h.db.Model(&model.WafConfig{}).Where("id = ?", id).Updates(map[string]any{
			"enable":     false,
			"status":     "error",
			"last_error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.log.Infof("[WAF] 启动: id=%d name=%s", id, cfg.Name)
	logger.WriteLog("info", "waf", fmt.Sprintf("启动WAF [%d] %s", cfg.ID, cfg.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *WafHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if waf.Default == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "WAF 引擎未初始化"})
		return
	}
	if err := waf.Default.Stop(uint(id)); err != nil {
		h.log.Errorf("[WAF] 停止失败: id=%d err=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.log.Infof("[WAF] 停止: id=%d", id)
	logger.WriteLog("info", "waf", fmt.Sprintf("停止WAF [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *WafHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	var logs []model.WafLog
	var total int64
	h.db.Model(&model.WafLog{}).Where("waf_config_id = ?", id).Count(&total)
	h.db.Where("waf_config_id = ?", id).
		Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

func (h *WafHandler) TestRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Rule string `json:"rule"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if waf.Default == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "WAF 引擎未初始化"})
		return
	}
	if err := waf.Default.TestRule(req.Rule); err != nil {
		h.log.Errorf("[WAF] 规则校验失败: id=%d err=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("规则语法错误: %v", err)})
		return
	}

	h.log.Infof("[WAF] 测试规则: id=%d rule=%s", id, req.Rule)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "规则语法正确"})
}
