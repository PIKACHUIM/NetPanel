package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/pkg/session"
)

// SetSessionCookie 设置平台访问控制 Cookie（登录成功后调用）。
//
// Secure 属性按当前请求是否为 HTTPS 自动判定：HTTPS 下置为 true，
// 避免 Cookie 经明文信道传输；同时设置 SameSite=Lax 降低 CSRF 风险。
func SetSessionCookie(c *gin.Context, username string) {
	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(session.CookieName, session.Issue(username), session.MaxAge, "/", "", secure, true)
}

// ClearSessionCookie 使会话 Cookie 立即失效（登出时调用）。
func ClearSessionCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(session.CookieName, "", -1, "/", "", false, true)
}

// ValidateSessionCookie 验证 session cookie，返回用户名。
func ValidateSessionCookie(cookie string) (string, bool) {
	return session.Validate(cookie)
}
