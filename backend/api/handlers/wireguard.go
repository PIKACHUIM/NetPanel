package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/wireguard"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== WireGuard 接口管理 =====

type WireguardHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *wireguard.Manager
}

func NewWireguardHandler(db *gorm.DB, log *logrus.Logger, mgr *wireguard.Manager) *WireguardHandler {
	return &WireguardHandler{db: db, log: log, mgr: mgr}
}

func (h *WireguardHandler) List(c *gin.Context) {
	var configs []model.WireguardConfig
	h.db.Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *WireguardHandler) Create(c *gin.Context) {
	var cfg model.WireguardConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	// 如果未提供密钥对，自动生成
	if cfg.PrivateKey == "" {
		privKey, pubKey, err := wireguard.GenerateKeyPair()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成密钥对失败: " + err.Error()})
			return
		}
		cfg.PrivateKey = privKey
		cfg.PublicKey = pubKey
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	if cfg.Enable {
		h.mgr.Start(cfg.ID)
	}
	logger.WriteLog("info", "wireguard", fmt.Sprintf("创建WireGuard接口 [%d] %s", cfg.ID, cfg.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *WireguardHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WireguardConfig
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
	logger.WriteLog("info", "wireguard", fmt.Sprintf("更新WireGuard接口 [%d] %s", id, req.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WireguardHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	// 删除关联的对等节点
	h.db.Where("wireguard_id = ?", id).Delete(&model.WireguardPeer{})
	h.db.Delete(&model.WireguardConfig{}, id)
	logger.WriteLog("info", "wireguard", fmt.Sprintf("删除WireGuard接口 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WireguardHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.WireguardConfig{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "wireguard", fmt.Sprintf("启动WireGuard接口 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *WireguardHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.WireguardConfig{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "wireguard", fmt.Sprintf("停止WireGuard接口 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *WireguardHandler) GetStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	status := h.mgr.GetStatus(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"status": status}})
}

// GenerateKeyPair 生成密钥对
func (h *WireguardHandler) GenerateKeyPair(c *gin.Context) {
	privKey, pubKey, err := wireguard.GenerateKeyPair()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成密钥对失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"private_key": privKey, "public_key": pubKey}})
}

// ===== WireGuard 对等节点管理 =====

func (h *WireguardHandler) ListPeers(c *gin.Context) {
	wgID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var peers []model.WireguardPeer
	h.db.Where("wireguard_id = ?", wgID).Order("id desc").Find(&peers)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": peers})
}

func (h *WireguardHandler) CreatePeer(c *gin.Context) {
	wgID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var peer model.WireguardPeer
	if err := c.ShouldBindJSON(&peer); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	peer.WireguardID = uint(wgID)
	h.db.Create(&peer)
	// 如果接口正在运行，重新加载配置
	h.mgr.ReloadConfig(uint(wgID))
	logger.WriteLog("info", "wireguard", fmt.Sprintf("创建WireGuard对等节点 [%d] %s", peer.ID, peer.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": peer, "message": "创建成功"})
}

func (h *WireguardHandler) UpdatePeer(c *gin.Context) {
	wgID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	peerID, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	var req model.WireguardPeer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(peerID)
	req.WireguardID = uint(wgID)
	h.db.Save(&req)
	h.mgr.ReloadConfig(uint(wgID))
	logger.WriteLog("info", "wireguard", fmt.Sprintf("更新WireGuard对等节点 [%d] %s", peerID, req.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WireguardHandler) DeletePeer(c *gin.Context) {
	wgID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	peerID, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	h.db.Delete(&model.WireguardPeer{}, peerID)
	h.mgr.ReloadConfig(uint(wgID))
	logger.WriteLog("info", "wireguard", fmt.Sprintf("删除WireGuard对等节点 [%d]", peerID))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// buildPeerClientConfig 为指定 Peer 生成客户端 .conf 配置文件内容
// 客户端配置：[Interface] 使用 Peer 自身的 IP（从 AllowedIPs 取第一个 /32 地址），
// [Peer] 指向服务端（本 WireGuard 接口）的公钥和监听地址。
func buildPeerClientConfig(wg *model.WireguardConfig, peer *model.WireguardPeer) string {
	var sb strings.Builder

	// [Interface] 段 —— 客户端自身配置
	sb.WriteString("[Interface]\n")
	// 客户端需要自己的私钥，此处留空提示用户填写
	sb.WriteString("# PrivateKey = <请填写客户端私钥>\n")
	// 从 AllowedIPs 中取第一个地址作为客户端 IP 提示
	if peer.AllowedIPs != "" {
		firstIP := strings.Split(peer.AllowedIPs, ",")[0]
		firstIP = strings.TrimSpace(firstIP)
		// 将 /32 或 /128 改为 /24 等子网，或直接使用
		sb.WriteString(fmt.Sprintf("Address = %s\n", firstIP))
	}
	if wg.DNS != "" {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", wg.DNS))
	}
	if wg.MTU > 0 {
		sb.WriteString(fmt.Sprintf("MTU = %d\n", wg.MTU))
	}

	// [Peer] 段 —— 服务端信息
	sb.WriteString("\n[Peer]\n")
	if wg.PublicKey != "" {
		sb.WriteString(fmt.Sprintf("PublicKey = %s\n", wg.PublicKey))
	}
	if peer.PresharedKey != "" {
		sb.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
	}
	if peer.Endpoint != "" {
		sb.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
	}
	// AllowedIPs 客户端侧通常为 0.0.0.0/0（全流量）或具体子网
	if peer.AllowedIPs != "" {
		sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIPs))
	}
	if peer.PersistentKeepalive > 0 {
		sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
	}

	return sb.String()
}

// GetPeerConfig 下载指定 Peer 的客户端隧道配置文件
// GET /wireguard/:id/peers/:pid/config
func (h *WireguardHandler) GetPeerConfig(c *gin.Context) {
	wgID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	peerID, _ := strconv.ParseUint(c.Param("pid"), 10, 64)

	var wg model.WireguardConfig
	if err := h.db.First(&wg, wgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "WireGuard 接口不存在"})
		return
	}

	var peer model.WireguardPeer
	if err := h.db.Where("id = ? AND wireguard_id = ?", peerID, wgID).First(&peer).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "对等节点不存在"})
		return
	}

	content := buildPeerClientConfig(&wg, &peer)

	// 文件名：使用节点名称，去掉非法字符
	filename := peer.Name
	if filename == "" {
		filename = fmt.Sprintf("peer-%d", peerID)
	}
	// 简单清理文件名
	filename = strings.ReplaceAll(filename, " ", "_")
	filename = strings.ReplaceAll(filename, "/", "-")
	filename += ".conf"

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, content)
}

// GetPeerQRCode 返回指定 Peer 客户端配置的二维码 PNG 图片
// GET /wireguard/:id/peers/:pid/qrcode
func (h *WireguardHandler) GetPeerQRCode(c *gin.Context) {
	wgID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	peerID, _ := strconv.ParseUint(c.Param("pid"), 10, 64)

	var wg model.WireguardConfig
	if err := h.db.First(&wg, wgID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "WireGuard 接口不存在"})
		return
	}

	var peer model.WireguardPeer
	if err := h.db.Where("id = ? AND wireguard_id = ?", peerID, wgID).First(&peer).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "对等节点不存在"})
		return
	}

	content := buildPeerClientConfig(&wg, &peer)

	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成二维码失败: " + err.Error()})
		return
	}

	c.Header("Content-Type", "image/png")
	c.Data(http.StatusOK, "image/png", png)
}
