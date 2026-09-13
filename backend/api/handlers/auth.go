package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api/middleware"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/ratelimit"
	"github.com/netpanel/netpanel/pkg/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AuthHandler 认证处理器
type AuthHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	// loginLimiter 按 IP 限制登录请求频率；loginBackoff 对"IP+用户名"
	// 组合做连续失败指数退避，防止在线爆破。
	loginLimiter *ratelimit.Limiter
	loginBackoff *ratelimit.Backoff
}

func NewAuthHandler(db *gorm.DB, log *logrus.Logger) *AuthHandler {
	return &AuthHandler{
		db:           db,
		log:          log,
		loginLimiter: ratelimit.NewLimiter(time.Minute, 20),
		loginBackoff: ratelimit.NewBackoff(30*time.Second, 15*time.Minute, 5),
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 登录
// 仅支持 User 表多用户登录（bcrypt）；带 IP 频控与失败指数退避防爆破
func (h *AuthHandler) Login(c *gin.Context) {
	// 按 IP 限频
	ip := c.ClientIP()
	if !h.loginLimiter.Allow(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "登录请求过于频繁，请稍后再试"})
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 连续失败指数退避（IP+用户名 维度）
	backoffKey := ip + "|" + req.Username
	if wait, blocked := h.loginBackoff.Blocked(backoffKey); blocked {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": fmt.Sprintf("失败次数过多，请 %d 秒后再试", int(wait.Seconds())+1),
		})
		return
	}

	// 用户身份信息：随 JWT 下发，作为后续权限判定的依据
	var (
		userID  uint
		isAdmin bool
	)

	// 从 User 表验证
	var user model.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err == nil && utils.CheckPassword(req.Password, user.Password) {
		if !user.Enable {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "账号已被禁用"})
			return
		}
		userID = user.ID
		isAdmin = user.IsAdmin
	} else {
		// 用户不存在或密码错误：统一报错文案，不区分两种情况（防用户名枚举）
		h.loginBackoff.RecordFailure(backoffKey)
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户名或密码错误"})
		return
	}

	// 旧版"admin 经 SystemConfig.admin_password 登录"的兼容路径已移除：
	// 该路径与 PUT /system/config 组合曾被用于非管理员提权，历史数据
	// 已由 db 层的一次性迁移（migrateLegacyAdminPassword）转为 User 记录。

	h.loginBackoff.Reset(backoffKey)

	token, err := middleware.GenerateToken(req.Username, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成 Token 失败"})
		return
	}

	// 设置平台访问控制 Cookie
	middleware.SetSessionCookie(c, req.Username)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"token":    token,
			"username": req.Username,
			"is_admin": isAdmin,
		},
	})
}

// Logout 登出
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已登出"})
}
