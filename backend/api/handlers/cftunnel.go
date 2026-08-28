package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/cftunnel"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== CF 隧道（Cloudflare Tunnel） =====

type CfTunnelHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *cftunnel.Manager
}

func NewCfTunnelHandler(db *gorm.DB, log *logrus.Logger, mgr *cftunnel.Manager) *CfTunnelHandler {
	return &CfTunnelHandler{db: db, log: log, mgr: mgr}
}

// tunnelResponse 响应体（Token 不回显）
type tunnelResponse struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Enable     bool   `json:"enable"`
	LocalURL   string `json:"local_url"`
	TunnelName string `json:"tunnel_name"`
	Hostname   string `json:"hostname"`
	Status     string `json:"status"`
	PublicURL  string `json:"public_url"`
	LastError  string `json:"last_error"`
	Remark     string `json:"remark"`
	CreatedAt  string `json:"created_at"`
	HasToken   bool   `json:"has_token"`
}

func toTunnelResponse(t model.CloudflareTunnel) tunnelResponse {
	return tunnelResponse{
		ID:         t.ID,
		Name:       t.Name,
		Type:       t.Type,
		Enable:     t.Enable,
		LocalURL:   t.LocalURL,
		TunnelName: t.TunnelName,
		Hostname:   t.Hostname,
		Status:     t.Status,
		PublicURL:  t.PublicURL,
		LastError:  t.LastError,
		Remark:     t.Remark,
		CreatedAt:  t.CreatedAt.Format("2006-01-02 15:04:05"),
		HasToken:   t.Token != "",
	}
}

// List 隧道列表
// GET /api/v1/cftunnel
func (h *CfTunnelHandler) List(c *gin.Context) {
	var tunnels []model.CloudflareTunnel
	h.db.Order("id desc").Find(&tunnels)
	resp := make([]tunnelResponse, 0, len(tunnels))
	for _, t := range tunnels {
		// 实时校正运行状态
		if h.mgr.GetStatus(t.ID) == "running" {
			t.Status = "running"
		}
		resp = append(resp, toTunnelResponse(t))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

// Create 创建隧道
// POST /api/v1/cftunnel
func (h *CfTunnelHandler) Create(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required,min=1,max=100"`
		Type       string `json:"type" binding:"required,oneof=quick named"`
		Enable     bool   `json:"enable"`
		LocalURL   string `json:"local_url"`
		Token      string `json:"token"`
		TunnelName string `json:"tunnel_name"`
		Hostname   string `json:"hostname"`
		Remark     string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if req.Type == "quick" && req.LocalURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "快速隧道需要填写内网目标地址"})
		return
	}
	if req.Type == "named" && req.Token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "命名隧道需要填写 Cloudflare Tunnel Token"})
		return
	}

	tunnel := model.CloudflareTunnel{
		Name:       req.Name,
		Type:       req.Type,
		Enable:     req.Enable,
		LocalURL:   req.LocalURL,
		Token:      req.Token,
		TunnelName: req.TunnelName,
		Hostname:   req.Hostname,
		Status:     "stopped",
		Remark:     req.Remark,
	}
	if err := h.db.Create(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("创建CF隧道 [%d] %s (%s)", tunnel.ID, tunnel.Name, tunnel.Type))
	if req.Enable {
		_ = h.mgr.Start(tunnel.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": toTunnelResponse(tunnel), "message": "创建成功"})
}

// Update 更新隧道
// PUT /api/v1/cftunnel/:id
func (h *CfTunnelHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Enable     *bool  `json:"enable"`
		LocalURL   string `json:"local_url"`
		Token      string `json:"token"` // 为空则不修改
		TunnelName string `json:"tunnel_name"`
		Hostname   string `json:"hostname"`
		Remark     string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	var tunnel model.CloudflareTunnel
	if err := h.db.First(&tunnel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "隧道不存在"})
		return
	}

	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Enable != nil {
		updates["enable"] = *req.Enable
	}
	if req.LocalURL != "" {
		updates["local_url"] = req.LocalURL
	}
	if req.Token != "" {
		updates["token"] = req.Token
	}
	updates["tunnel_name"] = req.TunnelName
	updates["hostname"] = req.Hostname
	updates["remark"] = req.Remark

	if err := h.db.Model(&tunnel).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("更新CF隧道 [%d] %s", tunnel.ID, req.Name))
	h.db.First(&tunnel, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": toTunnelResponse(tunnel), "message": "更新成功"})
}

// Delete 删除隧道（先停止）
// DELETE /api/v1/cftunnel/:id
func (h *CfTunnelHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.CloudflareTunnel{}, id)
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("删除CF隧道 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// Start 启动隧道
// POST /api/v1/cftunnel/:id/start
func (h *CfTunnelHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

// Stop 停止隧道
// POST /api/v1/cftunnel/:id/stop
func (h *CfTunnelHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// GetStatus 获取隧道状态
// GET /api/v1/cftunnel/:id/status
func (h *CfTunnelHandler) GetStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	status := h.mgr.GetStatus(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"status": status}})
}

// GetLogs 获取隧道日志
// GET /api/v1/cftunnel/:id/logs
func (h *CfTunnelHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logs := h.mgr.GetLogs(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"logs": logs}})
}
