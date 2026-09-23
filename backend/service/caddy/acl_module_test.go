package caddy

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
)

// nextHandler 记录是否被放行到业务处理链。
type nextHandler struct{ called bool }

func (h *nextHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) error {
	h.called = true
	w.WriteHeader(http.StatusOK)
	return nil
}

func doACL(t *testing.T, mw *IPACLMiddleware, remoteAddr, xff string) (int, bool) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = remoteAddr
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	next := &nextHandler{}
	rec := httptest.NewRecorder()
	if err := mw.ServeHTTP(rec, req, caddyhttp.HandlerFunc(next.ServeHTTP)); err != nil {
		t.Fatalf("ServeHTTP 返回错误: %v", err)
	}
	return rec.Code, next.called
}

func TestIPACLBlacklist(t *testing.T) {
	mw := &IPACLMiddleware{Mode: "blacklist", Ranges: []string{"203.0.113.7", "10.0.0.0/8"}}

	if code, called := doACL(t, mw, "203.0.113.7:1234", ""); code != http.StatusForbidden || called {
		t.Fatalf("黑名单命中 IP 应被拒绝: code=%d called=%v", code, called)
	}
	if code, called := doACL(t, mw, "10.1.2.3:1234", ""); code != http.StatusForbidden || called {
		t.Fatalf("黑名单 CIDR 命中应被拒绝: code=%d called=%v", code, called)
	}
	if code, called := doACL(t, mw, "198.51.100.9:1234", ""); code != http.StatusOK || !called {
		t.Fatalf("黑名单未命中的 IP 应放行: code=%d called=%v", code, called)
	}
}

func TestIPACLWhitelist(t *testing.T) {
	mw := &IPACLMiddleware{Mode: "whitelist", Ranges: []string{"192.168.1.0/24"}}

	if code, called := doACL(t, mw, "192.168.1.20:5555", ""); code != http.StatusOK || !called {
		t.Fatalf("白名单内的 IP 应放行: code=%d called=%v", code, called)
	}
	if code, called := doACL(t, mw, "8.8.8.8:5555", ""); code != http.StatusForbidden || called {
		t.Fatalf("白名单之外的 IP 应被拒绝: code=%d called=%v", code, called)
	}
}

// X-Forwarded-For 由客户端完全可控，必须不能影响判定结果。
func TestIPACLIgnoresForwardedFor(t *testing.T) {
	deny := &IPACLMiddleware{Mode: "whitelist", Ranges: []string{"192.168.1.0/24"}}
	if code, called := doACL(t, deny, "8.8.8.8:1000", "192.168.1.20"); code != http.StatusForbidden || called {
		t.Fatalf("伪造 XFF 不应绕过白名单: code=%d called=%v", code, called)
	}

	bl := &IPACLMiddleware{Mode: "blacklist", Ranges: []string{"8.8.8.8"}}
	if code, called := doACL(t, bl, "8.8.8.8:1000", "1.2.3.4"); code != http.StatusForbidden || called {
		t.Fatalf("伪造 XFF 不应绕过黑名单: code=%d called=%v", code, called)
	}
}

// 空配置（无 mode 无 ranges）应直接放行，避免误伤未配置 ACL 的站点。
func TestIPACLEmptyPassthrough(t *testing.T) {
	mw := &IPACLMiddleware{}
	if code, called := doACL(t, mw, "8.8.8.8:1000", ""); code != http.StatusOK || !called {
		t.Fatalf("空 ACL 配置应放行: code=%d called=%v", code, called)
	}
}
