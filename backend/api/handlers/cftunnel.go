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

// CftunnelHandler Cloudflare Tunnel 处理器
type CftunnelHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *cftunnel.Manager
}

func NewCftunnelHandler(db *gorm.DB, log *logrus.Logger, mgr *cftunnel.Manager) *CftunnelHandler {
	return &CftunnelHandler{db: db, log: log, mgr: mgr}
}

func (h *CftunnelHandler) List(c *gin.Context) {
	var tunnels []model.CftunnelConfig
	h.db.Order("id desc").Find(&tunnels)
	for i := range tunnels {
		tunnels[i].Status = h.mgr.GetStatus(tunnels[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tunnels})
}

func (h *CftunnelHandler) Create(c *gin.Context) {
	var tunnel model.CftunnelConfig
	if err := c.ShouldBindJSON(&tunnel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if tunnel.Mode == "" {
		tunnel.Mode = "quick"
	}
	tunnel.Status = "stopped"
	h.db.Create(&tunnel)
	if tunnel.Enable {
		h.mgr.Start(tunnel.ID)
	}
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("创建CF隧道 [%d] %s", tunnel.ID, tunnel.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tunnel, "message": "创建成功"})
}

func (h *CftunnelHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CftunnelConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.Stop(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	if req.Enable {
		h.mgr.Start(uint(id))
	}
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("更新CF隧道 [%d] %s", id, req.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CftunnelHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.CftunnelConfig{}, id)
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("删除CF隧道 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CftunnelHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("启动CF隧道 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *CftunnelHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("停止CF隧道 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *CftunnelHandler) GetStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	status := h.mgr.GetStatus(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"status": status}})
}

func (h *CftunnelHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logs := h.mgr.GetLogs(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": logs})
}

func (h *CftunnelHandler) GetBinaryPath(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"binary_path": h.mgr.GetBinaryPath()}})
}

// GetDownloadInfo 获取下载信息
func (h *CftunnelHandler) GetDownloadInfo(c *gin.Context) {
	info := cftunnel.GetDownloadInfo()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": info})
}

// DownloadBinary 下载 cloudflared 二进制
func (h *CftunnelHandler) DownloadBinary(c *gin.Context) {
	if !cftunnel.IsBinaryDownloadSupported() {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "当前平台不支持自动下载"})
		return
	}

	binDir := h.mgr.GetBinDir()
	
	// 使用 Server-Sent Events 报告进度
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "不支持流式传输"})
		return
	}

	// 发送进度回调
	progressCallback := func(downloaded, total int64) {
		percent := float64(0)
		if total > 0 {
			percent = float64(downloaded) / float64(total) * 100
		}
		fmt.Fprintf(c.Writer, "data: {\"downloaded\": %d, \"total\": %d, \"percent\": %.2f}\n\n", downloaded, total, percent)
		flusher.Flush()
	}

	finalPath, err := cftunnel.DownloadBinary(binDir, progressCallback)
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"error\": \"%s\"}\n\n", err.Error())
		flusher.Flush()
		logger.WriteLog("error", "cftunnel", fmt.Sprintf("下载 cloudflared 失败: %v", err))
		return
	}

	fmt.Fprintf(c.Writer, "data: {\"done\": true, \"path\": \"%s\"}\n\n", finalPath)
	flusher.Flush()
	logger.WriteLog("info", "cftunnel", fmt.Sprintf("下载 cloudflared 成功: %s", finalPath))
}
