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
	// 登录失败限流：按「IP+用户名」维度，防止针对单一账号的在线暴力破解
	loginLimiter *ratelimit.Limiter
	// 纯 IP 维度限流：即使攻击者轮换用户名（或伪造 X-Forwarded-For 未被信任时
	// 也仅剩真实 IP），也会在总失败次数上被拦截，避免「换用户名即重置配额」。
	ipLimiter *ratelimit.Limiter
	// loginBackoff 对"IP+用户名"组合做连续失败指数退避，防止在线爆破。
	loginBackoff *ratelimit.Backoff
}

func NewAuthHandler(db *gorm.DB, log *logrus.Logger) *AuthHandler {
	return &AuthHandler{
		db:           db,
		log:          log,
		loginLimiter: ratelimit.New(5, time.Minute, 15*time.Minute),
		ipLimiter:    ratelimit.New(20, 5*time.Minute, 15*time.Minute),
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
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 登录失败限流：三层防护
	//  1) 「IP+用户名」——防止针对单一账号的暴力破解
	//  2) 「纯 IP」——防止轮换用户名绕过（每个新用户名都会获得全新配额）
	//  3) 指数退避——连续失败后逐步拉长锁定时长
	// 注意 ClientIP 的可靠性取决于中间件配置的可信代理白名单（默认不信任任何代理）。
	clientIP := c.ClientIP()
	limiterKey := clientIP + "|" + req.Username
	if !h.loginLimiter.Allowed(limiterKey) || !h.ipLimiter.Allowed(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "失败次数过多，请稍后再试"})
		return
	}
	if wait, blocked := h.loginBackoff.Blocked(limiterKey); blocked {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":    429,
			"message": fmt.Sprintf("失败次数过多，请 %d 秒后再试", int(wait.Seconds())+1),
		})
		return
	}

	// 用户身份信息：随 JWT 下发，作为后续权限判定的依据
	var (
		userID       uint
		isAdmin      bool
		tokenVersion int
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
		tokenVersion = user.TokenVersion
	} else {
		// 用户不存在或密码错误：统一报错文案，不区分两种情况（防用户名枚举）
		h.loginLimiter.RecordFailure(limiterKey)
		h.ipLimiter.RecordFailure(clientIP)
		h.loginBackoff.RecordFailure(limiterKey)
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户名或密码错误"})
		return
	}

	// 旧版"admin 经 SystemConfig.admin_password 登录"的兼容路径已移除：
	// 该路径与 PUT /system/config 组合曾被用于非管理员提权，历史数据
	// 已由 db 层的一次性迁移（migrateLegacyAdminPassword）转为 User 记录。

	h.loginLimiter.Reset(limiterKey)
	h.loginBackoff.Reset(limiterKey)

	token, err := middleware.GenerateToken(req.Username, userID, isAdmin, tokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成 Token 失败"})
		return
	}

	// 登录成功，清零该维度的失败计数（IP 维度保留，避免用正确账号冲抵失败计数）
	h.loginLimiter.Reset(limiterKey)

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
//
// 除返回成功外，必须同时清除平台会话 Cookie：该 Cookie 是 Caddy page_login
// 站点与面板自身的访问凭据，此前不清除导致登出后 24 小时内仍然有效。
func (h *AuthHandler) Logout(c *gin.Context) {
	middleware.ClearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已登出"})
}
