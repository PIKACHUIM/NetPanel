package waf

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/corazawaf/coraza/v3"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// Manager WAF 引擎管理器
type Manager struct {
	db      *gorm.DB
	mu      sync.RWMutex
	engines map[uint]*engine
}

type engine struct {
	id  uint
	waf coraza.WAF
}

// Default 默认 WAF 管理器（供 Caddy 中间件与 Handler 使用）
var Default *Manager

// SetDefault 设置全局默认管理器（main.go 初始化时调用）
func SetDefault(m *Manager) {
	Default = m
}

// NewManager 创建 WAF 引擎管理器
func NewManager(db *gorm.DB) *Manager {
	return &Manager{
		db:      db,
		engines: make(map[uint]*engine),
	}
}

// buildDirectives 构建规则指令
func buildDirectives(cfg model.WafConfig) string {
	directives := "SecRuleEngine On\nSecRequestBodyAccess On\nSecRequestBodyLimit 13107200\n"
	if cfg.CustomRules != "" {
		directives += cfg.CustomRules + "\n"
	}
	return directives
}

// Start 启动指定配置的 WAF 引擎
func (m *Manager) Start(id uint) error {
	var cfg model.WafConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("WAF 配置不存在: %w", err)
	}

	w, err := coraza.NewWAF(coraza.NewWAFConfig().WithDirectives(buildDirectives(cfg)))
	if err != nil {
		return fmt.Errorf("构建 WAF 引擎失败: %w", err)
	}

	m.mu.Lock()
	m.engines[id] = &engine{id: id, waf: w}
	m.mu.Unlock()

	return m.db.Model(&model.WafConfig{}).Where("id = ?", id).Updates(map[string]any{
		"enable": true,
		"status": "running",
	}).Error
}

// Stop 停止指定配置的 WAF 引擎
func (m *Manager) Stop(id uint) error {
	m.mu.Lock()
	if e, ok := m.engines[id]; ok {
		_ = e.waf.NewTransaction().Close()
		delete(m.engines, id)
	}
	m.mu.Unlock()

	return m.db.Model(&model.WafConfig{}).Where("id = ?", id).Updates(map[string]any{
		"enable": false,
		"status": "stopped",
	}).Error
}

// TestRule 校验规则语法（返回 nil 表示语法正确）
func (m *Manager) TestRule(rule string) error {
	if strings.TrimSpace(rule) == "" {
		return fmt.Errorf("规则不能为空")
	}
	_, err := coraza.NewWAF(coraza.NewWAFConfig().WithDirectives(rule))
	return err
}

// Check 检查请求是否应被拦截（供 Caddy 中间件调用）
func (m *Manager) Check(id uint, r *http.Request) (blocked bool, err error) {
	m.mu.RLock()
	e, ok := m.engines[id]
	m.mu.RUnlock()
	if !ok {
		return false, fmt.Errorf("WAF 引擎未启动: %d", id)
	}

	tx := e.waf.NewTransaction()
	defer tx.Close()

	clientIP, clientPort := splitHostPort(r.RemoteAddr)
	tx.ProcessConnection(clientIP, clientPort, r.Host, 443)
	tx.ProcessURI(r.URL.String(), r.Method, r.Proto)
	for k, vv := range r.Header {
		for _, v := range vv {
			tx.AddRequestHeader(k, v)
		}
	}
	tx.AddRequestHeader("Host", r.Host)

	if it := tx.ProcessRequestHeaders(); it != nil {
		return true, nil
	}

	// 请求体检查(仅当引擎需要检查 body 时读取,并恢复 body 供下游使用)
	if r.Body != nil && r.Body != http.NoBody {
		it, _, err := tx.ReadRequestBodyFrom(r.Body)
		if err != nil {
			return false, err
		}
		if it != nil {
			return true, nil
		}
		// 恢复请求体,让下游 handler 仍可读取
		if rbr, err := tx.RequestBodyReader(); err == nil {
			r.Body = io.NopCloser(io.MultiReader(rbr, r.Body))
		}
		if it, err := tx.ProcessRequestBody(); err != nil {
			return false, err
		} else if it != nil {
			return true, nil
		}
	}

	return tx.IsInterrupted(), nil
}

// splitHostPort 拆分 host:port，无端口时返回默认值
func splitHostPort(addr string) (string, int) {
	if addr == "" {
		return "", 0
	}
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, 0
	}
	var port int
	fmt.Sscanf(portStr, "%d", &port)
	return host, port
}
