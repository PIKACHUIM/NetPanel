package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== NPS 服务端 =====

type NpsServerHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *nps.Manager
}

func NewNpsServerHandler(db *gorm.DB, log *logrus.Logger, mgr *nps.Manager) *NpsServerHandler {
	return &NpsServerHandler{db: db, log: log, mgr: mgr}
}

func (h *NpsServerHandler) List(c *gin.Context) {
	var configs []model.NpsServerConfig
	h.db.Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetServerStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *NpsServerHandler) Create(c *gin.Context) {
	var cfg model.NpsServerConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	if cfg.Enable {
		if err := h.mgr.StartServer(cfg.ID); err != nil {
			h.log.Warnf("[NPS服务端] 创建后启动失败: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *NpsServerHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.NpsServerConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopServer(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	if req.Enable {
		if err := h.mgr.StartServer(uint(id)); err != nil {
			h.log.Warnf("[NPS服务端] 更新后启动失败: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *NpsServerHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Delete(&model.NpsServerConfig{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *NpsServerHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartServer(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.NpsServerConfig{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *NpsServerHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Model(&model.NpsServerConfig{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// ===== NPS 客户端 =====

type NpsClientHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *nps.Manager
}

func NewNpsClientHandler(db *gorm.DB, log *logrus.Logger, mgr *nps.Manager) *NpsClientHandler {
	return &NpsClientHandler{db: db, log: log, mgr: mgr}
}

func (h *NpsClientHandler) List(c *gin.Context) {
	var configs []model.NpsClientConfig
	h.db.Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetClientStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *NpsClientHandler) Create(c *gin.Context) {
	var cfg model.NpsClientConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	if cfg.Enable {
		if err := h.mgr.StartClient(cfg.ID); err != nil {
			h.log.Warnf("[NPS客户端] 创建后启动失败: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *NpsClientHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.NpsClientConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopClient(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	if req.Enable {
		if err := h.mgr.StartClient(uint(id)); err != nil {
			h.log.Warnf("[NPS客户端] 更新后启动失败: %v", err)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *NpsClientHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Delete(&model.NpsClientConfig{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *NpsClientHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartClient(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.NpsClientConfig{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *NpsClientHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Model(&model.NpsClientConfig{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// ===== NPS 隧道 =====

func (h *NpsClientHandler) ListTunnels(c *gin.Context) {
	clientID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var tunnels []model.NpsTunnel
	h.db.Where("nps_client_id = ?", clientID).Order("id desc").Find(&tunnels)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tunnels})
}

func (h *NpsClientHandler) CreateTunnel(c *gin.Context) {
	clientID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var tunnel model.NpsTunnel
	if err := c.ShouldBindJSON(&tunnel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	tunnel.NpsClientID = uint(clientID)
	h.db.Create(&tunnel)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tunnel, "message": "创建成功"})
}

func (h *NpsClientHandler) UpdateTunnel(c *gin.Context) {
	tid, _ := strconv.ParseUint(c.Param("tid"), 10, 64)
	var req model.NpsTunnel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(tid)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *NpsClientHandler) DeleteTunnel(c *gin.Context) {
	tid, _ := strconv.ParseUint(c.Param("tid"), 10, 64)
	h.db.Delete(&model.NpsTunnel{}, tid)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}
