package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
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
		if s := h.mgr.GetStunStatus(rules[i].ID); s != "" {
			rules[i].StunStatus = s
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
	logger.WriteLog("info", "stun", fmt.Sprintf("创建STUN穿透规则 [%d]", rule.ID))
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
	logger.WriteLog("info", "stun", fmt.Sprintf("修改STUN穿透规则 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *StunHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.StunRule{}, id)
	logger.WriteLog("info", "stun", fmt.Sprintf("删除STUN穿透规则 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *StunHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.StunRule{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "stun", fmt.Sprintf("启动STUN穿透规则 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *StunHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.StunRule{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "stun", fmt.Sprintf("停止STUN穿透规则 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// DetectNAT 按需探测 NAT 类型（不依赖已配置的 STUN 规则），返回结果并附打洞可行性提示
func (h *StunHandler) DetectNAT(c *gin.Context) {
	server := c.Query("server")
	info, err := h.mgr.CheckNow(server)
	if info == nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"ip":        info.IP,
		"port":      info.Port,
		"nat_type":  string(info.NATType),
		"p2p_score": stun.P2PScore(info.NATType),
		"error":     errMessage(err),
	}})
}

// errMessage 探测过程中的非致命错误说明（如 UDP 被封锁）
func errMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
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
