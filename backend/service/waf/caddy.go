package waf

import (
	"net/http"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

func init() {
	caddy.RegisterModule(WafMiddleware{})
}

// WafMiddleware Caddy HTTP 中间件:在反代之前执行 WAF 检查
type WafMiddleware struct {
	ConfigID int `json:"config_id"`
}

// CaddyModule 注册模块
func (WafMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.netpanel_waf",
		New: func() caddy.Module { return new(WafMiddleware) },
	}
}

// ServeHTTP 拦截被 WAF 规则拒绝的请求
func (m *WafMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if Default == nil {
		return next.ServeHTTP(w, r)
	}
	blocked, err := Default.Check(uint(m.ConfigID), r)
	if err == nil && blocked {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 Forbidden: request blocked by WAF"))
		return nil
	}
	return next.ServeHTTP(w, r)
}

// Interface guard
var _ caddyhttp.MiddlewareHandler = (*WafMiddleware)(nil)
