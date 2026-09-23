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
	Username  string `json:"u"`
	ExpiresAt int64  `json:"e"`
}

// Issue 生成签名后的 Cookie 值。
func Issue(username string) string {
	payload, _ := json.Marshal(Data{
		Username:  username,
		ExpiresAt: time.Now().Add(MaxAge * time.Second).Unix(),
	})
	return hex.EncodeToString(payload) + "." + sign(payload)
}

// Validate 校验 Cookie 值的签名与有效期，返回用户名。
func Validate(cookie string) (string, bool) {
	payloadHex, signature, ok := strings.Cut(cookie, ".")
	if !ok {
		return "", false
	}

	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return "", false
	}

	// 使用 hmac.Equal 做定长比较，避免通过响应时间侧信道逐字节推断签名
	if !hmac.Equal([]byte(signature), []byte(sign(payload))) {
		return "", false
	}

	var data Data
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", false
	}
	if time.Now().Unix() > data.ExpiresAt {
		return "", false
	}
	return data.Username, true
}

// sign 使用 session 专用派生子密钥做 HMAC-SHA256 签名。
func sign(payload []byte) string {
	mac := hmac.New(sha256.New, secret.SessionKey())
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
