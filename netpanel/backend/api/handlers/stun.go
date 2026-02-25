package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/stun"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type StunHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *stun.Manager
}

func NewStunHandler(db *gorm.DB, log *logrus.Logger, mgr *stun.Manager) *StunHandler {
	return &StunHandler{db: db, log: log, mgr: mgr}
}

func (h *StunHandler) List(c *gin.Context) {
	var rules []model.StunRule
	h.db.Order("id desc").Find(&rules)
	for i := range rules {
		rules[i].Status = h.mgr.GetStatus(rules[i].ID)
		if info := h.mgr.GetCurrentInfo(rules[i].ID); info != nil {
			rules[i].CurrentIP = info.IP
			rules[i].CurrentPort = info.Port
			rules[i].NATType = string(info.NATType)
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rules})
}

func (h *StunHandler) Create(c *gin.Context) {
	var rule model.StunRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	rule.Status = "stopped"
	h.db.Create(&rule)
	if rule.Enable {
		h.mgr.Start(rule.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rule, "message": "创建成功"})
}

func (h *StunHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.StunRule
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
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *StunHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.StunRule{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *StunHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.StunRule{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *StunHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.StunRule{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *StunHandler) GetStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	status := h.mgr.GetStatus(uint(id))
	info := h.mgr.GetCurrentInfo(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"status": status,
		"info":   info,
	}})
}
