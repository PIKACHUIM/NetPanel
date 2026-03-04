package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/firewall"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== 系统防火墙 =====

// FirewallHandler 防火墙规则处理器
type FirewallHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *firewall.Manager
}

// NewFirewallHandler 创建防火墙处理器
func NewFirewallHandler(db *gorm.DB, log *logrus.Logger, mgr *firewall.Manager) *FirewallHandler {
	return &FirewallHandler{db: db, log: log, mgr: mgr}
}

// List 获取防火墙规则列表
func (h *FirewallHandler) List(c *gin.Context) {
	var rules []model.FirewallRule
	h.db.Order("priority asc, id asc").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rules})
}

// Create 创建防火墙规则
func (h *FirewallHandler) Create(c *gin.Context) {
	var rule model.FirewallRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	rule.ApplyStatus = "pending"
	if err := h.db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}
	// 若启用则立即应用
	if rule.Enable {
		if err := h.mgr.ApplyRule(&rule); err != nil {
			h.log.Warnf("[Firewall] 创建后应用规则失败: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rule, "message": "创建成功"})
}

// Update 更新防火墙规则
func (h *FirewallHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var old model.FirewallRule
	if err := h.db.First(&old, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "规则不存在"})
		return
	}
	var req model.FirewallRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	// 先删除旧规则（忽略错误，可能未应用）
	if old.Enable {
		_ = h.mgr.RemoveRule(&old)
	}
	req.ID = uint(id)
	req.ApplyStatus = "pending"
	h.db.Save(&req)
	// 若启用则重新应用
	if req.Enable {
		if err := h.mgr.ApplyRule(&req); err != nil {
			h.log.Warnf("[Firewall] 更新后应用规则失败: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

// Delete 删除防火墙规则
func (h *FirewallHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var rule model.FirewallRule
	if err := h.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "规则不存在"})
		return
	}
	// 先从系统防火墙删除
	if rule.Enable {
		_ = h.mgr.RemoveRule(&rule)
	}
	h.db.Delete(&model.FirewallRule{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// Apply 手动应用规则到系统防火墙
func (h *FirewallHandler) Apply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var rule model.FirewallRule
	if err := h.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "规则不存在"})
		return
	}
	if err := h.mgr.ApplyRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "应用失败: " + err.Error()})
		return
	}
	// 同步启用状态
	h.db.Model(&model.FirewallRule{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "规则已应用"})
}

// Remove 从系统防火墙移除规则（不删除数据库记录）
func (h *FirewallHandler) Remove(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var rule model.FirewallRule
	if err := h.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "规则不存在"})
		return
	}
	if err := h.mgr.RemoveRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "移除失败: " + err.Error()})
		return
	}
	h.db.Model(&model.FirewallRule{}).Where("id = ?", id).Updates(map[string]any{
		"enable":       false,
		"apply_status": "pending",
	})
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "规则已移除"})
}

// DetectBackend 检测当前系统防火墙后端
func (h *FirewallHandler) DetectBackend(c *gin.Context) {
	backend := h.mgr.DetectBackend()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"backend": backend}})
}

// SyncSystem 触发异步同步系统防火墙规则到数据库
func (h *FirewallHandler) SyncSystem(c *gin.Context) {
	h.mgr.SyncSystemRulesAsync()
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "同步任务已触发，正在后台执行"})
}

// GetSyncStatus 获取当前同步状态
func (h *FirewallHandler) GetSyncStatus(c *gin.Context) {
	status := h.mgr.GetSyncStatus()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": status})
}
