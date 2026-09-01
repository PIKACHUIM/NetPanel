package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

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

// tunnelResponse 响应体（Token 不回显，仅返回是否已配置）
type tunnelResponse struct {
	ID              uint   `json:"id"`
	Name            string `json:"name"`
	Mode            string `json:"mode"`
	Enable          bool   `json:"enable"`
	LocalURL        string `json:"local_url"`
	TunnelName      string `json:"tunnel_name"`
	CredentialsFile string `json:"credentials_file"`
	ConfigFile      string `json:"config_file"`
	Protocol        string `json:"protocol"`
	QuickURL        string `json:"quick_url"`
	Status          string `json:"status"`
	LastError       string `json:"last_error"`
	Remark          string `json:"remark"`
	CreatedAt       string `json:"created_at"`
	HasToken        bool   `json:"has_token"`
}

func toTunnelResponse(t model.CftunnelConfig) tunnelResponse {
	return tunnelResponse{
		ID:              t.ID,
		Name:            t.Name,
		Mode:            t.Mode,
		Enable:          t.Enable,
		LocalURL:        t.LocalURL,
		TunnelName:      t.TunnelName,
		CredentialsFile: t.CredentialsFile,
		ConfigFile:      t.ConfigFile,
		Protocol:        t.Protocol,
		QuickURL:        t.QuickURL,
		Status:          t.Status,
		LastError:       t.LastError,
		Remark:          t.Remark,
		CreatedAt:       t.CreatedAt.Format("2006-01-02 15:04:05"),
		HasToken:        t.Token != "",
	}
}

// List 隧道列表
// GET /api/v1/cftunnel
func (h *CfTunnelHandler) List(c *gin.Context) {
	var tunnels []model.CftunnelConfig
	if err := h.db.Order("id desc").Find(&tunnels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败: " + err.Error()})
		return
	}
	resp := make([]tunnelResponse, 0, len(tunnels))
	for _, t := range tunnels {
		// 以进程实际运行状态校正数据库中的状态字段
		if st, _ := h.mgr.GetStatus(t.ID); st != nil {
			if running, _ := st["running"].(bool); running {
				t.Status = "running"
			} else if t.Status == "running" {
				t.Status = "stopped"
			}
		}
		resp = append(resp, toTunnelResponse(t))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

// Create 创建隧道
// POST /api/v1/cftunnel
func (h *CfTunnelHandler) Create(c *gin.Context) {
	var req struct {
		Name            string `json:"name" binding:"required,min=1,max=100"`
		Mode            string `json:"mode" binding:"required,oneof=quick named token"`
		Enable          bool   `json:"enable"`
		LocalURL        string `json:"local_url"`
		Token           string `json:"token"`
		TunnelName      string `json:"tunnel_name"`
		CredentialsFile string `json:"credentials_file"`
		ConfigFile      string `json:"config_file"`
		Protocol        string `json:"protocol"`
		Remark          string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 按模式校验必填项，与 Manager.buildArgs 的要求保持一致
	switch req.Mode {
	case "quick":
		if req.LocalURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "quick 模式需要填写本地服务地址"})
			return
		}
	case "named":
		if req.TunnelName == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "named 模式需要填写隧道名称或 UUID"})
			return
		}
		if err := cftunnel.ValidateTunnelName(req.TunnelName); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
	case "token":
		if req.Token == "" {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "token 模式需要填写 Cloudflare Tunnel Token"})
			return
		}
	}

	protocol := req.Protocol
	if protocol == "" {
		protocol = "http"
	}

	tunnel := model.CftunnelConfig{
		Name:            req.Name,
		Mode:            req.Mode,
		Enable:          req.Enable,
		LocalURL:        req.LocalURL,
		Token:           req.Token,
		TunnelName:      req.TunnelName,
		CredentialsFile: req.CredentialsFile,
		ConfigFile:      req.ConfigFile,
		Protocol:        protocol,
		Status:          "stopped",
		Remark:          req.Remark,
	}
	if err := h.db.Create(&tunnel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("创建CF隧道 [%d] %s (%s)", tunnel.ID, tunnel.Name, tunnel.Mode))
	if req.Enable {
		if err := h.mgr.Start(tunnel.ID); err != nil {
			h.log.Warnf("[CF隧道][%d] 创建后启动失败: %v", tunnel.ID, err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": toTunnelResponse(tunnel), "message": "创建成功"})
}

// Update 更新隧道
// PUT /api/v1/cftunnel/:id
// 采用部分更新语义：字符串字段为空表示不修改，避免前端未提交的字段被清零。
func (h *CfTunnelHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Name            string  `json:"name"`
		Mode            string  `json:"mode"`
		Enable          *bool   `json:"enable"`
		LocalURL        string  `json:"local_url"`
		Token           string  `json:"token"` // 为空则不修改
		TunnelName      string  `json:"tunnel_name"`
		CredentialsFile *string `json:"credentials_file"` // 显式传 "" 可清空
		ConfigFile      *string `json:"config_file"`      // 显式传 "" 可清空
		Protocol        string  `json:"protocol"`
		Remark          *string `json:"remark"` // 显式传 "" 可清空
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	var tunnel model.CftunnelConfig
	if err := h.db.First(&tunnel, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "隧道不存在"})
		return
	}

	if req.Mode != "" && req.Mode != "quick" && req.Mode != "named" && req.Mode != "token" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "模式非法（可选 quick/named/token）"})
		return
	}
	if req.TunnelName != "" {
		if err := cftunnel.ValidateTunnelName(req.TunnelName); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
			return
		}
	}

	updates := map[string]any{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Mode != "" {
		updates["mode"] = req.Mode
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
	if req.TunnelName != "" {
		updates["tunnel_name"] = req.TunnelName
	}
	if req.Protocol != "" {
		updates["protocol"] = req.Protocol
	}
	if req.CredentialsFile != nil {
		updates["credentials_file"] = *req.CredentialsFile
	}
	if req.ConfigFile != nil {
		updates["config_file"] = *req.ConfigFile
	}
	if req.Remark != nil {
		updates["remark"] = *req.Remark
	}

	if len(updates) > 0 {
		if err := h.db.Model(&tunnel).Updates(updates).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
			return
		}
	}
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("更新CF隧道 [%d] %s", tunnel.ID, tunnel.Name))

	if err := h.db.First(&tunnel, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "读取更新结果失败: " + err.Error()})
		return
	}
	// 配置变更后按启用状态重新对齐运行态
	if tunnel.Enable {
		if err := h.mgr.Restart(tunnel.ID); err != nil {
			h.log.Warnf("[CF隧道][%d] 更新后重启失败: %v", tunnel.ID, err)
		}
	} else if err := h.mgr.Stop(tunnel.ID); err != nil {
		h.log.Warnf("[CF隧道][%d] 更新后停止失败: %v", tunnel.ID, err)
	}

	h.db.First(&tunnel, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": toTunnelResponse(tunnel), "message": "更新成功"})
}

// Delete 删除隧道（先停止）
// DELETE /api/v1/cftunnel/:id
func (h *CfTunnelHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.mgr.Stop(id); err != nil {
		h.log.Warnf("[CF隧道][%d] 删除前停止失败: %v", id, err)
	}
	if err := h.db.Delete(&model.CftunnelConfig{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("删除CF隧道 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// Start 启动隧道
// POST /api/v1/cftunnel/:id/start
func (h *CfTunnelHandler) Start(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.mgr.Start(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("启动CF隧道 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

// Stop 停止隧道
// POST /api/v1/cftunnel/:id/stop
func (h *CfTunnelHandler) Stop(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.mgr.Stop(id); err != nil {
		h.log.Warnf("[CF隧道][%d] 停止失败: %v", id, err)
	}
	h.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("停止CF隧道 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// GetStatus 获取隧道状态
// GET /api/v1/cftunnel/:id/status
func (h *CfTunnelHandler) GetStatus(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	st, err := h.mgr.GetStatus(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "隧道不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": st})
}

// GetLogs 获取隧道日志
// GET /api/v1/cftunnel/:id/logs
func (h *CfTunnelHandler) GetLogs(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	logs := h.mgr.GetLogs(id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"logs": logs}})
}

// GetBinaryPath 返回 cloudflared 二进制路径（存在时返回路径，否则为空）
// GET /api/v1/cftunnel/binary
func (h *CfTunnelHandler) GetBinaryPath(c *gin.Context) {
	path := ""
	if h.mgr.IsBinaryAvailable() {
		path = h.mgr.GetBinaryPath()
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"binary_path": path}})
}

// GetDownloadInfo 获取 cloudflared 下载信息
// GET /api/v1/cftunnel/download/info
func (h *CfTunnelHandler) GetDownloadInfo(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cftunnel.GetDownloadInfo()})
}

// DownloadBinary 下载 cloudflared 二进制（SSE 流式进度）
// POST /api/v1/cftunnel/download
func (h *CfTunnelHandler) DownloadBinary(c *gin.Context) {
	if !cftunnel.IsBinaryDownloadSupported() {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "当前平台不支持自动下载"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "不支持流式传输"})
		return
	}

	// 进度事件统一用 json 编码，避免手工拼接字符串导致的转义问题
	writeEvent := func(payload gin.H) {
		b, err := json.Marshal(payload)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", b)
		flusher.Flush()
	}

	progressCallback := func(downloaded, total int64) {
		percent := float64(0)
		if total > 0 {
			percent = float64(downloaded) / float64(total) * 100
		}
		writeEvent(gin.H{"downloaded": downloaded, "total": total, "percent": percent})
	}

	finalPath, err := cftunnel.DownloadBinary(h.mgr.GetBinDir(), progressCallback)
	if err != nil {
		writeEvent(gin.H{"error": err.Error()})
		logger.WriteLog("error", "cftunnel", fmt.Sprintf("下载 cloudflared 失败: %v", err))
		return
	}

	writeEvent(gin.H{"done": true, "path": finalPath})
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("下载 cloudflared 成功: %s", finalPath))
}
