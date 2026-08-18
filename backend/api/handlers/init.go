package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/pkg/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// InitHandler 首次初始化处理器
type InitHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewInitHandler(db *gorm.DB, log *logrus.Logger) *InitHandler {
	return &InitHandler{db: db, log: log}
}

// isInitialized 判断系统是否已初始化（存在用户或旧版 admin_password 配置即视为已初始化）
func (h *InitHandler) isInitialized() bool {
	var userCount int64
	h.db.Model(&model.User{}).Count(&userCount)
	if userCount > 0 {
		return true
	}
	var cfgCount int64
	h.db.Model(&model.SystemConfig{}).Where("key = ?", "admin_password").Count(&cfgCount)
	return cfgCount > 0
}

// Status 获取初始化状态
// GET /api/v1/init/status
func (h *InitHandler) Status(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"initialized": h.isInitialized()}})
}

// Setup 首次初始化：创建首个管理员账号
// POST /api/v1/init/setup  body: {username, password}
func (h *InitHandler) Setup(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=2,max=50"`
		Password string `json:"password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if h.isInitialized() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "系统已初始化，无需重复设置"})
		return
	}

	// 用户名唯一性
	var count int64
	h.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "用户名已存在"})
		return
	}

	hashed, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "密码加密失败"})
		return
	}

	user := model.User{
		Username: req.Username,
		Password: hashed,
		Enable:   true,
		IsAdmin:  true,
		Remark:   "系统初始化创建",
	}
	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}

	// 同步旧版 admin_password 配置，兼容历史登录逻辑
	var cfg model.SystemConfig
	if err := h.db.Where("key = ?", "admin_password").First(&cfg).Error; err == nil {
		h.db.Model(&model.SystemConfig{}).Where("key = ?", "admin_password").Update("value", hashed)
	} else {
		h.db.Create(&model.SystemConfig{Key: "admin_password", Value: hashed})
	}

	logger.WriteLog("info", "system", fmt.Sprintf("系统初始化：创建管理员 %s", req.Username))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "初始化完成", "data": gin.H{"username": req.Username}})
}
