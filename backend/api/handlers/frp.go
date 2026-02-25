package handlers

import (
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== FRP 客户端 =====

type FrpcHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *frp.Manager
}

func NewFrpcHandler(db *gorm.DB, log *logrus.Logger, mgr *frp.Manager) *FrpcHandler {
	return &FrpcHandler{db: db, log: log, mgr: mgr}
}

func (h *FrpcHandler) List(c *gin.Context) {
	var configs []model.FrpcConfig
	h.db.Preload("Proxies").Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetClientStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *FrpcHandler) Create(c *gin.Context) {
	var cfg model.FrpcConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	if cfg.Enable {
		h.mgr.StartClient(cfg.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *FrpcHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.FrpcConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopClient(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	if req.Enable {
		h.mgr.StartClient(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *FrpcHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Where("frpc_id = ?", id).Delete(&model.FrpcProxy{})
	h.db.Delete(&model.FrpcConfig{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *FrpcHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartClient(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *FrpcHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// Restart 重启 FRP 客户端
func (h *FrpcHandler) Restart(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.RestartClient(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已重启"})
}

func (h *FrpcHandler) ListProxies(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var proxies []model.FrpcProxy
	h.db.Where("frpc_id = ?", id).Find(&proxies)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": proxies})
}

func (h *FrpcHandler) CreateProxy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var proxy model.FrpcProxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	proxy.FrpcID = uint(id)
	h.db.Create(&proxy)
	// 重启客户端以应用新代理
	h.mgr.RestartClient(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": proxy, "message": "创建成功"})
}

func (h *FrpcHandler) UpdateProxy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	var proxy model.FrpcProxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	proxy.ID = uint(pid)
	proxy.FrpcID = uint(id)
	h.db.Save(&proxy)
	h.mgr.RestartClient(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": proxy, "message": "更新成功"})
}

func (h *FrpcHandler) DeleteProxy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	h.db.Delete(&model.FrpcProxy{}, pid)
	h.mgr.RestartClient(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// ===== FRP 服务端 =====

type FrpsHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *frp.Manager
}

func NewFrpsHandler(db *gorm.DB, log *logrus.Logger, mgr *frp.Manager) *FrpsHandler {
	return &FrpsHandler{db: db, log: log, mgr: mgr}
}

func (h *FrpsHandler) List(c *gin.Context) {
	var configs []model.FrpsConfig
	h.db.Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetServerStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *FrpsHandler) Create(c *gin.Context) {
	var cfg model.FrpsConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	if cfg.Enable {
		h.mgr.StartServer(cfg.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *FrpsHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.FrpsConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopServer(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	if req.Enable {
		h.mgr.StartServer(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *FrpsHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Delete(&model.FrpsConfig{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *FrpsHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartServer(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *FrpsHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// GetDashboardURL 返回 frps Dashboard 的访问地址
func (h *FrpsHandler) GetDashboardURL(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var cfg model.FrpsConfig
	if err := h.db.First(&cfg, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "配置不存在"})
		return
	}

	if cfg.DashboardPort == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "未配置 Dashboard 端口"})
		return
	}

	status := h.mgr.GetServerStatus(uint(id))
	if status != "running" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "FRP 服务端未运行"})
		return
	}

	addr := cfg.DashboardAddr
	if addr == "" || addr == "0.0.0.0" {
		// 监听在所有网卡时，使用请求来源的 Host（去掉端口部分）
		host := c.Request.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			addr = h
		} else {
			addr = host
		}
	}

	url := fmt.Sprintf("http://%s:%d", addr, cfg.DashboardPort)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"url": url}})
}
