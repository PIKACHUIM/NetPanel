package metrics

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// metricsToken 从环境变量 NETPANEL_METRICS_TOKEN 读取的采集令牌。
// 为空表示未启用鉴权（此时端点只应暴露在本机/内网可信环境）。
func metricsToken() string {
	return strings.TrimSpace(os.Getenv("NETPANEL_METRICS_TOKEN"))
}

// TokenAuth 返回 Bearer token 鉴权中间件。
// - 未配置 NETPANEL_METRICS_TOKEN：直接放行（由部署方通过监听地址/防火墙控制暴露面）
// - 已配置：校验 Authorization: Bearer <token>，失败返回 401
func TokenAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := metricsToken()
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(auth) <= len(prefix) || !strings.HasPrefix(auth, prefix) ||
			subtle.ConstantTimeCompare([]byte(auth[len(prefix):]), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HasToken 是否配置了采集令牌（供启动日志提示用）
func HasToken() bool {
	return metricsToken() != ""
}
