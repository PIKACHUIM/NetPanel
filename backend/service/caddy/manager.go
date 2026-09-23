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
	"github.com/netpanel/netpanel/service/access"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager Caddy 网站服务管理器
type Manager struct {
	db        *gorm.DB
	log       *logrus.Logger
	dataDir   string
	panelPort int // NetPanel 面板监听端口
	mu        sync.Mutex
	started   bool
	admin     adminEndpoint
	adminHTTP *http.Client
	// syncMu 保护「读库 → 构建配置 → PUT 到 Caddy」整段流程，
	// 否则并发的 Start/Stop 会各自基于旧快照生成配置并互相覆盖
	syncMu sync.Mutex
	// upstreamOverride 内存中的上游覆盖（自动选线切换用，不落库，重启进程后回退）
	overrideMu       sync.Mutex
	upstreamOverride map[uint]string
}

func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	// netpanel_session_auth 中间件由 Caddy 依据 JSON 配置实例化，
	// 拿不到 Manager 上的依赖，这里把 DB 注入到包级桥接变量
	SetAuthDB(db)
	admin := newAdminEndpoint(dataDir)
	return &Manager{
		db:               db,
		log:              log,
		dataDir:          dataDir,
		panelPort:        8080, // 默认值
		admin:            admin,
		adminHTTP:        admin.httpClient(),
		upstreamOverride: make(map[uint]string),
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
	// 与 syncPort 互斥，避免停机过程中有并发的配置热加载
	m.syncMu.Lock()
	defer m.syncMu.Unlock()

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

// serverKeyForPort 根据监听端口生成 Caddy server key。
// 同端口的所有站点共享同一个 server，通过 Host/SNI 匹配路由，避免端口冲突。
func serverKeyForPort(port int) string {
	return fmt.Sprintf("netpanel_port_%d", port)
}

// syncPort 重新构建指定端口下的全部启用站点，合并为单个 Caddy server 并热加载。
// 端口下无启用站点时删除该 server。
func (m *Manager) syncPort(port int) error {
	// 串行化「读库 → 构建 → PUT」：并发的 Start/Stop/UpdateUpstream 命中同一端口时
	// 各自基于不同快照生成配置，后写者会覆盖先写者，导致丢站点或配置回退
	m.syncMu.Lock()
	defer m.syncMu.Unlock()

	var sites []model.CaddySite
	m.db.Where("enable = ? AND port = ?", true, port).Find(&sites)

	key := serverKeyForPort(port)
	path := fmt.Sprintf("/config/apps/http/servers/%s", key)

	// 无启用站点 → 删除该端口 server
	if len(sites) == 0 {
		m.adminRequest("DELETE", path, nil)
		return nil
	}

	// 一次性加载 WAF 与认证规则的绑定关系，避免每个站点各自全表扫描
	deps := m.loadRouteDeps(sites)

	// 排序：有 host 匹配（真实域名）的站点在前，无域名的站点（localhost/IP/空）
	// 必须排最后——它们会生成无条件匹配路由，若靠前会吞掉同端口其他站点的流量
	sites = sortSitesForRouting(sites)

	// 同端口下最多允许一个无域名站点，否则互斥路由会静默失效
	wildcard := countWildcardSites(sites)
	if wildcard > 1 {
		for i := range sites {
			m.setError(sites[i].ID, fmt.Sprintf("端口 %d 下存在 %d 个无域名站点，仅允许一个（否则会互相抢占流量）", port, wildcard))
		}
		return fmt.Errorf("端口 %d 下存在 %d 个无域名站点，仅允许一个", port, wildcard)
	}

	// 合并所有站点的路由与 TLS 策略
	var routes []interface{}
	var tlsPolicies []interface{}
	// 成功构建的站点 ID：只对它们置 running，构建失败的不被误标
	builtIDs := make([]uint, 0, len(sites))
	for i := range sites {
		site := &sites[i]
		// 应用内存中的上游覆盖（自动选线切换）
		m.overrideMu.Lock()
		if ov, ok := m.upstreamOverride[site.ID]; ok && ov != "" {
			site.UpstreamAddr = ov
		}
		m.overrideMu.Unlock()

		r, err := m.buildRoutes(site, deps)
		if err != nil {
			// 单个站点构建失败不拖垮整个端口，标记错误后跳过
			m.setError(site.ID, err.Error())
			continue
		}
		routes = append(routes, r...)
		builtIDs = append(builtIDs, site.ID)
		if site.TLSEnable {
			if tls := m.buildTLSConfig(site); tls != nil {
				tlsPolicies = append(tlsPolicies, tls)
			}
		}
	}

	if len(routes) == 0 {
		m.adminRequest("DELETE", path, nil)
		return nil
	}

	serverCfg := map[string]interface{}{
		"listen": []string{fmt.Sprintf(":%d", port)},
		"routes": routes,
		// 禁用自动 HTTPS 重定向
		"automatic_https": map[string]interface{}{
			"disable": true,
		},
		// 开启访问日志
		"logs": map[string]interface{}{
			"default_logger_name": key,
		},
	}
	if len(tlsPolicies) > 0 {
		// 带 SNI 的策略在前，无 match 的兜底策略在最后，
		// 否则无 match 策略会被 Caddy 当作第一个匹配项而让后续 SNI 策略失效
		serverCfg["tls_connection_policies"] = orderTLSPolicies(tlsPolicies)
	}

	// 输出调试日志
	if cfgJSON, err := json.MarshalIndent(serverCfg, "", "  "); err == nil {
		m.log.Infof("[Caddy] 端口 %d 配置:\n%s", port, string(cfgJSON))
	}

	if err := m.adminRequest("PUT", path, serverCfg); err != nil {
		// 标记该端口所有站点为 error
		for i := range sites {
			m.setError(sites[i].ID, err.Error())
		}
		return fmt.Errorf("加载端口 %d 站点配置失败: %w", port, err)
	}

	// 仅标记成功构建的站点为 running
	m.db.Model(&model.CaddySite{}).Where("id IN ?", builtIDs).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	return nil
}

// Start 启动指定站点（要求已启用），并同步其所在端口的全部站点
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

	// 应用内存中的上游覆盖（自动选线切换），使预校验与 syncPort 一致
	m.overrideMu.Lock()
	if ov, ok := m.upstreamOverride[id]; ok && ov != "" {
		site.UpstreamAddr = ov
	}
	m.overrideMu.Unlock()

	// 预校验单站点配置，提前暴露构建错误
	if _, err := m.buildRoutes(&site, m.loadRouteDeps([]model.CaddySite{site})); err != nil {
		m.setError(id, err.Error())
		return fmt.Errorf("构建路由配置失败: %w", err)
	}

	if err := m.syncPort(site.Port); err != nil {
		return err
	}

	m.log.Infof("[Caddy] 站点 [%s] 已启动，监听 :%d", site.Name, site.Port)
	return nil
}

// Stop 停止指定站点：仅负责运行态（置 stopped 并同步所在端口），
// 持久化启用开关（enable）由调用方（handler）负责，避免语义与 StopAll 不一致。
func (m *Manager) Stop(id uint) {
	var site model.CaddySite
	if err := m.db.First(&site, id).Error; err != nil {
		return
	}
	port := site.Port

	// 清理内存中的上游覆盖
	m.overrideMu.Lock()
	delete(m.upstreamOverride, id)
	m.overrideMu.Unlock()

	m.db.Model(&model.CaddySite{}).Where("id = ?", id).Update("status", "stopped")

	if err := m.syncPort(port); err != nil {
		m.log.Errorf("[Caddy] 站点 [%s] 停止后同步端口 %d 失败: %v", site.Name, port, err)
	}
}

// Restart 重启指定站点（重新同步其所在端口）
func (m *Manager) Restart(id uint) error {
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
	if err := validateUpstream(upstream); err != nil {
		return err
	}
	if err := m.ensureCaddyRunning(); err != nil {
		return fmt.Errorf("Caddy 引擎未就绪: %w", err)
	}

	// 内存中替换上游目标，不写库（保留用户原始配置，重启后回退）
	m.overrideMu.Lock()
	m.upstreamOverride[id] = upstream
	m.overrideMu.Unlock()

	if err := m.syncPort(site.Port); err != nil {
		return err
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
			Listen:        m.admin.listen,
			EnforceOrigin: true,
			Origins:       m.admin.origins(),
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
		resp, err := m.adminHTTP.Get(m.admin.baseURL + "/config/")
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

// buildRoute 构建路由配置，返回路由数组。
// deps 为预先加载的 WAF/认证绑定关系，可为 nil（此时跳过 WAF 与认证查询）。
func (m *Manager) buildRoutes(site *model.CaddySite, deps *routeDeps) ([]interface{}, error) {
	// 匹配条件：只有当域名是真实域名（非 localhost、非 IP）时才添加 host matcher
	var hostMatchers []interface{}
	if hasHostMatcher(site.Domain) {
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
		root, err := ValidateRootPath(site.RootPath)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(root, 0755); err != nil {
			return nil, fmt.Errorf("创建静态文件目录失败: %w", err)
		}
		fileHandler := map[string]interface{}{
			"handler": "file_server",
			"root":    root,
		}
		if site.FileList {
			fileHandler["browse"] = map[string]interface{}{}
		}
		mainHandlers = append(mainHandlers, fileHandler)

	case "redirect":
		if err := ValidateRedirectTarget(site.RedirectTo); err != nil {
			return nil, err
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

	// WAF:站点绑定启用的 WAF 配置时,在 handler 链最前插入 WAF 检查中间件
	if deps != nil {
		if wafCfgID := deps.waf[site.ID]; wafCfgID != 0 {
			mainHandlers = append([]interface{}{map[string]interface{}{
				"handler":   "netpanel_waf",
				"config_id": wafCfgID,
			}}, mainHandlers...)
		}
	}

	// 站点级 IP 黑白名单：绑定了网站的反代站点由 Caddy 直接对外服务，
	// 请求不经过面板的 Gin 中间件，因此必须在 Caddy 侧判定。
	// 放在 handler 链最前，使拒绝尽早发生。
	if deps != nil {
		if b, ok := deps.acl[site.ID]; ok && len(b.ips) > 0 {
			mainHandlers = append([]interface{}{map[string]interface{}{
				"handler": "netpanel_ip_acl",
				"mode":    b.mode,
				"ranges":  b.ips,
			}}, mainHandlers...)
		}
	}

	// 查询认证规则
	var authMode string
	var authRule model.AccessRule
	if deps != nil {
		if b, ok := deps.auth[site.ID]; ok {
			authMode = b.mode
			authRule = b.rule
		}
	}

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
		// 页面跳转登录：认证由 netpanel_session_auth 中间件完成
		return m.buildPageLoginRoutes(hostMatchers, mainHandlers, authRule), nil

	default:
		// 无认证
		route := map[string]interface{}{"handle": mainHandlers}
		if len(hostMatchers) > 0 {
			route["match"] = hostMatchers
		}
		return []interface{}{route}, nil
	}
}

// authBinding 站点绑定的认证规则。
type authBinding struct {
	mode string
	rule model.AccessRule
}

// aclBinding 站点绑定的 IP 访问控制（已解析为具体 IP/CIDR 列表）。
type aclBinding struct {
	mode string
	ips  []string
}

// routeDeps 站点路由构建所需的预加载依赖。
// 此前 buildRoutes 对每个站点各执行一次 findEnabledWafForSite / findAuthRule，
// 二者内部都是全表扫描再逐个 JSON 解析绑定关系，端口下 N 个站点就是 2N 次全表扫描。
// 现改为按端口一次性加载，构建 map[siteID] 供 O(1) 查询。
type routeDeps struct {
	waf  map[uint]uint        // siteID -> WAF 配置 ID
	auth map[uint]authBinding // siteID -> 认证规则
	acl  map[uint]aclBinding  // siteID -> IP 黑白名单
}

// loadRouteDeps 一次性加载启用 WAF、认证规则与 IP 访问控制的绑定关系。
func (m *Manager) loadRouteDeps(sites []model.CaddySite) *routeDeps {
	deps := &routeDeps{
		waf:  make(map[uint]uint),
		auth: make(map[uint]authBinding),
		acl:  make(map[uint]aclBinding),
	}

	var wafConfigs []model.WafConfig
	m.db.Where("enable = ?", true).Find(&wafConfigs)
	for _, cfg := range wafConfigs {
		for _, sid := range parseSiteIDs(cfg.BindSiteIDs) {
			if _, ok := deps.waf[sid]; !ok {
				deps.waf[sid] = cfg.ID
			}
		}
	}

	var rules []model.AccessRule
	m.db.Where("enable = ?", true).Find(&rules)
	for _, rule := range rules {
		siteIDs := parseSiteIDs(rule.BindSiteIDs)
		if len(siteIDs) == 0 {
			continue
		}
		// IP 黑白名单：绑定到站点的规则需要在 Caddy 侧判定
		if ips := access.ResolveIPs(m.db, rule); len(ips) > 0 {
			mode := rule.Mode
			if mode == "" {
				mode = "blacklist"
			}
			for _, sid := range siteIDs {
				if _, ok := deps.acl[sid]; !ok {
					deps.acl[sid] = aclBinding{mode: mode, ips: ips}
				}
			}
		}
		// 登录认证：仅 auth_mode 非空时生效
		if rule.AuthMode == "" {
			continue
		}
		for _, sid := range siteIDs {
			if _, ok := deps.auth[sid]; !ok {
				deps.auth[sid] = authBinding{mode: rule.AuthMode, rule: rule}
			}
		}
	}
	return deps
}

// parseSiteIDs 解析 JSON 数组形式的站点绑定 ID 列表。
func parseSiteIDs(raw string) []uint {
	if raw == "" {
		return nil
	}
	var ids []uint
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}

// hasHostMatcher 判断域名是否应生成 host/sni 匹配条件。
// localhost、IP 与空域名不生成 matcher，其路由是无条件匹配（兜底）。
func hasHostMatcher(domain string) bool {
	return domain != "" && !isLocalOrIP(domain)
}

// sortSitesForRouting 将有 host 匹配（真实域名）的站点排在前，无域名的排在后。
func sortSitesForRouting(sites []model.CaddySite) []model.CaddySite {
	out := make([]model.CaddySite, 0, len(sites))
	var wildcard []model.CaddySite
	for _, s := range sites {
		if hasHostMatcher(s.Domain) {
			out = append(out, s)
		} else {
			wildcard = append(wildcard, s)
		}
	}
	return append(out, wildcard...)
}

// countWildcardSites 统计无域名匹配（兜底路由）站点的数量。
func countWildcardSites(sites []model.CaddySite) int {
	n := 0
	for _, s := range sites {
		if !hasHostMatcher(s.Domain) {
			n++
		}
	}
	return n
}

// orderTLSPolicies 将带 SNI match 的策略排在前、无 match 的兜底策略排最后。
// Caddy 按顺序取第一个匹配的策略，无 match 的策略若靠前会成为兜底，
// 使后续具体的 SNI 策略失效。
func orderTLSPolicies(policies []interface{}) []interface{} {
	var matched, unmatched []interface{}
	for _, p := range policies {
		pm, ok := p.(map[string]interface{})
		if ok && pm["match"] != nil {
			matched = append(matched, p)
		} else {
			unmatched = append(unmatched, p)
		}
	}
	return append(matched, unmatched...)
}

// buildPageLoginRoutes 构建页面跳转登录的路由。
//
// 认证由 netpanel_session_auth 中间件完成：它会校验 Cookie 的 HMAC 签名、
// 有效期以及用户是否仍然启用，未通过时 302 到面板登录页。
// 不能仅靠 Caddy 的 header_regexp 匹配 Cookie 是否存在——那样任何人
// 手工设置 netpanel_session=x 就能绕过认证。
func (m *Manager) buildPageLoginRoutes(hostMatchers []interface{}, mainHandlers []interface{}, rule model.AccessRule) []interface{} {
	var allowedIDs []uint
	if rule.AllowedUserIDs != "" {
		json.Unmarshal([]byte(rule.AllowedUserIDs), &allowedIDs)
	}

	authHandler := map[string]interface{}{
		"handler":    "netpanel_session_auth",
		"panel_port": m.panelPort,
	}
	if len(allowedIDs) > 0 {
		authHandler["allowed_user_ids"] = allowedIDs
	}

	route := map[string]interface{}{
		"handle": append([]interface{}{authHandler}, mainHandlers...),
	}
	if len(hostMatchers) > 0 {
		route["match"] = hostMatchers
	}
	return []interface{}{route}
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

	if hasHostMatcher(site.Domain) {
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

	url := m.admin.baseURL + path
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

// validateUpstream 校验上游目标地址格式，防止非法值损坏 Caddy 配置。
// 允许 host:port（如 1.2.3.4:7000）或带协议前缀（http:// / https://）的形式；
// 复用 normalizeUpstreamDial 的规范化逻辑，能解析出合法 host:port 即通过。
func validateUpstream(upstream string) error {
	addr := normalizeUpstreamDial(upstream)
	if addr == "" {
		return fmt.Errorf("非法上游目标地址: %q", upstream)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return fmt.Errorf("非法上游目标地址: %q", upstream)
	}
	// 拒绝含空白/控制字符的地址（可能注入 Caddy 配置）
	for _, r := range addr {
		if r <= ' ' {
			return fmt.Errorf("非法上游目标地址（含空白字符）: %q", upstream)
		}
	}
	return nil
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
