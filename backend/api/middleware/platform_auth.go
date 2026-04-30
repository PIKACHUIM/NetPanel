package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// SetSessionCookie 设置平台访问控制 Cookie（登录成功后调用）
func SetSessionCookie(c *gin.Context, username string) {
	data := SessionData{
		Username:  username,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
	}
	payload, _ := json.Marshal(data)
	signature := signSession(string(payload))
	value := hex.EncodeToString(payload) + "." + signature
	c.SetCookie(sessionCookieName, value, sessionMaxAge, "/", "", false, true)
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

// signSession HMAC-SHA256 签名
func signSession(payload string) string {
	mac := hmac.New(sha256.New, []byte(jwtSecret)) // 复用 JWT 密钥
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
