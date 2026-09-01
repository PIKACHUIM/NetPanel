package handlers

import (
	"fmt"
	"net/http"
	"sync"

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
	mu  sync.Mutex // 串行化 setup，防止并发请求同时通过初始化检查
}

func NewInitHandler(db *gorm.DB, log *logrus.Logger) *InitHandler {
	return &InitHandler{db: db, log: log}
}

// isInitialized 判断系统是否已初始化（存在用户或旧版 admin_password 配置即视为已初始化）。
//
// 安全说明：数据库查询失败时必须保守返回 true。若返回 false，未认证的调用方
// 即可通过 /init/setup 创建管理员并覆盖 admin_password，形成认证绕过。
func (h *InitHandler) isInitialized() bool {
	var userCount int64
	if err := h.db.Model(&model.User{}).Count(&userCount).Error; err != nil {
		h.log.Warnf("[初始化] 查询用户数失败，保守视为已初始化: %v", err)
		return true
	}
	if userCount > 0 {
		return true
	}
	var cfgCount int64
	if err := h.db.Model(&model.SystemConfig{}).Where("key = ?", "admin_password").Count(&cfgCount).Error; err != nil {
		h.log.Warnf("[初始化] 查询系统配置失败，保守视为已初始化: %v", err)
		return true
	}
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

	// 互斥锁：串行化"检查已初始化 -> 创建管理员"（单实例内的快速失败路径）
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.isInitialized() {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "系统已初始化，无需重复设置"})
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

	// 事务内重新校验并写入：进程级互斥锁在多实例共用同一数据库时不成立，
	// 必须由数据库保证"仅首次初始化可成功"这一不变量。
	var alreadyInitialized bool
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		var userCount int64
		if err := tx.Model(&model.User{}).Count(&userCount).Error; err != nil {
			return err
		}
		if userCount > 0 {
			alreadyInitialized = true
			return fmt.Errorf("系统已初始化")
		}
		if err := tx.Create(&user).Error; err != nil {
			return err
		}
		// 同步旧版 admin_password 配置，兼容历史登录逻辑
		var cfg model.SystemConfig
		if err := tx.Where("key = ?", "admin_password").First(&cfg).Error; err == nil {
			return tx.Model(&model.SystemConfig{}).Where("key = ?", "admin_password").Update("value", hashed).Error
		}
		return tx.Create(&model.SystemConfig{Key: "admin_password", Value: hashed}).Error
	}); err != nil {
		if alreadyInitialized {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "系统已初始化，无需重复设置"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}

	logger.WriteLog("info", "system", fmt.Sprintf("系统初始化：创建管理员 %s", req.Username))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "初始化完成", "data": gin.H{"username": req.Username}})
}
