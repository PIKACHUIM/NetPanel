package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/pkg/secret"
)

const (
	sessionCookieName = "netpanel_session"
	sessionMaxAge     = 86400 // 24 小时
)

// SessionData session 数据
type SessionData struct {
	Username  string `json:"u"`
	ExpiresAt int64  `json:"e"`
}

// SetSessionCookie 设置平台访问控制 Cookie（登录成功后调用）。
//
// Secure 属性按当前请求是否为 HTTPS 自动判定：HTTPS 下置为 true，
// 避免 Cookie 经明文信道传输；同时设置 SameSite=Lax 降低 CSRF 风险。
func SetSessionCookie(c *gin.Context, username string) {
	data := SessionData{
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	payload, _ := json.Marshal(data)
	signature := signSession(string(payload))
	value := hex.EncodeToString(payload) + "." + signature

	secure := c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https")
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, value, sessionMaxAge, "/", "", secure, true)
}

// ValidateSessionCookie 验证 session cookie，返回用户名
func ValidateSessionCookie(cookie string) (string, bool) {
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		return "", false
	}

	payloadHex := parts[0]
	signature := parts[1]

	// 验证签名
	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return "", false
	}

	expectedSig := signSession(string(payload))
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return "", false
	}

	// 解析数据
	var data SessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", false
	}

	// 检查过期
	if time.Now().Unix() > data.ExpiresAt {
		return "", false
	}

	return data.Username, true
}

// signSession HMAC-SHA256 签名。
// 使用独立派生的 session 子密钥，与 JWT 密钥分域，避免跨用途复用。
func signSession(payload string) string {
	mac := hmac.New(sha256.New, secret.SessionKey())
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
