package handlers

import (
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/pkg/utils"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SystemHandler 系统信息处理器
type SystemHandler struct {
	db     *gorm.DB
	log    *logrus.Logger
	config *config.Config
}

func NewSystemHandler(db *gorm.DB, log *logrus.Logger, cfg *config.Config) *SystemHandler {
	return &SystemHandler{db: db, log: log, config: cfg}
}

// startTime 记录程序启动时间
var startTime = time.Now()

// GetInfo 获取系统信息
func (h *SystemHandler) GetInfo(c *gin.Context) {
	hostname, _ := os.Hostname()
	hostInfo, _ := host.Info()

	uptime := uint64(time.Since(startTime).Seconds())
	if hostInfo != nil && hostInfo.Uptime > 0 {
		uptime = hostInfo.Uptime
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"hostname":   hostname,
			"version":    h.config.Version,
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"go_version": runtime.Version(),
			"uptime":     uptime,
		},
	})
}

// GetStats 获取系统资源统计
func (h *SystemHandler) GetStats(c *gin.Context) {
	// CPU 使用率 & 核心数
	cpuPercent, _ := cpu.Percent(0, false)
	cpuUsage := 0.0
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}
	cpuCores, _ := cpu.Counts(true) // 逻辑核心数

	// 内存使用
	memInfo, _ := mem.VirtualMemory()

	// 交换内存
	swapInfo, _ := mem.SwapMemory()

	// 磁盘使用
	diskInfo, _ := disk.Usage("/")

	data := gin.H{
		"cpu_usage":    cpuUsage,
		"cpu_cores":    cpuCores,
		"mem_total":    memInfo.Total,
		"mem_used":     memInfo.Used,
		"mem_free":     memInfo.Available,
		"mem_percent":  memInfo.UsedPercent,
		"disk_total":   diskInfo.Total,
		"disk_used":    diskInfo.Used,
		"disk_free":    diskInfo.Free,
		"disk_percent": diskInfo.UsedPercent,
	}
	if swapInfo != nil {
		data["swap_total"] = swapInfo.Total
		data["swap_used"] = swapInfo.Used
		data["swap_percent"] = swapInfo.UsedPercent
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": data})
}

// GetConfig 获取系统配置
func (h *SystemHandler) GetConfig(c *gin.Context) {
	var configs []model.SystemConfig
	h.db.Find(&configs)

	result := make(map[string]string)
	for _, cfg := range configs {
		// 不返回密码
		if cfg.Key == "admin_password" {
			continue
		}
		result[cfg.Key] = cfg.Value
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
}

// UpdateConfig 更新系统配置
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	for key, value := range req {
		h.db.Model(&model.SystemConfig{}).Where("key = ?", key).Update("value", value)
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "配置已更新"})
}

// GetInterfaces 获取网络接口列表
func (h *SystemHandler) GetInterfaces(c *gin.Context) {
	interfaces := utils.GetNetInterfaces()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": interfaces})
}

// ChangePassword 修改管理员密码
func (h *SystemHandler) ChangePassword(c *gin.Context) {
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required,min=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	// 查询当前密码
	var cfg model.SystemConfig
	if err := h.db.Where("key = ?", "admin_password").First(&cfg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询密码失败"})
		return
	}

	// 验证旧密码
	if !utils.CheckPassword(req.OldPassword, cfg.Value) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "旧密码错误"})
		return
	}

	// 更新新密码
	hashed, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "密码加密失败"})
		return
	}
	h.db.Model(&model.SystemConfig{}).Where("key = ?", "admin_password").Update("value", hashed)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "密码修改成功"})
}
