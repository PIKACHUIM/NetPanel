package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/secret"
	"gorm.io/gorm"
)

// Claims JWT 声明。
// IsAdmin 随令牌下发，供 AdminOnly 中间件做权限判定；
// UserID 作为稳定标识（用户名可被修改，不能作为身份依据）；
// TokenVersion 用于吊销：与 User 表中的当前版本比对，不一致即失效。
type Claims struct {
	Username     string `json:"username"`
	UserID       uint   `json:"user_id"`
	IsAdmin      bool   `json:"is_admin"`
	TokenVersion int    `json:"token_version"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT token。
// tokenVersion 取自用户记录，改密/禁用/降权时递增以吊销旧令牌。
func GenerateToken(username string, userID uint, isAdmin bool, tokenVersion int) (string, error) {
	claims := Claims{
		Username:     username,
		UserID:       userID,
		IsAdmin:      isAdmin,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret.JWTKey())
}

// ParseToken 解析 JWT token
func ParseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// 显式校验签名算法，防止 alg 混淆攻击（如伪造 alg=none）
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secret.JWTKey(), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, jwt.ErrSignatureInvalid
}

// JWTAuth JWT 认证中间件。
//
// db 用于在每次请求时核对令牌版本与账号状态：仅校验签名无法让已签发的
// 令牌在改密/禁用/降权后立即失效（令牌最长 24 小时有效）。db 为 nil 时
// 跳过这一核对（保留给测试等场景）。
func JWTAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未授权，请先登录"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Token 格式错误"})
			c.Abort()
			return
		}

		claims, err := ParseToken(parts[1])
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "Token 无效或已过期"})
			c.Abort()
			return
		}

		// 有稳定用户 ID 时核对账号状态与令牌版本，实现即时吊销
		if db != nil && claims.UserID != 0 {
			var user model.User
			if err := db.Select("id", "enable", "is_admin", "token_version").
				First(&user, claims.UserID).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "账号不存在"})
				c.Abort()
				return
			}
			if !user.Enable {
				c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "账号已被禁用"})
				c.Abort()
				return
			}
			if user.TokenVersion != claims.TokenVersion {
				c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "登录已失效，请重新登录"})
				c.Abort()
				return
			}
			// 权限以数据库实时状态为准，防止降权后旧令牌仍带管理员标识
			claims.IsAdmin = user.IsAdmin
		}

		c.Set("username", claims.Username)
		c.Set("user_id", claims.UserID)
		c.Set("is_admin", claims.IsAdmin)
		c.Next()
	}
}

// AdminOnly 管理员权限中间件。必须置于 JWTAuth 之后。
//
// 说明：此前 /admin/* 各接口仅挂在 JWTAuth 上，且权限判断依赖
// `currentUsername != "admin"` 这类用户名字面量比较。由于用户名可被修改，
// 该判断不可靠，任意登录用户可通过创建/修改用户接口给自己授予管理员权限。
// 现统一改为基于令牌中的 IsAdmin 声明做判定。
func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		isAdmin, exists := c.Get("is_admin")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未授权，请先登录"})
			c.Abort()
			return
		}
		if admin, ok := isAdmin.(bool); !ok || !admin {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "需要管理员权限"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// CurrentUserID 从上下文取当前用户 ID（稳定标识，优于用户名）。
func CurrentUserID(c *gin.Context) (uint, bool) {
	v, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok && id != 0
}

// IsCurrentUserAdmin 从上下文取当前用户是否为管理员。
func IsCurrentUserAdmin(c *gin.Context) bool {
	v, exists := c.Get("is_admin")
	if !exists {
		return false
	}
	admin, ok := v.(bool)
	return ok && admin
}

// TrustedProxies 返回可信反向代理的地址列表，用于 gin SetTrustedProxies。
//
// 默认返回 nil（不信任任何代理），使 c.ClientIP() 直接取 RemoteAddr。
// gin 的默认行为是信任所有代理（0.0.0.0/0），
// 攻击者只需伪造 X-Forwarded-For 即可绕过登录失败限流与 IP 黑白名单，
// 因此除非显式配置 NETPANEL_TRUSTED_PROXIES，否则一律不信任。
func TrustedProxies() []string {
	raw := os.Getenv("NETPANEL_TRUSTED_PROXIES")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// allowedOrigins 允许的跨域来源，通过环境变量 NETPANEL_ALLOWED_ORIGINS 配置
// （逗号分隔）。未配置时不下发跨域头，即仅允许同源访问。
func allowedOrigins() []string {
	raw := os.Getenv("NETPANEL_ALLOWED_ORIGINS")
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// CORS 跨域中间件。
//
// 原实现固定返回 Access-Control-Allow-Origin: *，与携带认证信息的接口组合后
// 会放大 CSRF 与信息泄露风险。现改为按白名单回显 Origin，默认仅同源。
func CORS() gin.HandlerFunc {
	origins := allowedOrigins()
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			for _, allowed := range origins {
				if allowed == origin || allowed == "*" {
					c.Header("Access-Control-Allow-Origin", origin)
					c.Header("Access-Control-Allow-Credentials", "true")
					c.Header("Vary", "Origin")
					break
				}
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS,PATCH")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
