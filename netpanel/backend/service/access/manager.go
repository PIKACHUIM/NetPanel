package access

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager 访问控制管理器
type Manager struct {
	db    *gorm.DB
	log   *logrus.Logger
	rules []model.AccessRule
	mu    sync.RWMutex
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	m := &Manager{db: db, log: log}
	m.loadRules()
	return m
}

func (m *Manager) loadRules() {
	var rules []model.AccessRule
	m.db.Where("enable = ?", true).Find(&rules)
	m.mu.Lock()
	m.rules = rules
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
		// API 路由不受访问控制影响（只控制前端访问）
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}

		clientIP := getClientIP(c.Request)

		m.mu.RLock()
		rules := m.rules
		m.mu.RUnlock()

		for _, rule := range rules {
			if !rule.Enable {
				continue
			}

			var ipList []string
			json.Unmarshal([]byte(rule.IPList), &ipList)

			matched := matchIP(clientIP, ipList)

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

		c.Next()
	}
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
