package caddy

import (
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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
	// AllowedHosts 受保护站点的 Host 白名单，防止 Host 头注入重定向地址。
	AllowedHosts []string `json:"allowed_hosts,omitempty"`
	// OAuthProvider 未授权时用于跳转的 Provider；为空时返回面板登录页。
	OAuthProvider string `json:"oauth_provider,omitempty"`
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
	authenticated, authorized := m.authenticate(r)
	if authenticated && authorized {
		return next.ServeHTTP(w, r)
	}
	if authenticated {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("You are not allowed to access this resource.\n"))
		return nil
	}

	loginURL, validHost := m.loginURL(r)
	w.Header().Set("Cache-Control", "no-store")
	if !validHost {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("The request host does not match this protected site.\n"))
		return nil
	}
	w.Header().Set("Location", loginURL)
	// 避免中间代理缓存这条重定向，否则登录后仍会被打回登录页。
	w.WriteHeader(http.StatusFound)
	return nil
}

// authenticate 校验签名会话和用户状态，并单独返回用户是否被该规则授权。
func (m *SessionAuthMiddleware) authenticate(r *http.Request) (bool, bool) {
	c, err := r.Cookie(session.CookieName)
	if err != nil || c.Value == "" {
		return false, false
	}
	sessionData, ok := session.ValidateData(c.Value)
	if !ok {
		return false, false
	}

	db := getAuthDB()
	if db == nil {
		return false, false
	}
	var user model.User
	if err := db.Where("username = ? AND enable = ?", sessionData.Username, true).First(&user).Error; err != nil {
		return false, false
	}
	if sessionData.TokenVersion != user.TokenVersion {
		return false, false
	}

	if len(m.AllowedUserIDs) == 0 {
		return true, true
	}
	for _, id := range m.AllowedUserIDs {
		if id == user.ID {
			return true, true
		}
	}
	return true, false
}

// loginURL 生成只指向已配置站点 Host 的登录页地址。
func (m *SessionAuthMiddleware) loginURL(r *http.Request) (string, bool) {
	requestHost := r.Host
	if host, _, err := net.SplitHostPort(requestHost); err == nil {
		requestHost = host
	} else if strings.HasPrefix(requestHost, "[") && strings.HasSuffix(requestHost, "]") {
		requestHost = strings.TrimSuffix(strings.TrimPrefix(requestHost, "["), "]")
	}
	requestHost = strings.ToLower(strings.TrimSuffix(requestHost, "."))
	if requestHost == "" || strings.ContainsAny(requestHost, "@%/\\") {
		return "", false
	}
	var trustedHost string
	for _, configured := range m.AllowedHosts {
		configured = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(configured), "."))
		if configured == requestHost {
			trustedHost = configured
			break
		}
		if strings.HasPrefix(configured, "*.") {
			suffix := configured[1:]
			prefix := strings.TrimSuffix(requestHost, suffix)
			if strings.HasSuffix(requestHost, suffix) && prefix != "" && !strings.Contains(prefix, ".") {
				trustedHost = requestHost
				break
			}
		}
	}
	if trustedHost == "" {
		return "", false
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	port := m.PanelPort
	if port <= 0 {
		port = 8080
	}
	query := url.Values{}
	query.Set("redirect", scheme+"://"+r.Host+r.URL.RequestURI())
	target := url.URL{
		Scheme:   scheme,
		Host:     net.JoinHostPort(trustedHost, strconv.Itoa(port)),
		Path:     "/login",
		RawQuery: query.Encode(),
	}
	if strings.Contains(trustedHost, ":") {
		target.Host = net.JoinHostPort(trustedHost, strconv.Itoa(port))
	}
	if m.OAuthProvider != "" {
		providerPath := "/api/v1/auth/oauth/" + m.OAuthProvider + "/authorize"
		target.Path = providerPath
		target.RawPath = "/api/v1/auth/oauth/" + url.PathEscape(m.OAuthProvider) + "/authorize"
	}
	return target.String(), true
}

var _ caddyhttp.MiddlewareHandler = (*SessionAuthMiddleware)(nil)
