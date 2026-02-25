package frp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/client"
	"github.com/fatedier/frp/server"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// clientEntry FRP 客户端运行实例
type clientEntry struct {
	svc    *client.Service
	cancel context.CancelFunc
}

// serverEntry FRP 服务端运行实例
type serverEntry struct {
	svc    *server.Service
	cancel context.CancelFunc
}

// Manager FRP 管理器（客户端+服务端）
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	clients sync.Map // map[uint]*clientEntry
	servers sync.Map // map[uint]*serverEntry
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// StartAll 启动所有已启用的 FRP 实例
func (m *Manager) StartAll() {
	var clients []model.FrpcConfig
	m.db.Where("enable = ?", true).Find(&clients)
	for _, c := range clients {
		if err := m.StartClient(c.ID); err != nil {
			m.log.Errorf("[FRP客户端][%s] 启动失败: %v", c.Name, err)
		}
	}

	var servers []model.FrpsConfig
	m.db.Where("enable = ?", true).Find(&servers)
	for _, s := range servers {
		if err := m.StartServer(s.ID); err != nil {
			m.log.Errorf("[FRP服务端][%s] 启动失败: %v", s.Name, err)
		}
	}
}

// StopAll 停止所有 FRP 实例
func (m *Manager) StopAll() {
	m.clients.Range(func(key, value any) bool {
		entry := value.(*clientEntry)
		entry.cancel()
		return true
	})
	m.servers.Range(func(key, value any) bool {
		entry := value.(*serverEntry)
		entry.cancel()
		return true
	})
}

// ===== 客户端 =====

// StartClient 启动指定 FRP 客户端
func (m *Manager) StartClient(id uint) error {
	m.StopClient(id)

	var cfg model.FrpcConfig
	if err := m.db.Preload("Proxies").First(&cfg, id).Error; err != nil {
		return fmt.Errorf("FRP 客户端配置不存在: %w", err)
	}
	if !cfg.Enable {
		return fmt.Errorf("FRP 客户端 [%s] 未启用", cfg.Name)
	}

	// 构建 frp 客户端配置
	frpCfg, proxyCfgs, err := buildClientConfig(&cfg)
	if err != nil {
		m.setClientError(id, err.Error())
		return fmt.Errorf("构建 FRP 客户端配置失败: %w", err)
	}

	// 验证配置
	if _, err := validation.ValidateAllClientConfig(frpCfg, proxyCfgs, nil); err != nil {
		m.setClientError(id, err.Error())
		return fmt.Errorf("FRP 客户端配置验证失败: %w", err)
	}

	// 创建服务
	svc, err := client.NewService(client.ServiceOptions{
		Common:         frpCfg,
		ProxyCfgs:      proxyCfgs,
		VisitorCfgs:    nil,
		ConfigFilePath: "",
	})
	if err != nil {
		m.setClientError(id, err.Error())
		return fmt.Errorf("创建 FRP 客户端服务失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &clientEntry{svc: svc, cancel: cancel}
	m.clients.Store(id, entry)

	// 更新状态
	m.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Updates(map[string]any{
		"status":     "running",
		"last_error": "",
	})

	go m.runClient(ctx, id, cfg.Name, svc)

	m.log.Infof("[FRP客户端][%s] 已启动，连接 %s:%d，代理数: %d",
		cfg.Name, cfg.ServerAddr, cfg.ServerPort, len(proxyCfgs))
	return nil
}

// StopClient 停止指定 FRP 客户端
func (m *Manager) StopClient(id uint) {
	if val, ok := m.clients.Load(id); ok {
		entry := val.(*clientEntry)
		entry.cancel()
		m.clients.Delete(id)
	}
	m.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("status", "stopped")
}

// RestartClient 重启指定 FRP 客户端
func (m *Manager) RestartClient(id uint) error {
	m.StopClient(id)
	time.Sleep(500 * time.Millisecond)
	return m.StartClient(id)
}

// GetClientStatus 获取客户端状态
func (m *Manager) GetClientStatus(id uint) string {
	if _, ok := m.clients.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// runClient 在 goroutine 中运行 FRP 客户端
func (m *Manager) runClient(ctx context.Context, id uint, name string, svc *client.Service) {
	defer func() {
		m.clients.Delete(id)
		m.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("status", "stopped")
		m.log.Infof("[FRP客户端][%s] 已停止", name)
	}()

	doneCh := make(chan struct{}, 1)
	go func() {
		svc.Run(ctx)
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		svc.Close()
		<-doneCh
	case <-doneCh:
		if ctx.Err() == nil {
			m.log.Warnf("[FRP客户端][%s] 服务意外退出", name)
		}
	}
}

// setClientError 设置客户端错误状态
func (m *Manager) setClientError(id uint, errMsg string) {
	m.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Updates(map[string]any{
		"status":     "error",
		"last_error": errMsg,
	})
}

// buildClientConfig 将数据库配置转换为 frp v1 配置
func buildClientConfig(cfg *model.FrpcConfig) (*v1.ClientCommonConfig, []v1.ProxyConfigurer, error) {
	common := &v1.ClientCommonConfig{}
	common.Complete()

	common.ServerAddr = cfg.ServerAddr
	common.ServerPort = cfg.ServerPort

	if cfg.Token != "" {
		common.Auth = v1.AuthClientConfig{
			Method: v1.AuthMethodToken,
			Token:  cfg.Token,
		}
	}

	if cfg.TLSEnable {
		common.Transport.TLS.Enable = &cfg.TLSEnable
	}

	if cfg.LogLevel != "" {
		common.Log.Level = cfg.LogLevel
	}
	common.Log.To = "console"

	// 构建代理配置
	var proxyCfgs []v1.ProxyConfigurer
	for _, p := range cfg.Proxies {
		if !p.Enable {
			continue
		}
		pc, err := buildProxyConfig(&p)
		if err != nil {
			return nil, nil, fmt.Errorf("代理 [%s] 配置错误: %w", p.Name, err)
		}
		proxyCfgs = append(proxyCfgs, pc)
	}

	return common, proxyCfgs, nil
}

// buildProxyConfig 构建单个代理配置
func buildProxyConfig(p *model.FrpcProxy) (v1.ProxyConfigurer, error) {
	base := v1.ProxyBaseConfig{
		Name: p.Name,
		Transport: v1.ProxyTransport{
			UseEncryption:  p.UseEncryption,
			UseCompression: p.UseCompression,
		},
	}

	switch strings.ToLower(p.Type) {
	case "tcp":
		cfg := &v1.TCPProxyConfig{
			ProxyBaseConfig: base,
			RemotePort:      p.RemotePort,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		return cfg, nil

	case "udp":
		cfg := &v1.UDPProxyConfig{
			ProxyBaseConfig: base,
			RemotePort:      p.RemotePort,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		return cfg, nil

	case "http":
		cfg := &v1.HTTPProxyConfig{
			ProxyBaseConfig: base,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		if p.CustomDomains != "" {
			cfg.CustomDomains = strings.Split(p.CustomDomains, ",")
		}
		if p.Subdomain != "" {
			cfg.SubDomain = p.Subdomain
		}
		return cfg, nil

	case "https":
		cfg := &v1.HTTPSProxyConfig{
			ProxyBaseConfig: base,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		if p.CustomDomains != "" {
			cfg.CustomDomains = strings.Split(p.CustomDomains, ",")
		}
		if p.Subdomain != "" {
			cfg.SubDomain = p.Subdomain
		}
		return cfg, nil

	case "stcp":
		cfg := &v1.STCPProxyConfig{
			ProxyBaseConfig: base,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		return cfg, nil

	case "xtcp":
		cfg := &v1.XTCPProxyConfig{
			ProxyBaseConfig: base,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		return cfg, nil

	default:
		return nil, fmt.Errorf("不支持的代理类型: %s", p.Type)
	}
}

// ===== 服务端 =====

// StartServer 启动指定 FRP 服务端
func (m *Manager) StartServer(id uint) error {
	m.StopServer(id)

	var cfg model.FrpsConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("FRP 服务端配置不存在: %w", err)
	}
	if !cfg.Enable {
		return fmt.Errorf("FRP 服务端 [%s] 未启用", cfg.Name)
	}

	// 构建 frp 服务端配置
	frpCfg, err := buildServerConfig(&cfg)
	if err != nil {
		m.setServerError(id, err.Error())
		return fmt.Errorf("构建 FRP 服务端配置失败: %w", err)
	}

	// 验证配置
	if _, err := validation.ValidateServerConfig(frpCfg); err != nil {
		m.setServerError(id, err.Error())
		return fmt.Errorf("FRP 服务端配置验证失败: %w", err)
	}

	// 创建服务
	svc, err := server.NewService(frpCfg)
	if err != nil {
		m.setServerError(id, err.Error())
		return fmt.Errorf("创建 FRP 服务端服务失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &serverEntry{svc: svc, cancel: cancel}
	m.servers.Store(id, entry)

	// 更新状态
	m.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Updates(map[string]any{
		"status":     "running",
		"last_error": "",
	})

	go m.runServer(ctx, id, cfg.Name, svc)

	m.log.Infof("[FRP服务端][%s] 已启动，监听 %s:%d", cfg.Name, cfg.BindAddr, cfg.BindPort)
	return nil
}

// StopServer 停止指定 FRP 服务端
func (m *Manager) StopServer(id uint) {
	if val, ok := m.servers.Load(id); ok {
		entry := val.(*serverEntry)
		entry.cancel()
		m.servers.Delete(id)
	}
	m.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("status", "stopped")
}

// GetServerStatus 获取服务端状态
func (m *Manager) GetServerStatus(id uint) string {
	if _, ok := m.servers.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// runServer 在 goroutine 中运行 FRP 服务端
func (m *Manager) runServer(ctx context.Context, id uint, name string, svc *server.Service) {
	defer func() {
		m.servers.Delete(id)
		m.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("status", "stopped")
		m.log.Infof("[FRP服务端][%s] 已停止", name)
	}()

	doneCh := make(chan struct{}, 1)
	go func() {
		svc.Run(ctx)
		close(doneCh)
	}()

	select {
	case <-ctx.Done():
		svc.Close()
		<-doneCh
	case <-doneCh:
		if ctx.Err() == nil {
			m.log.Warnf("[FRP服务端][%s] 服务意外退出", name)
		}
	}
}

// setServerError 设置服务端错误状态
func (m *Manager) setServerError(id uint, errMsg string) {
	m.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Updates(map[string]any{
		"status":     "error",
		"last_error": errMsg,
	})
}

// buildServerConfig 将数据库配置转换为 frp v1 服务端配置
func buildServerConfig(cfg *model.FrpsConfig) (*v1.ServerConfig, error) {
	frpCfg := &v1.ServerConfig{}
	frpCfg.Complete()

	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	frpCfg.BindAddr = bindAddr
	frpCfg.BindPort = cfg.BindPort

	if cfg.Token != "" {
		frpCfg.Auth = v1.AuthServerConfig{
			Method: v1.AuthMethodToken,
			Token:  cfg.Token,
		}
	}

	if cfg.DashboardPort > 0 {
		frpCfg.WebServer.Addr = cfg.DashboardAddr
		if frpCfg.WebServer.Addr == "" {
			frpCfg.WebServer.Addr = "0.0.0.0"
		}
		frpCfg.WebServer.Port = cfg.DashboardPort
		frpCfg.WebServer.User = cfg.DashboardUser
		frpCfg.WebServer.Password = cfg.DashboardPassword
	}

	if cfg.MaxPortsPerClient > 0 {
		frpCfg.MaxPortsPerClient = int64(cfg.MaxPortsPerClient)
	}

	if cfg.LogLevel != "" {
		frpCfg.Log.Level = cfg.LogLevel
	}
	frpCfg.Log.To = "console"

	return frpCfg, nil
}