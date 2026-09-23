package caddy

import (
	"net"
	"net/http"
	"strings"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(IPACLMiddleware{})
}

// IPACLMiddleware 站点级 IP 黑/白名单校验中间件。
//
// 背景：访问控制规则可绑定到网站服务（Caddy 站点），但此前该绑定只用于
// AuthMode，Mode/IPList/BindIPDBIDs 在 Caddy 侧从未被消费。而反代站点由 Caddy
// 直接对外服务，请求根本不会经过面板的 Gin 中间件，导致「站点级 IP 黑白名单」
// 完全不生效。该中间件补上判定，使访问控制在站点层真正生效。
//
// 判定使用连接的 remote_ip，不读取 X-Forwarded-For，避免被请求头伪造绕过。
type IPACLMiddleware struct {
	// Mode blacklist（命中列表即拒绝）/ whitelist（不在列表即拒绝）
	Mode string `json:"mode,omitempty"`
	// Ranges 允许/拒绝的 IP 或 CIDR 列表
	Ranges []string `json:"ranges,omitempty"`
}

// CaddyModule 注册模块 ID。
func (IPACLMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.netpanel_ip_acl",
		New: func() caddy.Module { return new(IPACLMiddleware) },
	}
}

// ServeHTTP 实现 caddyhttp.MiddlewareHandler。
func (m *IPACLMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if m.Mode == "" && len(m.Ranges) == 0 {
		return next.ServeHTTP(w, r)
	}

	ip := remoteIP(r)

	deny := false
	if ip == nil {
		// 无法解析对端地址：白名单语义下保守拒绝
		deny = m.Mode == "whitelist"
	} else {
		matched := m.match(ip)
		switch m.Mode {
		case "whitelist":
			deny = !matched
		default: // blacklist 或未设置
			deny = matched
		}
	}

	if deny {
		// 避免中间代理缓存该响应
		w.Header().Set("Cache-Control", "no-store")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil
	}
	return next.ServeHTTP(w, r)
}

// match 判断 IP 是否命中任一 IP/CIDR。
func (m *IPACLMiddleware) match(ip net.IP) bool {
	for _, raw := range m.Ranges {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "/") {
			if _, ipNet, err := net.ParseCIDR(raw); err == nil && ipNet.Contains(ip) {
				return true
			}
			continue
		}
		if parsed := net.ParseIP(raw); parsed != nil && parsed.Equal(ip) {
			return true
		}
	}
	return false
}

// remoteIP 从连接的 RemoteAddr 解析客户端 IP。
func remoteIP(r *http.Request) net.IP {
	host := r.RemoteAddr
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		host = h
	}
	return net.ParseIP(host)
}

var _ caddyhttp.MiddlewareHandler = (*IPACLMiddleware)(nil)
