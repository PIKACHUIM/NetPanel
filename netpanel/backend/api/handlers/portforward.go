package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/portforward"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// PortForwardHandler 端口转发处理器
type PortForwardHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *portforward.Manager
}

func NewPortForwardHandler(db *gorm.DB, log *logrus.Logger, mgr *portforward.Manager) *PortForwardHandler {
	return &PortForwardHandler{db: db, log: log, mgr: mgr}
}

// List 获取端口转发列表
func (h *PortForwardHandler) List(c *gin.Context) {
	var rules []model.PortForwardRule
	if err := h.db.Order("id desc").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	// 注入运行时状态
	for i := range rules {
		rules[i].Status = h.mgr.GetStatus(rules[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rules})
}

// Create 创建端口转发规则
func (h *PortForwardHandler) Create(c *gin.Context) {
	var rule model.PortForwardRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	rule.Status = "stopped"
	if err := h.db.Create(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if rule.Enable {
		if err := h.mgr.Start(rule.ID); err != nil {
			h.log.Warnf("端口转发 [%d] 自动启动失败: %v", rule.ID, err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rule, "message": "创建成功"})
}

// Update 更新端口转发规则
func (h *PortForwardHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var rule model.PortForwardRule
	if err := h.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "规则不存在"})
		return
	}

	var req model.PortForwardRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	// 先停止
	h.mgr.Stop(uint(id))

	req.ID = uint(id)
	if err := h.db.Save(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	if req.Enable {
		if err := h.mgr.Start(uint(id)); err != nil {
			h.log.Warnf("端口转发 [%d] 重启失败: %v", id, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

// Delete 删除端口转发规则
func (h *PortForwardHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	if err := h.db.Delete(&model.PortForwardRule{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// Start 启动端口转发
func (h *PortForwardHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.PortForwardRule{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

// Stop 停止端口转发
func (h *PortForwardHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.PortForwardRule{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// GetLogs 获取日志
func (h *PortForwardHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logs := h.mgr.GetLogs(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": logs})
}
