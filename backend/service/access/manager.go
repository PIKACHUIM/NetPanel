package access

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/session"
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
	invalid              bool
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
	rulesLoadErr bool
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

// ResolveIPs 解析规则中的 IP/CIDR 列表（手动输入 + 绑定的 IPDB 条目）。
//
// 面板侧 Gin 中间件与 Caddy 站点级 ACL 都需要这份列表，抽出来避免两处逻辑漂移。
func ResolveIPs(db *gorm.DB, rule model.AccessRule) []string {
	ips, _ := ResolveIPsChecked(db, rule)
	return ips
}

// ResolveIPsChecked 解析 IP/CIDR，并将输入格式或数据库失败反馈给授权调用方。
func ResolveIPsChecked(db *gorm.DB, rule model.AccessRule) ([]string, error) {
	var manualIPs []string
	if rule.IPList != "" {
		if err := json.Unmarshal([]byte(rule.IPList), &manualIPs); err != nil || manualIPs == nil {
			return nil, fmt.Errorf("invalid manual IP list in access rule %d", rule.ID)
		}
	}

	var ipdbIPs []string
	if rule.BindIPDBIDs != "" {
		var ipdbIDs []uint
		if err := json.Unmarshal([]byte(rule.BindIPDBIDs), &ipdbIDs); err != nil || ipdbIDs == nil {
			return nil, fmt.Errorf("invalid IPDB binding in access rule %d", rule.ID)
		}
		if len(ipdbIDs) > 0 {
			var entries []model.IPDBEntry
			if err := db.Where("id IN ?", ipdbIDs).Find(&entries).Error; err != nil {
				return nil, fmt.Errorf("load IPDB entries for access rule %d: %w", rule.ID, err)
			}
			if len(entries) != len(uniqueUintIDs(ipdbIDs)) {
				return nil, fmt.Errorf("missing IPDB entry in access rule %d", rule.ID)
			}
			for _, entry := range entries {
				added := 0
				for _, cidr := range strings.Split(entry.CIDR, ",") {
					if cidr = strings.TrimSpace(cidr); cidr != "" {
						ipdbIPs = append(ipdbIPs, cidr)
						added++
					}
				}
				if added == 0 {
					return nil, fmt.Errorf("empty IPDB entry %d in access rule %d", entry.ID, rule.ID)
				}
			}
		}
	}

	ips := append(manualIPs, ipdbIPs...)
	for _, ip := range ips {
		if net.ParseIP(ip) == nil {
			if _, _, err := net.ParseCIDR(ip); err != nil {
				return nil, fmt.Errorf("invalid IP/CIDR %q in access rule %d", ip, rule.ID)
			}
		}
	}
	return ips, nil
}

func uniqueUintIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	unique := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func (m *Manager) loadRules() {
	var rules []model.AccessRule
	if err := m.db.Where("enable = ?", true).Find(&rules).Error; err != nil {
		m.log.Errorf("[访问控制] 加载规则失败，暂时拒绝所有请求: %v", err)
		m.mu.Lock()
		m.rules = nil
		m.rulesLoadErr = true
		m.mu.Unlock()
		return
	}

	resolved := make([]resolvedRule, 0, len(rules))
	for _, rule := range rules {
		r := resolvedRule{AccessRule: rule}

		// 1~3. 解析手动输入与 IPDB 绑定的 IP 并合并（与 Caddy 站点级 ACL 共用同一实现）
		ips, err := ResolveIPsChecked(m.db, rule)
		if err != nil {
			m.log.Errorf("[访问控制] 规则 %d 数据无效，将失败关闭: %v", rule.ID, err)
			r.invalid = true
		}
		r.AllIPs = ips
		if r.Mode == "" && len(ips) > 0 {
			r.Mode = "blacklist"
		}
		switch r.Mode {
		case "", "blacklist":
		case "whitelist":
			if len(ips) == 0 {
				m.log.Errorf("[访问控制] 白名单规则 %d 未配置任何 IP，将失败关闭", rule.ID)
				r.invalid = true
			}
		default:
			m.log.Errorf("[访问控制] 规则 %d 的 IP 策略无效，将失败关闭", rule.ID)
			r.invalid = true
		}

		// 4. 解析绑定的站点；无效 JSON、查询失败或目标不存在时必须失败关闭。
		if strings.TrimSpace(rule.BindSiteIDs) != "" {
			var siteIDs []uint
			if err := json.Unmarshal([]byte(rule.BindSiteIDs), &siteIDs); err != nil || siteIDs == nil || len(siteIDs) == 0 {
				m.log.Errorf("[访问控制] 规则 %d 的站点绑定无效，将失败关闭", rule.ID)
				r.invalid = true
			} else {
				siteIDs = uniqueUintIDs(siteIDs)
				var sites []model.CaddySite
				if err := m.db.Where("id IN ?", siteIDs).Find(&sites).Error; err != nil || len(sites) != len(siteIDs) {
					m.log.Errorf("[访问控制] 规则 %d 的站点绑定无法完整解析，将失败关闭: %v", rule.ID, err)
					r.invalid = true
				} else {
					for _, site := range sites {
						r.BindSites = append(r.BindSites, siteMatch{Domain: site.Domain, Port: site.Port})
					}
				}
			}
		}

		if r.AuthMode != "" && r.AuthMode != "basic_auth" && r.AuthMode != "page_login" && r.AuthMode != "oidc" {
			m.log.Errorf("[访问控制] 规则 %d 的认证模式无效，将失败关闭", rule.ID)
			r.invalid = true
		}
		if r.AuthMode == "oidc" && strings.TrimSpace(r.BindSiteIDs) == "" {
			m.log.Errorf("[访问控制] OIDC 规则 %d 未绑定站点，将失败关闭", rule.ID)
			r.invalid = true
		}

		// 5. 解析允许的用户 ID 列表
		if rule.AllowedUserIDs != "" {
			if err := json.Unmarshal([]byte(rule.AllowedUserIDs), &r.ParsedAllowedUserIDs); err != nil || r.ParsedAllowedUserIDs == nil {
				m.log.Errorf("[访问控制] 规则 %d 的用户白名单无效，将失败关闭", rule.ID)
				r.invalid = true
			}
		}

		resolved = append(resolved, r)
	}

	m.mu.Lock()
	m.rules = resolved
	m.rulesLoadErr = false
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

		// 使用 gin 的 ClientIP：其是否采信 X-Forwarded-For 由
		// engine.SetTrustedProxies 决定（本项目默认不信任任何代理）。
		// 原先此处自实现并直接取 XFF 首个值，任何客户端都能伪造来源 IP，
		// 使白名单形同虚设、黑名单可被绕过。
		clientIP := c.ClientIP()
		requestHost := c.Request.Host // 包含域名和端口

		m.mu.RLock()
		rules := m.rules
		rulesLoadErr := m.rulesLoadErr
		m.mu.RUnlock()
		if rulesLoadErr {
			c.Header("Cache-Control", "no-store")
			c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "访问控制规则暂不可用"})
			c.Abort()
			return
		}

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
			if rule.invalid {
				c.Header("Cache-Control", "no-store")
				c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": "访问控制规则配置无效"})
				c.Abort()
				return
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
	cookie, err := c.Cookie(session.CookieName)
	if err != nil || cookie == "" {
		// 重定向到登录页面（带回跳地址）
		redirectURL := fmt.Sprintf("/login?redirect=%s", url.QueryEscape(c.Request.URL.RequestURI()))
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		c.Abort()
		return false
	}

	// 验证 session cookie（使用 HMAC 签名验证）
	sessionData, valid := session.ValidateData(cookie)
	if !valid {
		redirectURL := fmt.Sprintf("/login?redirect=%s", url.QueryEscape(c.Request.URL.RequestURI()))
		c.Redirect(http.StatusTemporaryRedirect, redirectURL)
		c.Abort()
		return false
	}

	// 查找用户
	var user model.User
	if err := m.db.Where("username = ? AND enable = ?", sessionData.Username, true).First(&user).Error; err != nil || user.TokenVersion != sessionData.TokenVersion {
		redirectURL := fmt.Sprintf("/login?redirect=%s", url.QueryEscape(c.Request.URL.RequestURI()))
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
