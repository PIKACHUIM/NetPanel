package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	ipLimiter       *ratelimit.Limiter
	redirectLimiter *ratelimit.Limiter
}

func NewAuthHandler(db *gorm.DB, log *logrus.Logger) *AuthHandler {
	return &AuthHandler{
		db:              db,
		log:             log,
		loginLimiter:    ratelimit.New(5, time.Minute, 15*time.Minute),
		ipLimiter:       ratelimit.New(20, 5*time.Minute, 15*time.Minute),
		redirectLimiter: ratelimit.New(30, time.Minute, 5*time.Minute),
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login 登录
// 支持多用户登录：优先从 User 表验证，兼容旧版 SystemConfig 明文密码
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 登录失败限流：两层维度
	//  1) 「IP+用户名」——防止针对单一账号的暴力破解
	//  2) 「纯 IP」——防止轮换用户名绕过（每个新用户名都会获得全新配额）
	// 注意 ClientIP 的可靠性取决于中间件配置的可信代理白名单（默认不信任任何代理）。
	clientIP := c.ClientIP()
	limiterKey := clientIP + "|" + req.Username
	if !h.loginLimiter.Allowed(limiterKey) || !h.ipLimiter.Allowed(clientIP) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "失败次数过多，请稍后再试"})
		return
	}

	// 用户身份信息：随 JWT 下发，作为后续权限判定的依据
	var (
		userID       uint
		isAdmin      bool
		tokenVersion int
		roles        string
	)

	// 优先从 User 表查找用户
	var user model.User
	if err := h.db.Where("username = ?", req.Username).First(&user).Error; err == nil {
		// 用户存在：验证密码和状态
		if !user.Enable {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "账号已被禁用"})
			return
		}
		if !utils.CheckPassword(req.Password, user.Password) {
			h.loginLimiter.RecordFailure(limiterKey)
			h.ipLimiter.RecordFailure(clientIP)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户名或密码错误"})
			return
		}
		userID = user.ID
		isAdmin = user.IsAdminLegacy()
		tokenVersion = user.TokenVersion
		roles = user.Roles
	} else {
		// User 表中不存在，兼容旧版：仅允许 admin 用户通过 SystemConfig 验证
		if req.Username != "admin" {
			h.loginLimiter.RecordFailure(limiterKey)
			h.ipLimiter.RecordFailure(clientIP)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户名或密码错误"})
			return
		}
		var cfg model.SystemConfig
		if err := h.db.Where("key = ?", "admin_password").First(&cfg).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "系统错误"})
			return
		}
		if !utils.CheckPassword(req.Password, cfg.Value) {
			h.loginLimiter.RecordFailure(limiterKey)
			h.ipLimiter.RecordFailure(clientIP)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户名或密码错误"})
			return
		}
		// 旧版引导账号视为管理员（此路径无对应 User 记录，userID 保持 0）
		isAdmin = true
		roles = model.RoleAdmin
	}

	token, err := middleware.GenerateTokenWithRoles(req.Username, userID, isAdmin, roles, tokenVersion)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成 Token 失败"})
		return
	}

	// 登录成功，清零该维度的失败计数（IP 维度保留，避免用正确账号冲抵失败计数）
	h.loginLimiter.Reset(limiterKey)

	// 设置平台访问控制 Cookie
	middleware.SetSessionCookie(c, req.Username, tokenVersion)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "登录成功",
		"data": gin.H{
			"token":    token,
			"username": req.Username,
			"is_admin": isAdmin,
			"roles":    roles,
		},
	})
}

// ValidateRedirect 只允许回跳到本机路由或显式配置为页面登录的 Caddy 站点。
func (h *AuthHandler) ValidateRedirect(c *gin.Context) {
	if !h.redirectLimiter.Allowed(c.ClientIP()) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "请求过于频繁，请稍后再试"})
		return
	}
	_, valid := validateLoginRedirect(h.db, c.Query("url"))
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"valid": valid}})
}

// validateLoginRedirect 返回安全回跳目标。绝对 URL 只允许配置了 page_login 的启用站点。
func validateLoginRedirect(db *gorm.DB, raw string) (string, bool) {
	if len(raw) == 0 || len(raw) > 2048 {
		return "", false
	}
	if path := safeOAuthRedirect(raw); path != "" {
		return path, true
	}
	if strings.Contains(raw, `\`) {
		return "", false
	}
	target, err := url.Parse(raw)
	if err != nil || !target.IsAbs() || target.Opaque != "" || target.User != nil || target.Fragment != "" {
		return "", false
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return "", false
	}
	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	if host == "" || strings.Contains(host, "%") || net.ParseIP(host) != nil || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", false
	}
	port := 80
	if target.Scheme == "https" {
		port = 443
	}
	if explicitPort := target.Port(); explicitPort != "" {
		port, err = strconv.Atoi(explicitPort)
		if err != nil || port < 1 || port > 65535 {
			return "", false
		}
	}

	var sites []model.CaddySite
	if err := db.Where("enable = ?", true).Find(&sites).Error; err != nil || len(sites) == 0 {
		return "", false
	}
	var rules []model.AccessRule
	if err := db.Where("enable = ? AND auth_mode IN ?", true, []string{"page_login", "oidc"}).Find(&rules).Error; err != nil || len(rules) == 0 {
		return "", false
	}
	for _, site := range sites {
		if site.Port != port || !matchesCaddyHost(site.Domain, host) {
			continue
		}
		for _, rule := range rules {
			if rule.AuthMode == "oidc" && rule.OidcProviderID != 0 {
				var provider model.OAuthProviderConfig
				if err := db.Where("id = ? AND enable = ?", rule.OidcProviderID, true).First(&provider).Error; err != nil {
					continue
				}
			}
			var siteIDs []uint
			if json.Unmarshal([]byte(rule.BindSiteIDs), &siteIDs) != nil {
				continue
			}
			for _, siteID := range siteIDs {
				if siteID == site.ID {
					return target.String(), true
				}
			}
		}
	}
	return "", false
}

func matchesCaddyHost(configured, target string) bool {
	configured = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(configured), "."))
	if configured == target {
		return true
	}
	if strings.HasPrefix(configured, "*.") {
		suffix := configured[1:]
		prefix := strings.TrimSuffix(target, suffix)
		return strings.HasSuffix(target, suffix) && prefix != "" && !strings.Contains(prefix, ".")
	}
	return false
}

// Logout 登出并吊销 JWT，同时清除 Caddy page_login 使用的平台会话 Cookie。
func (h *AuthHandler) Logout(c *gin.Context) {
	if userID, ok := middleware.CurrentUserID(c); ok {
		if err := h.db.Model(&model.User{}).Where("id = ?", userID).
			UpdateColumn("token_version", gorm.Expr("token_version + 1")).Error; err != nil {
			middleware.ClearSessionCookie(c)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "撤销登录令牌失败"})
			return
		}
	}
	middleware.ClearSessionCookie(c)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已登出"})
}
