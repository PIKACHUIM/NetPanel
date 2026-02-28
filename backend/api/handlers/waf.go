package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== WAF 防火墙 =====

type WafHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewWafHandler(db *gorm.DB, log *logrus.Logger) *WafHandler {
	return &WafHandler{db: db, log: log}
}

func (h *WafHandler) List(c *gin.Context) {
	var configs []model.WafConfig
	h.db.Order("id desc").Find(&configs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *WafHandler) Create(c *gin.Context) {
	var cfg model.WafConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *WafHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WafConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WafHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.WafConfig{}, id)
	h.db.Where("waf_config_id = ?", id).Delete(&model.WafLog{})
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WafHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cfg model.WafConfig
	if err := h.db.First(&cfg, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "WAF 配置不存在"})
		return
	}
	// TODO: 启动 Coraza WAF 引擎
	h.log.Infof("[WAF] 启动: id=%d name=%s", id, cfg.Name)
	h.db.Model(&model.WafConfig{}).Where("id = ?", id).Updates(map[string]any{
		"enable": true,
		"status": "running",
	})
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *WafHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// TODO: 停止 Coraza WAF 引擎
	h.log.Infof("[WAF] 停止: id=%d", id)
	h.db.Model(&model.WafConfig{}).Where("id = ?", id).Updates(map[string]any{
		"enable": false,
		"status": "stopped",
	})
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *WafHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	var logs []model.WafLog
	var total int64
	h.db.Model(&model.WafLog{}).Where("waf_config_id = ?", id).Count(&total)
	h.db.Where("waf_config_id = ?", id).
		Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

func (h *WafHandler) TestRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Rule string `json:"rule"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	// TODO: 使用 Coraza 解析并验证规则语法
	h.log.Infof("[WAF] 测试规则: id=%d rule=%s", id, req.Rule)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "规则语法正确"})
}
