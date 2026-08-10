package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/caddyauth"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/fileserver"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/headers"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp/reverseproxy"
	_ "github.com/caddyserver/caddy/v2/modules/caddytls"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const caddyAdminAddr = "localhost:2019"

// Manager Caddy 网站服务管理器
type Manager struct {
	db        *gorm.DB
	log       *logrus.Logger
	dataDir   string
	panelPort int // NetPanel 面板监听端口
	mu        sync.Mutex
	started   bool
	adminHTTP *http.Client
}

func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{
		db:        db,
		log:       log,
		dataDir:   dataDir,
		panelPort: 8080, // 默认值
		adminHTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SetPanelPort 设置 NetPanel 面板的实际监听端口（用于 page_login 重定向）
func (m *Manager) SetPanelPort(port int) {
	m.panelPort = port
}

// StartAll 启动 Caddy 引擎并加载所有已启用站点（异步，不阻塞主进程）
func (m *Manager) StartAll() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				m.log.Errorf("[Caddy] StartAll panic: %v", r)
			}
		}()

		var sites []model.CaddySite
		m.db.Where("enable = ?", true).Find(&sites)
		if len(sites) == 0 {
			return
		}

		if err := m.ensureCaddyRunning(); err != nil {
			m.log.Errorf("[Caddy] 启动引擎失败: %v", err)
			return
		}

		for _, s := range sites {
			if err := m.Start(s.ID); err != nil {
				m.log.Errorf("[Caddy] 站点 [%s] 启动失败: %v", s.Name, err)
			}
		}
	}()
}

// StopAll 停止所有站点并关闭 Caddy 引擎
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return
	}

	// 清空所有路由
	m.adminRequest("DELETE", "/config/apps/http/servers", nil)

	caddy.Stop()
	m.started = false
	m.log.Info("[Caddy] 引擎已停止")

	// 更新所有站点状态
	m.db.Model(&model.CaddySite{}).Where("1 = 1").Update("status", "stopped")
}

// Start 启动指定站点
func (m *Manager) Start(id uint) error {
	var site model.CaddySite
	if err := m.db.First(&site, id).Error; err != nil {
		return fmt.Errorf("站点不存在: %w", err)
	}
	if !site.Enable {
		return fmt.Errorf("站点 [%s] 未启用", site.Name)
	}

	if err := m.ensureCaddyRunning(); err != nil {
		return fmt.Errorf("Caddy 引擎未就绪: %w", err)
	}

	// 构建路由配置
	routes, err := m.buildRoutes(&site)
	if err != nil {
		m.setError(id, err.Error())
		return fmt.Errorf("构建路由配置失败: %w", err)
	}

	// 通过 Admin API 添加路由
	serverKey := fmt.Sprintf("netpanel_%d", id)
	serverCfg := m.buildServerConfig(&site, routes)

	// 输出调试日志：打印实际发送给 Caddy 的配置
	if cfgJSON, err := json.MarshalIndent(serverCfg, "", "  "); err == nil {
		m.log.Infof("[Caddy] 站点 [%s] 配置:\n%s", site.Name, string(cfgJSON))
	}

	// 先删除可能已存在的旧配置（避免 409 key already exists 错误）
	m.adminRequest("DELETE",
		fmt.Sprintf("/config/apps/http/servers/%s", serverKey),
		nil,
	)

	if err := m.adminRequest("PUT",
		fmt.Sprintf("/config/apps/http/servers/%s", serverKey),
		serverCfg,
	); err != nil {
		m.setError(id, err.Error())
		return fmt.Errorf("加载站点配置失败: %w", err)
	}

	m.db.Model(&model.CaddySite{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[Caddy] 站点 [%s] 已启动，监听 :%d", site.Name, site.Port)
	return nil
}

// Stop 停止指定站点
func (m *Manager) Stop(id uint) {
	serverKey := fmt.Sprintf("netpanel_%d", id)
	m.adminRequest("DELETE",
		fmt.Sprintf("/config/apps/http/servers/%s", serverKey),
		nil,
	)
	m.db.Model(&model.CaddySite{}).Where("id = ?", id).Update("status", "stopped")
}

// Restart 重启指定站点
func (m *Manager) Restart(id uint) error {
	m.Stop(id)
	time.Sleep(200 * time.Millisecond)
	return m.Start(id)
}

// UpdateUpstream 动态更新反向代理站点的上游目标并热加载（不落库）。
// 用于自动选线切换：选线结果变化时，把 Caddy 反代目标指向当前线路的入口。
func (m *Manager) UpdateUpstream(id uint, upstream string) error {
	var site model.CaddySite
	if err := m.db.First(&site, id).Error; err != nil {
		return fmt.Errorf("站点不存在: %w", err)
	}
	if site.SiteType != "reverse_proxy" {
		return fmt.Errorf("仅反向代理站点支持动态切换上游，当前类型: %s", site.SiteType)
	}
	if upstream == "" {
		return fmt.Errorf("上游目标地址不能为空")
	}
	if err := m.ensureCaddyRunning(); err != nil {
		return fmt.Errorf("Caddy 引擎未就绪: %w", err)
	}

	// 内存中替换上游目标，不写库（保留用户原始配置，重启后回退）
	site.UpstreamAddr = upstream
	routes, err := m.buildRoutes(&site)
	if err != nil {
		m.setError(id, err.Error())
		return fmt.Errorf("构建路由配置失败: %w", err)
	}
	serverKey := fmt.Sprintf("netpanel_%d", id)
	serverCfg := m.buildServerConfig(&site, routes)

	if err := m.adminRequest("PUT",
		fmt.Sprintf("/config/apps/http/servers/%s", serverKey),
		serverCfg,
	); err != nil {
		m.setError(id, err.Error())
		return fmt.Errorf("热加载站点配置失败: %w", err)
	}
	m.log.Infof("[Caddy] 站点 [%s] 上游目标已切换为 %s", site.Name, upstream)
	return nil
}

// GetStatus 获取站点状态
func (m *Manager) GetStatus(id uint) string {
	var site model.CaddySite
	if err := m.db.First(&site, id).Error; err != nil {
		return "unknown"
	}
	return site.Status
}

// ensureCaddyRunning 确保 Caddy 引擎已启动
func (m *Manager) ensureCaddyRunning() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil
	}

	// 设置 Caddy Admin 监听地址
	adminCfg := &caddy.Config{
		Admin: &caddy.AdminConfig{
			Listen: caddyAdminAddr,
		},
		Logging: &caddy.Logging{
			Logs: map[string]*caddy.CustomLog{
				"default": {
					BaseLog: caddy.BaseLog{
						Level: "INFO",
					},
				},
			},
		},
		AppsRaw: caddy.ModuleMap{
			"http": json.RawMessage(`{"servers":{}}`),
		},
	}

	if err := caddy.Run(adminCfg); err != nil {
		return fmt.Errorf("启动 Caddy 引擎失败: %w", err)
	}

	// 等待 Admin API 就绪
	for i := 0; i < 10; i++ {
		resp, err := m.adminHTTP.Get("http://" + caddyAdminAddr + "/config/")
		if err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	m.started = true
	m.log.Info("[Caddy] 引擎已启动")
	return nil
}

// buildServerConfig 构建 Caddy 服务器配置
func (m *Manager) buildServerConfig(site *model.CaddySite, routes []interface{}) map[string]interface{} {
	listenAddr := fmt.Sprintf(":%d", site.Port)

	serverCfg := map[string]interface{}{
		"listen": []string{listenAddr},
		"routes": routes,
		// 禁用自动 HTTPS 重定向
		"automatic_https": map[string]interface{}{
			"disable": true,
		},
		// 开启访问日志
		"logs": map[string]interface{}{
			"default_logger_name": fmt.Sprintf("netpanel_%d", site.ID),
		},
	}

	// TLS 配置
	if site.TLSEnable {
		tlsCfg := m.buildTLSConfig(site)
		if tlsCfg != nil {
			serverCfg["tls_connection_policies"] = []interface{}{tlsCfg}
		}
	}

	return serverCfg
}

// buildRoute 构建路由配置，返回路由数组
func (m *Manager) buildRoutes(site *model.CaddySite) ([]interface{}, error) {
	// 匹配条件：只有当域名是真实域名（非 localhost、非 IP）时才添加 host matcher
	var hostMatchers []interface{}
	if site.Domain != "" && !isLocalOrIP(site.Domain) {
		hostMatchers = append(hostMatchers, map[string]interface{}{
			"host": []string{site.Domain},
		})
	}

	// 业务处理器
	var mainHandlers []interface{}

	switch site.SiteType {
	case "reverse_proxy":
		if site.UpstreamAddr == "" {
			return nil, fmt.Errorf("反向代理目标地址不能为空")
		}
		dialAddr := normalizeUpstreamDial(site.UpstreamAddr)
		mainHandlers = append(mainHandlers, map[string]interface{}{
			"handler": "reverse_proxy",
			"upstreams": []interface{}{
				map[string]interface{}{"dial": dialAddr},
			},
			"headers": map[string]interface{}{
				"request": map[string]interface{}{
					"set": map[string]interface{}{
						"Host":              []string{"{http.request.host}"},
						"X-Real-IP":         []string{"{http.request.remote.host}"},
						"X-Forwarded-For":   []string{"{http.request.remote.host}"},
						"X-Forwarded-Proto": []string{"{http.request.scheme}"},
					},
				},
			},
			"transport": map[string]interface{}{
				"protocol":      "http",
				"read_timeout":  300000000000,
				"write_timeout": 300000000000,
			},
		})

	case "static":
		if site.RootPath == "" {
			return nil, fmt.Errorf("静态文件根目录不能为空")
		}
		if err := os.MkdirAll(site.RootPath, 0755); err != nil {
			return nil, fmt.Errorf("创建静态文件目录失败: %w", err)
		}
		fileHandler := map[string]interface{}{
			"handler": "file_server",
			"root":    site.RootPath,
		}
		if site.FileList {
			fileHandler["browse"] = map[string]interface{}{}
		}
		mainHandlers = append(mainHandlers, fileHandler)

	case "redirect":
		if site.RedirectTo == "" {
			return nil, fmt.Errorf("重定向目标地址不能为空")
		}
		code := site.RedirectCode
		if code == 0 {
			code = 301
		}
		mainHandlers = append(mainHandlers, map[string]interface{}{
			"handler":     "static_response",
			"status_code": code,
			"headers": map[string]interface{}{
				"Location": []string{site.RedirectTo},
			},
		})

	default:
		return nil, fmt.Errorf("不支持的站点类型: %s", site.SiteType)
	}

	// 查询认证规则
	authMode, authRule := m.findAuthRule(site.ID)

	switch authMode {
	case "basic_auth":
		// Basic Auth: 在 handler 链前面插入 authentication handler
		basicHandler := m.buildBasicAuthHandler(authRule)
		if basicHandler != nil {
			mainHandlers = append([]interface{}{basicHandler}, mainHandlers...)
		}
		route := map[string]interface{}{"handle": mainHandlers}
		if len(hostMatchers) > 0 {
			route["match"] = hostMatchers
		}
		return []interface{}{route}, nil

	case "page_login":
		// 页面跳转登录: 两个 route
		// Route 1: 没有有效 session cookie → 重定向到 NetPanel 登录页
		// Route 2: 有 cookie → 正常代理
		return m.buildPageLoginRoutes(hostMatchers, mainHandlers, site), nil

	default:
		// 无认证
		route := map[string]interface{}{"handle": mainHandlers}
		if len(hostMatchers) > 0 {
			route["match"] = hostMatchers
		}
		return []interface{}{route}, nil
	}
}

// findAuthRule 查找绑定到指定站点的认证规则
func (m *Manager) findAuthRule(siteID uint) (string, model.AccessRule) {
	var rules []model.AccessRule
	m.db.Where("enable = ? AND auth_mode != ''", true).Find(&rules)

	for _, rule := range rules {
		if rule.BindSiteIDs == "" {
			continue
		}
		var siteIDs []uint
		if err := json.Unmarshal([]byte(rule.BindSiteIDs), &siteIDs); err != nil {
			continue
		}
		for _, id := range siteIDs {
			if id == siteID {
				return rule.AuthMode, rule
			}
		}
	}
	return "", model.AccessRule{}
}

// buildPageLoginRoutes 构建页面跳转登录的路由
// 无 cookie 时重定向到 NetPanel 面板的登录页面
func (m *Manager) buildPageLoginRoutes(hostMatchers []interface{}, mainHandlers []interface{}, site *model.CaddySite) []interface{} {
	// 构建 NetPanel 面板登录页的 URL
	// 使用请求的 scheme + 请求的 hostname + 面板端口
	// redirect 参数带上代理站点的完整地址，登录成功后跳回
	panelLogin := fmt.Sprintf("{http.request.scheme}://{http.request.host}:%d/login?redirect={http.request.scheme}://{http.request.hostport}{http.request.uri}", m.panelPort)

	// Route 1: 没有 netpanel_session cookie → 重定向到面板登录页
	redirectRoute := map[string]interface{}{
		"match": []interface{}{
			map[string]interface{}{
				"not": []interface{}{
					map[string]interface{}{
						"header_regexp": map[string]interface{}{
							"Cookie": map[string]interface{}{
								"pattern": "netpanel_session=.+",
							},
						},
					},
				},
			},
		},
		"handle": []interface{}{
			map[string]interface{}{
				"handler":     "static_response",
				"status_code": 302,
				"headers": map[string]interface{}{
					"Location": []string{panelLogin},
				},
			},
		},
	}

	// Route 2: 有 cookie → 正常代理
	proxyRoute := map[string]interface{}{
		"handle": mainHandlers,
	}

	// 如果有 host matcher，添加到两个 route 上
	if len(hostMatchers) > 0 {
		// 对 redirectRoute，合并 host matcher 和 cookie not-match
		redirectRoute["match"] = append(hostMatchers, redirectRoute["match"].([]interface{})...)
		proxyRoute["match"] = hostMatchers
	}

	return []interface{}{redirectRoute, proxyRoute}
}

// buildBasicAuthHandler 构建 Caddy Basic Auth handler
func (m *Manager) buildBasicAuthHandler(rule model.AccessRule) map[string]interface{} {
	// 获取允许的用户列表
	var allowedIDs []uint
	if rule.AllowedUserIDs != "" {
		json.Unmarshal([]byte(rule.AllowedUserIDs), &allowedIDs)
	}

	// 查询用户（如果有指定用户则只查询指定的，否则查询所有启用用户）
	var users []model.User
	if len(allowedIDs) > 0 {
		m.db.Where("id IN ? AND enable = ?", allowedIDs, true).Find(&users)
	} else {
		m.db.Where("enable = ?", true).Find(&users)
	}

	if len(users) == 0 {
		return nil
	}

	// 构建 Caddy 的 basic_auth accounts
	// Caddy 需要 bcrypt hash 格式的密码
	var accounts []interface{}
	for _, u := range users {
		if u.Password == "" {
			continue // OAuth 用户无密码，跳过
		}
		accounts = append(accounts, map[string]interface{}{
			"username": u.Username,
			"password": u.Password, // 已经是 bcrypt hash
		})
	}

	if len(accounts) == 0 {
		return nil
	}

	return map[string]interface{}{
		"handler": "authentication",
		"providers": map[string]interface{}{
			"http_basic": map[string]interface{}{
				"accounts": accounts,
				"hash": map[string]interface{}{
					"algorithm": "bcrypt",
				},
			},
		},
	}
}

// buildTLSConfig 构建 TLS 配置
func (m *Manager) buildTLSConfig(site *model.CaddySite) map[string]interface{} {
	tlsCfg := map[string]interface{}{}

	if site.Domain != "" {
		tlsCfg["match"] = map[string]interface{}{
			"sni": []string{site.Domain},
		}
	}

	switch site.TLSMode {
	case "manual":
		// 手动指定证书文件
		certFile := site.TLSCertFile
		keyFile := site.TLSKeyFile

		// 如果关联了域名证书，从数据库获取路径
		if site.DomainCertID > 0 {
			var cert model.DomainCert
			if err := m.db.First(&cert, site.DomainCertID).Error; err == nil {
				certFile = cert.CertFile
				keyFile = cert.KeyFile
			}
		}

		if certFile != "" && keyFile != "" {
			tlsCfg["certificate_selection"] = map[string]interface{}{
				"any_tag": []string{fmt.Sprintf("cert_%d", site.ID)},
			}
			// 加载证书到 Caddy TLS 存储
			m.loadCertificate(site.ID, certFile, keyFile)
		}

	case "acme":
		// ACME 自动申请
		tlsCfg["certificate_selection"] = map[string]interface{}{
			"any_tag": []string{fmt.Sprintf("acme_%d", site.ID)},
		}

	default: // auto - Caddy 自动管理
		// 不需要额外配置，Caddy 会自动处理
	}

	return tlsCfg
}

// loadCertificate 加载证书到 Caddy
func (m *Manager) loadCertificate(siteID uint, certFile, keyFile string) {
	certData, err := os.ReadFile(certFile)
	if err != nil {
		m.log.Errorf("[Caddy] 读取证书文件失败: %v", err)
		return
	}
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		m.log.Errorf("[Caddy] 读取私钥文件失败: %v", err)
		return
	}

	payload := map[string]interface{}{
		"certificate": string(certData),
		"key":         string(keyData),
		"tags":        []string{fmt.Sprintf("cert_%d", siteID)},
	}

	m.adminRequest("POST", "/certificates", payload)
}

// adminRequest 向 Caddy Admin API 发送请求
func (m *Manager) adminRequest(method, path string, body interface{}) error {
	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(data)
	} else {
		reqBody = bytes.NewReader(nil)
	}

	url := "http://" + caddyAdminAddr + path
	req, err := http.NewRequestWithContext(
		context.Background(),
		method,
		url,
		reqBody,
	)
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := m.adminHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("请求 Caddy Admin API 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("Caddy Admin API 返回错误 %d: %v", resp.StatusCode, errResp)
	}

	return nil
}

// setError 设置站点错误状态
func (m *Manager) setError(id uint, errMsg string) {
	m.db.Model(&model.CaddySite{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": errMsg,
	})
}

// GetCaddyDataDir 获取 Caddy 数据目录
func (m *Manager) GetCaddyDataDir() string {
	return filepath.Join(m.dataDir, "caddy")
}

// normalizeUpstreamDial 将上游地址转换为 Caddy reverse_proxy 的 dial 格式 (host:port)
// 支持输入格式: "http://127.0.0.1:1087", "https://example.com", "127.0.0.1:1087", "example.com"
func normalizeUpstreamDial(addr string) string {
	addr = strings.TrimSpace(addr)

	// 去除协议前缀，提取 scheme 用于默认端口
	scheme := ""
	if strings.HasPrefix(addr, "https://") {
		scheme = "https"
		addr = strings.TrimPrefix(addr, "https://")
	} else if strings.HasPrefix(addr, "http://") {
		scheme = "http"
		addr = strings.TrimPrefix(addr, "http://")
	}

	// 去除路径部分（只取 host:port）
	if idx := strings.Index(addr, "/"); idx != -1 {
		addr = addr[:idx]
	}

	// 如果已经包含端口，直接返回
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}

	// 没有端口，根据 scheme 补充默认端口
	switch scheme {
	case "https":
		return addr + ":443"
	default:
		return addr + ":80"
	}
}

// isLocalOrIP 判断域名是否为 localhost 或 IP 地址（这些不应该作为 host matcher）
func isLocalOrIP(domain string) bool {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "localhost" || domain == "" {
		return true
	}
	// 检查是否为 IP 地址
	if net.ParseIP(domain) != nil {
		return true
	}
	return false
}
