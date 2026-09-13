package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== EasyTier 客户端 =====

type EasytierHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *easytier.Manager
}

func NewEasytierHandler(db *gorm.DB, log *logrus.Logger, mgr *easytier.Manager) *EasytierHandler {
	return &EasytierHandler{db: db, log: log, mgr: mgr}
}

func (h *EasytierHandler) List(c *gin.Context) {
	var clients []model.EasytierClient
	h.db.Order("id desc").Find(&clients)
	for i := range clients {
		clients[i].Status = h.mgr.GetClientStatus(clients[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": clients})
}

func (h *EasytierHandler) Create(c *gin.Context) {
	var client model.EasytierClient
	if err := c.ShouldBindJSON(&client); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	client.Status = "stopped"
	h.db.Create(&client)
	if client.Enable {
		h.mgr.StartClient(client.ID)
	}
	logger.WriteLog("info", "easytier", fmt.Sprintf("创建EasyTier客户端 [%d] %s", client.ID, client.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": client, "message": "创建成功"})
}

func (h *EasytierHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.EasytierClient
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopClient(uint(id))
	req.ID = uint(id)
	var existing model.EasytierClient
	if err := h.db.First(&existing, id).Error; err == nil {
		model.PreserveSecrets(&req, &existing)
	}
	h.db.Save(&req)
	if req.Enable {
		h.mgr.StartClient(uint(id))
	}
	logger.WriteLog("info", "easytier", fmt.Sprintf("更新EasyTier客户端 [%d] %s", id, req.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *EasytierHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Delete(&model.EasytierClient{}, id)
	logger.WriteLog("info", "easytier", fmt.Sprintf("删除EasyTier客户端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *EasytierHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartClient(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.EasytierClient{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "easytier", fmt.Sprintf("启动EasyTier客户端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *EasytierHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Model(&model.EasytierClient{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "easytier", fmt.Sprintf("停止EasyTier客户端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *EasytierHandler) GetStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	status := h.mgr.GetClientStatus(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"status": status}})
}

func (h *EasytierHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logs := h.mgr.GetClientLogs(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": logs})
}

func (h *EasytierHandler) GetPeers(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	info, err := h.mgr.GetClientPeers(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": info})
}

// ===== EasyTier 服务端 =====

type EasytierServerHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *easytier.Manager
}

func NewEasytierServerHandler(db *gorm.DB, log *logrus.Logger, mgr *easytier.Manager) *EasytierServerHandler {
	return &EasytierServerHandler{db: db, log: log, mgr: mgr}
}

func (h *EasytierServerHandler) List(c *gin.Context) {
	var servers []model.EasytierServer
	h.db.Order("id desc").Find(&servers)
	for i := range servers {
		servers[i].Status = h.mgr.GetServerStatus(servers[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": servers})
}

func (h *EasytierServerHandler) Create(c *gin.Context) {
	var server model.EasytierServer
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	server.Status = "stopped"
	h.db.Create(&server)
	if server.Enable {
		h.mgr.StartServer(server.ID)
	}
	logger.WriteLog("info", "easytier", fmt.Sprintf("创建EasyTier服务端 [%d] %s", server.ID, server.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": server, "message": "创建成功"})
}

func (h *EasytierServerHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.EasytierServer
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopServer(uint(id))
	req.ID = uint(id)
	var existing model.EasytierServer
	if err := h.db.First(&existing, id).Error; err == nil {
		model.PreserveSecrets(&req, &existing)
	}
	h.db.Save(&req)
	if req.Enable {
		h.mgr.StartServer(uint(id))
	}
	logger.WriteLog("info", "easytier", fmt.Sprintf("更新EasyTier服务端 [%d] %s", id, req.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *EasytierServerHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Delete(&model.EasytierServer{}, id)
	logger.WriteLog("info", "easytier", fmt.Sprintf("删除EasyTier服务端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *EasytierServerHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartServer(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.EasytierServer{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "easytier", fmt.Sprintf("启动EasyTier服务端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *EasytierServerHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Model(&model.EasytierServer{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "easytier", fmt.Sprintf("停止EasyTier服务端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *EasytierServerHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logs := h.mgr.GetServerLogs(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": logs})
}

func (h *EasytierServerHandler) GetPeers(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	info, err := h.mgr.GetServerPeers(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": info})
}
