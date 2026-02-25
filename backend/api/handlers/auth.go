package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api/middleware"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewAuthHandler(db *gorm.DB, log *logrus.Logger) *AuthHandler {
	return &AuthHandler{db: db, log: log}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 登录
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 从数据库获取密码
	var cfg model.SystemConfig
	if err := h.db.Where("key = ?", "admin_password").First(&cfg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "系统错误"})
		return
	}

	if req.Username != "admin" || req.Password != cfg.Value {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户名或密码错误"})
		return
	}

	token, err := middleware.GenerateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成 Token 失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"token":    token,
			"username": req.Username,
		},
	})
}

// Logout 登出
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已登出"})
}
