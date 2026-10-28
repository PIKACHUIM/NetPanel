// Package session 提供平台会话 Cookie 的签发与校验。
//
// 该逻辑此前在 api/middleware/platform_auth.go 与 service/access/manager.go
// 中各实现一份，且 service/caddy 构造的 page_login 路由完全没有校验签名
// （仅用正则匹配 Cookie 是否存在），导致任意人设置 netpanel_session=x
// 即可绕过认证。现统一收敛到此包，供各处复用。
package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/netpanel/netpanel/pkg/secret"
)

// CookieName 平台会话 Cookie 名称。
const CookieName = "netpanel_session"

// MaxAge 会话有效期（秒）。
const MaxAge = 86400

// Data 会话载荷。
type Data struct {
	Username     string `json:"u"`
	ExpiresAt    int64  `json:"e"`
	TokenVersion int    `json:"v"`
}

// Issue 生成签名后的 Cookie 值。
func Issue(username string, tokenVersion int) string {
	payload, _ := json.Marshal(Data{
		Username:     username,
		ExpiresAt:    time.Now().Add(MaxAge * time.Second).Unix(),
		TokenVersion: tokenVersion,
	})
	return hex.EncodeToString(payload) + "." + sign(payload)
}

// Validate 校验 Cookie 值的签名与有效期，返回用户名。
func Validate(cookie string) (string, bool) {
	data, ok := ValidateData(cookie)
	return data.Username, ok
}

// ValidateData 校验 Cookie 签名、有效期与携带的会话版本。
func ValidateData(cookie string) (Data, bool) {
	payloadHex, signature, ok := strings.Cut(cookie, ".")
	if !ok {
		return Data{}, false
	}

	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return Data{}, false
	}

	// 使用 hmac.Equal 做定长比较，避免通过响应时间侧信道逐字节推断签名
	if !hmac.Equal([]byte(signature), []byte(sign(payload))) {
		return Data{}, false
	}

	var data Data
	if err := json.Unmarshal(payload, &data); err != nil {
		return Data{}, false
	}
	if data.Username == "" || data.ExpiresAt <= time.Now().Unix() || data.TokenVersion < 0 {
		return Data{}, false
	}
	return data, true
}

// sign 使用 session 专用派生子密钥做 HMAC-SHA256 签名。
func sign(payload []byte) string {
	mac := hmac.New(sha256.New, secret.SessionKey())
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
