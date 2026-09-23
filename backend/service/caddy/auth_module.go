package caddy

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/session"
	"gorm.io/gorm"
)

func init() {
	caddy.RegisterModule(SessionAuthMiddleware{})
}

var (
	authDBMu sync.RWMutex
	authDB   *gorm.DB
)

// SetAuthDB 注入用于校验用户状态的数据库句柄。
// Caddy 通过 JSON 配置实例化模块，无法直接传递依赖，故用包级变量桥接。
func SetAuthDB(db *gorm.DB) {
	authDBMu.Lock()
	authDB = db
	authDBMu.Unlock()
}

func getAuthDB() *gorm.DB {
	authDBMu.RLock()
	defer authDBMu.RUnlock()
	return authDB
}

// SessionAuthMiddleware 校验 NetPanel 平台会话 Cookie 的 Caddy 中间件。
//
// 此前 page_login 模式仅在 Caddy 配置里用 header_regexp 匹配
// `netpanel_session=.+`，任何人手工设置该 Cookie 即可绕过认证。
// 现改为在中间件内做完整的 HMAC 签名与有效期校验，并核对用户仍然存在且启用。
type SessionAuthMiddleware struct {
	// PanelPort NetPanel 面板监听端口，用于拼装登录页地址
	PanelPort int `json:"panel_port,omitempty"`
	// AllowedUserIDs 允许访问的用户 ID 列表，为空表示所有已登录用户均可访问
	AllowedUserIDs []uint `json:"allowed_user_ids,omitempty"`
}

func (SessionAuthMiddleware) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.netpanel_session_auth",
		New: func() caddy.Module { return new(SessionAuthMiddleware) },
	}
}

func (m *SessionAuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	if m.authorized(r) {
		return next.ServeHTTP(w, r)
	}

	w.Header().Set("Location", m.loginURL(r))
	// 避免中间代理缓存这条重定向，否则登录后仍会被打回登录页
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusFound)
	return nil
}

// authorized 校验会话 Cookie 并确认用户仍处于启用状态且在允许列表内。
func (m *SessionAuthMiddleware) authorized(r *http.Request) bool {
	c, err := r.Cookie(session.CookieName)
	if err != nil || c.Value == "" {
		return false
	}
	username, ok := session.Validate(c.Value)
	if !ok {
		return false
	}

	db := getAuthDB()
	if db == nil {
		// 依赖未注入时保守拒绝：宁可不可用，也不能放行未经核实的会话
		return false
	}
	var user model.User
	if err := db.Where("username = ? AND enable = ?", username, true).First(&user).Error; err != nil {
		return false
	}

	if len(m.AllowedUserIDs) == 0 {
		return true
	}
	for _, id := range m.AllowedUserIDs {
		if id == user.ID {
			return true
		}
	}
	return false
}

// loginURL 生成面板登录页地址，并把当前请求地址作为 redirect 参数带上。
// redirect 值经过 URL 编码，防止站点 URI 中的 & / # 截断查询串。
func (m *SessionAuthMiddleware) loginURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		host = h
	}
	back := url.QueryEscape(scheme + "://" + r.Host + r.URL.RequestURI())
	return fmt.Sprintf("%s://%s:%d/login?redirect=%s", scheme, host, m.PanelPort, back)
}

var _ caddyhttp.MiddlewareHandler = (*SessionAuthMiddleware)(nil)
