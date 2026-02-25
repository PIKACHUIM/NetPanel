package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/caddyserver/caddy/v2"
	_ "github.com/caddyserver/caddy/v2/modules/caddyhttp"
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
	mu        sync.Mutex
	started   bool
	adminHTTP *http.Client
}

func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{
		db:      db,
		log:     log,
		dataDir: dataDir,
		adminHTTP: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
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
	route, err := m.buildRoute(&site)
	if err != nil {
		m.setError(id, err.Error())
		return fmt.Errorf("构建路由配置失败: %w", err)
	}

	// 通过 Admin API 添加路由
	serverKey := fmt.Sprintf("netpanel_%d", id)
	serverCfg := m.buildServerConfig(&site, route)

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
func (m *Manager) buildServerConfig(site *model.CaddySite, route map[string]interface{}) map[string]interface{} {
	listenAddr := fmt.Sprintf(":%d", site.Port)

	serverCfg := map[string]interface{}{
		"listen": []string{listenAddr},
		"routes": []interface{}{route},
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

// buildRoute 构建路由配置
func (m *Manager) buildRoute(site *model.CaddySite) (map[string]interface{}, error) {
	// 匹配条件
	var matchers []interface{}
	if site.Domain != "" {
		matchers = append(matchers, map[string]interface{}{
			"host": []string{site.Domain},
		})
	}

	// 处理器
	var handlers []interface{}

	switch site.SiteType {
	case "reverse_proxy":
		if site.UpstreamAddr == "" {
			return nil, fmt.Errorf("反向代理目标地址不能为空")
		}
		handlers = append(handlers, map[string]interface{}{
			"handler": "reverse_proxy",
			"upstreams": []interface{}{
				map[string]interface{}{"dial": site.UpstreamAddr},
			},
			"headers": map[string]interface{}{
				"request": map[string]interface{}{
					"set": map[string]interface{}{
						"X-Real-IP":       []string{"{http.request.remote.host}"},
						"X-Forwarded-For": []string{"{http.request.remote.host}"},
					},
				},
			},
		})

	case "static":
		if site.RootPath == "" {
			return nil, fmt.Errorf("静态文件根目录不能为空")
		}
		// 确保目录存在
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
		handlers = append(handlers, fileHandler)

	case "redirect":
		if site.RedirectTo == "" {
			return nil, fmt.Errorf("重定向目标地址不能为空")
		}
		code := site.RedirectCode
		if code == 0 {
			code = 301
		}
		handlers = append(handlers, map[string]interface{}{
			"handler":   "static_response",
			"status_code": code,
			"headers": map[string]interface{}{
				"Location": []string{site.RedirectTo},
			},
		})

	default:
		return nil, fmt.Errorf("不支持的站点类型: %s", site.SiteType)
	}

	route := map[string]interface{}{
		"handle": handlers,
	}
	if len(matchers) > 0 {
		route["match"] = matchers
	}

	return route, nil
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