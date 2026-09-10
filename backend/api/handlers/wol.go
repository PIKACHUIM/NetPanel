package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== WOL =====

type WolHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewWolHandler(db *gorm.DB, log *logrus.Logger) *WolHandler {
	return &WolHandler{db: db, log: log}
}

func (h *WolHandler) List(c *gin.Context) {
	var devices []model.WolDevice
	h.db.Order("id desc").Find(&devices)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": devices})
}

func (h *WolHandler) Create(c *gin.Context) {
	var device model.WolDevice
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&device)
	logger.WriteLog("info", "wol", fmt.Sprintf("创建WOL设备 [%d] %s", device.ID, device.MACAddress))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": device, "message": "创建成功"})
}

func (h *WolHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WolDevice
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "wol", fmt.Sprintf("更新WOL设备 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WolHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.WolDevice{}, id)
	logger.WriteLog("info", "wol", fmt.Sprintf("删除WOL设备 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WolHandler) Wake(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var device model.WolDevice
	if err := h.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "设备不存在"})
		return
	}
	if err := sendWakePacket(device.MACAddress, device.BroadcastIP, device.NetInterface, device.Port); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "唤醒失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "wol", fmt.Sprintf("唤醒WOL设备 [%d] MAC=%s", id, device.MACAddress))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "唤醒包已发送"})
}
