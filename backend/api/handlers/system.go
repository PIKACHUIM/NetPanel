package handlers

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/pkg/logger"
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

// updatableConfigKeys 允许经 PUT /system/config 写入的键白名单。
// 该接口此前无白名单，任意登录用户可写任意键（包括 admin_password），
// 曾构成提权路径；现仅放行界面偏好类键，且路由已收入 admin 组。
var updatableConfigKeys = map[string]bool{
	"language":                true,
	"theme":                   true,
	"speedtest_popup_enabled": true,
}

// UpdateConfig 更新系统配置（仅管理员，键白名单见 updatableConfigKeys）
func (h *SystemHandler) UpdateConfig(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	var keys []string
	for key := range req {
		if !updatableConfigKeys[key] {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": fmt.Sprintf("不允许修改配置键 %q（可修改: language, theme, speedtest_popup_enabled）", key),
			})
			return
		}
	}
	for key, value := range req {
		if err := h.db.Model(&model.SystemConfig{}).Where("key = ?", key).Update("value", value).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "配置写入失败"})
			return
		}
		keys = append(keys, key)
	}
	logger.WriteLog("info", "system", fmt.Sprintf("更新系统配置: %s", strings.Join(keys, ", ")))
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

	// 获取当前登录用户名
	username := c.GetString("username")

	// 优先从 User 表验证并更新密码
	var user model.User
	if err := h.db.Where("username = ?", username).First(&user).Error; err == nil {
		// User 表中存在该用户，验证旧密码
		if !utils.CheckPassword(req.OldPassword, user.Password) {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "旧密码错误"})
			return
		}
		// 加密新密码
		hashed, err := utils.HashPassword(req.NewPassword)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "密码加密失败"})
			return
		}
		// 更新 User 表中的密码
		if err := h.db.Model(&user).Update("password", hashed).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "密码更新失败"})
			return
		}
		logger.WriteLog("info", "system", fmt.Sprintf("用户 %s 修改了密码", username))
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "密码修改成功"})
		return
	}

	// User 表中不存在该用户名（历史版本经 SystemConfig 登录的兼容路径已移除，
	// 能登录进来的用户必然有 User 记录），属于异常状态
	c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "当前用户记录不存在"})
}
