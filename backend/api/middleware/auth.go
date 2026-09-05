package middleware

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/netpanel/netpanel/pkg/secret"
)

// Claims JWT 声明。
// IsAdmin 随令牌下发，供 AdminOnly 中间件做权限判定；
// UserID 作为稳定标识（用户名可被修改，不能作为身份依据）；
// Roles 为逗号分隔的角色列表（如 "admin,editor"）。
type Claims struct {
	Username string `json:"username"`
	UserID   uint   `json:"user_id"`
	IsAdmin  bool   `json:"is_admin"`
	Roles    string `json:"roles"`
	jwt.RegisteredClaims
}

// GenerateToken 生成 JWT token
func GenerateToken(username string, userID uint, isAdmin bool) (string, error) {
	return generateTokenInternal(username, userID, isAdmin, "")
}

// GenerateTokenWithRoles 生成 JWT token，并将角色列表写入 claims。
func GenerateTokenWithRoles(username string, userID uint, isAdmin bool, roles string) (string, error) {
	return generateTokenInternal(username, userID, isAdmin, roles)
}

func generateTokenInternal(username string, userID uint, isAdmin bool, roles string) (string, error) {
	claims := Claims{
		Username: username,
		UserID:   userID,
		IsAdmin:  isAdmin,
		Roles:    roles,
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

// JWTAuth JWT 认证中间件
func JWTAuth() gin.HandlerFunc {
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

		c.Set("username", claims.Username)
		c.Set("user_id", claims.UserID)
		c.Set("is_admin", claims.IsAdmin)
		c.Set("roles", claims.Roles)
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

// RoleRequired 角色权限中间件。必须置于 JWTAuth 之后。
// requiredRoles 至少包含一个候选角色，命中任意一个即放行；为空时退化为 AdminOnly。
func RoleRequired(requiredRoles ...string) gin.HandlerFunc {
	if len(requiredRoles) == 0 {
		return AdminOnly()
	}
	return func(c *gin.Context) {
		roles, exists := c.Get("roles")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未授权，请先登录"})
			c.Abort()
			return
		}
		userRoles, ok := roles.(string)
		if !ok || userRoles == "" {
			c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "没有访问权限"})
			c.Abort()
			return
		}
		userRoleSet := make(map[string]struct{}, strings.Count(userRoles, ",")+1)
		for _, r := range strings.Split(userRoles, ",") {
			userRoleSet[strings.TrimSpace(r)] = struct{}{}
		}
		for _, req := range requiredRoles {
			if _, granted := userRoleSet[req]; granted {
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "权限不足"})
		c.Abort()
	}
}

// MustCurrentUser 提取当前请求用户 ID；不存在时中止响应。
func MustCurrentUser(c *gin.Context) uint {
	id, ok := CurrentUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
		c.Abort()
	}
	return id
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

// CurrentRoles 从上下文取当前用户角色字符串（逗号分隔）。
func CurrentRoles(c *gin.Context) string {
	v, exists := c.Get("roles")
	if !exists {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
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
