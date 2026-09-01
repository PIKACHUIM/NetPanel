package access

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/pkg/secret"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/utils"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// resolvedRule 预解析后的访问控制规则（包含从 IPDB 解析出的 IP 列表）
type resolvedRule struct {
	model.AccessRule
	// 合并后的 IP 列表（手动输入 + IPDB 条目）
	AllIPs []string
	// 绑定的站点域名/端口列表（用于匹配请求）
	BindSites []siteMatch
	// 解析后的允许用户 ID 列表
	ParsedAllowedUserIDs []uint
}

// siteMatch 站点匹配信息
type siteMatch struct {
	Domain string
	Port   int
}

// Manager 访问控制管理器
type Manager struct {
	db           *gorm.DB
	log          *logrus.Logger
	rules        []resolvedRule
	mu           sync.RWMutex
	// excludePaths 不受访问控制影响的路径前缀（可通过 SetExcludePaths 配置）
	excludePaths []string
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	m := &Manager{
		db:  db,
		log: log,
		// 默认不豁免任何路径，所有请求均受访问控制
		excludePaths: []string{},
	}
	m.loadRules()
	return m
}

// SetExcludePaths 设置不受访问控制影响的路径前缀列表
// 例如：["/api/v1/system/login"] 使登录接口不受访问控制影响
func (m *Manager) SetExcludePaths(paths []string) {
	m.mu.Lock()
	m.excludePaths = paths
	m.mu.Unlock()
}

func (m *Manager) loadRules() {
	var rules []model.AccessRule
	m.db.Where("enable = ?", true).Find(&rules)

	resolved := make([]resolvedRule, 0, len(rules))
	for _, rule := range rules {
		r := resolvedRule{AccessRule: rule}

		// 1. 解析手动输入的 IP 列表
		var manualIPs []string
		if rule.IPList != "" {
			json.Unmarshal([]byte(rule.IPList), &manualIPs)
		}

		// 2. 从 IPDB 条目获取 IP/CIDR（一条记录可能包含多个逗号分隔的 IP/CIDR）
		var ipdbIPs []string
		if rule.BindIPDBIDs != "" {
			var ipdbIDs []uint
			if err := json.Unmarshal([]byte(rule.BindIPDBIDs), &ipdbIDs); err == nil && len(ipdbIDs) > 0 {
				var entries []model.IPDBEntry
				m.db.Where("id IN ?", ipdbIDs).Find(&entries)
				for _, e := range entries {
					if e.CIDR != "" {
						// 拆分逗号分隔的多个 IP/CIDR
						for _, cidr := range strings.Split(e.CIDR, ",") {
							cidr = strings.TrimSpace(cidr)
							if cidr != "" {
								ipdbIPs = append(ipdbIPs, cidr)
							}
						}
					}
				}
			}
		}

		// 3. 合并所有 IP
		r.AllIPs = append(manualIPs, ipdbIPs...)

		// 4. 解析绑定的站点
		if rule.BindSiteIDs != "" {
			var siteIDs []uint
			if err := json.Unmarshal([]byte(rule.BindSiteIDs), &siteIDs); err == nil && len(siteIDs) > 0 {
				var sites []model.CaddySite
				m.db.Where("id IN ?", siteIDs).Find(&sites)
				for _, s := range sites {
					r.BindSites = append(r.BindSites, siteMatch{
						Domain: s.Domain,
						Port:   s.Port,
					})
				}
			}
		}

		// 5. 解析允许的用户 ID 列表
		if rule.AllowedUserIDs != "" {
			json.Unmarshal([]byte(rule.AllowedUserIDs), &r.ParsedAllowedUserIDs)
		}

		resolved = append(resolved, r)
	}

	m.mu.Lock()
	m.rules = resolved
	m.mu.Unlock()
}

func (m *Manager) Reload() {
	m.loadRules()
}

func (m *Manager) SetGinEngine(r *gin.Engine) {
	r.Use(m.GinMiddleware())
}

// GinMiddleware 访问控制 Gin 中间件
func (m *Manager) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否在豁免路径列表中
		m.mu.RLock()
		excludePaths := m.excludePaths
		m.mu.RUnlock()

		path := c.Request.URL.Path
		for _, ep := range excludePaths {
			if strings.HasPrefix(path, ep) {
				c.Next()
				return
			}
		}

		clientIP := getClientIP(c.Request)
		requestHost := c.Request.Host // 包含域名和端口

		m.mu.RLock()
		rules := m.rules
		m.mu.RUnlock()

		for _, rule := range rules {
			if !rule.Enable {
				continue
			}

			// 如果绑定了站点，检查当前请求是否匹配绑定的站点
			if len(rule.BindSites) > 0 {
				if !matchRequestSite(requestHost, rule.BindSites) {
					// 当前请求不属于绑定的站点，跳过此规则
					continue
				}
			}

			// === IP 访问控制 ===
			if len(rule.AllIPs) > 0 {
				matched := matchIP(clientIP, rule.AllIPs)

				switch rule.Mode {
				case "blacklist":
					if matched {
						m.log.Warnf("[访问控制] IP %s 在黑名单中，拒绝访问", clientIP)
						c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "访问被拒绝"})
						c.Abort()
						return
					}
				case "whitelist":
					if !matched {
						m.log.Warnf("[访问控制] IP %s 不在白名单中，拒绝访问", clientIP)
						c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "访问被拒绝"})
						c.Abort()
						return
					}
				}
			}

			// === 用户认证策略 ===
			if rule.AuthMode != "" {
				if !m.handleUserAuth(c, rule) {
					return // 已在 handleUserAuth 中 Abort
				}
			}
		}

		c.Next()
	}
}

// handleUserAuth 处理用户认证策略，返回 true 表示认证通过，false 表示已拒绝
func (m *Manager) handleUserAuth(c *gin.Context, rule resolvedRule) bool {
	switch rule.AuthMode {
	case "basic_auth":
		return m.handleBasicAuth(c, rule)
	case "page_login":
		return m.handlePageLogin(c, rule)
	}
	return true
}

// handleBasicAuth 处理 Basic Auth 认证
func (m *Manager) handleBasicAuth(c *gin.Context, rule resolvedRule) bool {
	auth := c.GetHeader("Authorization")
	if auth == "" || !strings.HasPrefix(auth, "Basic ") {
		c.Header("WWW-Authenticate", `Basic realm="NetPanel Access Control"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	// 解码 Basic Auth
	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		c.Header("WWW-Authenticate", `Basic realm="NetPanel Access Control"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		c.Header("WWW-Authenticate", `Basic realm="NetPanel Access Control"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	username, password := parts[0], parts[1]

	// 验证用户
	var user model.User
	if err := m.db.Where("username = ? AND enable = ?", username, true).First(&user).Error; err != nil {
		c.Header("WWW-Authenticate", `Basic realm="NetPanel Access Control"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	// 验证密码
	if !utils.CheckPassword(password, user.Password) {
		c.Header("WWW-Authenticate", `Basic realm="NetPanel Access Control"`)
		c.AbortWithStatus(http.StatusUnauthorized)
		return false
	}

	// 检查用户是否在允许列表中
	if !m.isUserAllowed(user.ID, rule.ParsedAllowedUserIDs) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "您无权访问此站点"})
		c.Abort()
		return false
	}

	return true
}

// handlePageLogin 处理页面跳转登录认证
func (m *Manager) handlePageLogin(c *gin.Context, rule resolvedRule) bool {
	// 检查 session cookie
	cookie, err := c.Cookie("netpanel_session")
	if err != nil || cookie == "" {
		// 重定向到登录页面（带回跳地址）
		redirectURL := fmt.Sprintf("/login?redirect=%s", c.Request.URL.RequestURI())
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		c.Abort()
		return false
	}

	// 验证 session cookie（使用 HMAC 签名验证）
	username, valid := validateSessionCookieForAccess(cookie)
	if !valid {
		redirectURL := fmt.Sprintf("/login?redirect=%s", c.Request.URL.RequestURI())
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		c.Abort()
		return false
	}

	// 查找用户
	var user model.User
	if err := m.db.Where("username = ? AND enable = ?", username, true).First(&user).Error; err != nil {
		redirectURL := fmt.Sprintf("/login?redirect=%s", c.Request.URL.RequestURI())
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		c.Abort()
		return false
	}

	// 检查用户是否在允许列表中
	if !m.isUserAllowed(user.ID, rule.ParsedAllowedUserIDs) {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "您无权访问此站点"})
		c.Abort()
		return false
	}

	return true
}

// isUserAllowed 检查用户是否在允许列表中（空列表表示所有已认证用户都允许）
func (m *Manager) isUserAllowed(userID uint, allowedIDs []uint) bool {
	if len(allowedIDs) == 0 {
		return true // 空列表表示所有已登录用户均可访问
	}
	for _, id := range allowedIDs {
		if id == userID {
			return true
		}
	}
	return false
}

// validateSessionCookieForAccess 验证 session cookie（复用 platform_auth 的逻辑）
// 使用与 middleware/platform_auth.go 相同的 session 派生密钥签名
func validateSessionCookieForAccess(cookie string) (string, bool) {
	parts := strings.SplitN(cookie, ".", 2)
	if len(parts) != 2 {
		return "", false
	}

	payloadHex := parts[0]
	signature := parts[1]

	// 解码 hex payload
	payload, err := hex.DecodeString(payloadHex)
	if err != nil {
		return "", false
	}

	// 验证 HMAC-SHA256 签名（与 middleware.signSession 使用同一 session 派生密钥）
	mac := hmac.New(sha256.New, secret.SessionKey())
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedSig)) != 1 {
		return "", false
	}

	// 解析数据
	var data struct {
		Username  string `json:"u"`
		ExpiresAt int64  `json:"e"`
	}
	if err := json.Unmarshal(payload, &data); err != nil {
		return "", false
	}

	// 检查过期
	if time.Now().Unix() > data.ExpiresAt {
		return "", false
	}

	return data.Username, true
}

// matchRequestSite 检查请求的 Host 是否匹配绑定的站点列表
func matchRequestSite(requestHost string, sites []siteMatch) bool {
	// 解析请求的域名和端口
	host, port, err := net.SplitHostPort(requestHost)
	if err != nil {
		// 没有端口的情况
		host = requestHost
		port = ""
	}

	for _, site := range sites {
		// 域名匹配（忽略大小写）
		if site.Domain != "" && !strings.EqualFold(host, site.Domain) {
			continue
		}
		// 端口匹配
		if site.Port > 0 && port != "" && port != fmt.Sprintf("%d", site.Port) {
			continue
		}
		return true
	}
	return false
}

// getClientIP 获取客户端真实 IP
func getClientIP(r *http.Request) string {
	// 检查 X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}
	// 检查 X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// 使用 RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// matchIP 检查 IP 是否匹配列表（支持 CIDR）
func matchIP(ip string, ipList []string) bool {
	clientIP := net.ParseIP(ip)
	if clientIP == nil {
		return false
	}

	for _, item := range ipList {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}

		// CIDR 匹配
		if strings.Contains(item, "/") {
			_, ipNet, err := net.ParseCIDR(item)
			if err == nil && ipNet.Contains(clientIP) {
				return true
			}
			continue
		}

		// 精确匹配
		if item == ip {
			return true
		}
	}
	return false
}
