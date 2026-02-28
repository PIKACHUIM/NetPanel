package frp

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fatedier/frp/assets"
	"github.com/fatedier/frp/client"
	v1 "github.com/fatedier/frp/pkg/config/v1"
	"github.com/fatedier/frp/pkg/config/types"
	"github.com/fatedier/frp/pkg/config/v1/validation"
	"github.com/fatedier/frp/server"
	frpsassets "github.com/netpanel/netpanel/assets/frps"
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
	// 注册 frps Dashboard 静态资源，使 frp WebServer 能正常提供前端页面
	assets.Register(frpsassets.StaticFiles)
	return &Manager{db: db, log: log}
}

// StartAll 启动所有已启用的 FRP 实例
func (m *Manager) StartAll() {
	var clients []model.FrpcConfig
	m.db.Where("enable = ?", true).Find(&clients)
	for _, c := range clients {
		if err := m.StartClient(c.ID); err != nil {
			m.log.Errorf("[FRP客户][%s] 启动失败: %v", c.Name, err)
		}
	}

	var servers []model.FrpsConfig
	m.db.Where("enable = ?", true).Find(&servers)
	for _, s := range servers {
		if err := m.StartServer(s.ID); err != nil {
			m.log.Errorf("[FRP服务][%s] 启动失败: %v", s.Name, err)
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
	if _, err := validation.ValidateAllClientConfig(frpCfg, proxyCfgs, nil, nil); err != nil {
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

	m.log.Infof("[FRP客户][%s] 已启动，连接 %s:%d，代理数: %d",
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
		m.log.Infof("[FRP客户][%s] 已停止", name)
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
			m.log.Warnf("[FRP客户][%s] 服务意外退出", name)
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
	if err := common.Complete(); err != nil {
		return nil, nil, fmt.Errorf("初始化 FRP 客户端默认配置失败: %w", err)
	}

	// ===== 基本连接 =====
	common.ServerAddr = cfg.ServerAddr
	common.ServerPort = cfg.ServerPort

	// 用户名（代理名前缀）
	if cfg.User != "" {
		common.User = cfg.User
	}

	// ===== 认证 =====
	authMethod := v1.AuthMethodToken
	if cfg.AuthMethod == "oidc" {
		authMethod = v1.AuthMethodOIDC
	}
	if cfg.Token != "" {
		common.Auth = v1.AuthClientConfig{
			Method: authMethod,
			Token:  cfg.Token,
		}
	} else if cfg.AuthMethod != "" {
		common.Auth.Method = authMethod
	}

	// ===== 传输层 =====
	// 传输协议（tcp/kcp/quic/websocket/wss）
	if cfg.TransportProtocol != "" {
		common.Transport.Protocol = cfg.TransportProtocol
	}

	// KCP 协议时覆盖连接端口
	if cfg.TransportProtocol == "kcp" && cfg.KCPPort > 0 {
		common.ServerPort = cfg.KCPPort
	}
	// QUIC 协议时覆盖连接端口
	if cfg.TransportProtocol == "quic" && cfg.QUICPort > 0 {
		common.ServerPort = cfg.QUICPort
	}

	// TCP 多路复用
	tcpMux := cfg.TCPMux
	common.Transport.TCPMux = &tcpMux
	if cfg.TCPMuxKeepaliveInterval > 0 {
		common.Transport.TCPMuxKeepaliveInterval = int64(cfg.TCPMuxKeepaliveInterval)
	}

	// 连接超时 & keepalive
	if cfg.DialServerTimeout > 0 {
		common.Transport.DialServerTimeout = int64(cfg.DialServerTimeout)
	}
	if cfg.DialServerKeepalive > 0 {
		common.Transport.DialServerKeepAlive = int64(cfg.DialServerKeepalive)
	}

	// 心跳
	if cfg.HeartbeatInterval != 0 {
		common.Transport.HeartbeatInterval = int64(cfg.HeartbeatInterval)
	}
	if cfg.HeartbeatTimeout > 0 {
		common.Transport.HeartbeatTimeout = int64(cfg.HeartbeatTimeout)
	}

	// 连接池
	if cfg.PoolCount > 0 {
		common.Transport.PoolCount = cfg.PoolCount
	}

	// 绑定本地 IP
	if cfg.ConnectServerLocalIP != "" {
		common.Transport.ConnectServerLocalIP = cfg.ConnectServerLocalIP
	}

	// 代理地址
	if cfg.ProxyURL != "" {
		common.Transport.ProxyURL = cfg.ProxyURL
	}

	// TLS
	if cfg.TLSEnable {
		common.Transport.TLS.Enable = &cfg.TLSEnable
	}

	// ===== 网络 =====
	// STUN 服务器（xtcp 打洞）
	if cfg.NatHoleStunServer != "" {
		common.NatHoleSTUNServer = cfg.NatHoleStunServer
	}
	// 自定义 DNS
	if cfg.DNSServer != "" {
		common.DNSServer = cfg.DNSServer
	}
	// 首次登录失败退出
	common.LoginFailExit = &cfg.LoginFailExit
	// UDP 包大小
	if cfg.UDPPacketSize > 0 {
		common.UDPPacketSize = int64(cfg.UDPPacketSize)
	}

	// ===== Web 管理 =====
	if cfg.WebServerPort > 0 {
		common.WebServer.Addr = "127.0.0.1"
		common.WebServer.Port = cfg.WebServerPort
		common.WebServer.User = cfg.WebServerUser
		common.WebServer.Password = cfg.WebServerPassword
	}

	// ===== 日志 =====
	if cfg.LogLevel != "" {
		common.Log.Level = cfg.LogLevel
	}
	common.Log.To = "console"

	// ===== 构建代理配置 =====
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

	// 带宽限制
	if p.BandwidthLimit != "" {
		bwLimit, err := types.NewBandwidthQuantity(p.BandwidthLimit)
		if err == nil {
			base.Transport.BandwidthLimit = bwLimit
		}
		if p.BandwidthLimitMode != "" {
			base.Transport.BandwidthLimitMode = p.BandwidthLimitMode
		}
	}

	// 健康检查
	if p.HealthCheckType != "" {
		base.HealthCheck = v1.HealthCheckConfig{
			Type:             p.HealthCheckType,
			TimeoutSeconds:   p.HealthCheckTimeoutS,
			MaxFailed:        p.HealthCheckMaxFailed,
			IntervalSeconds:  p.HealthCheckIntervalS,
			Path:             p.HealthCheckPath,
		}
	}

	// 负载均衡
	if p.LoadBalancerGroup != "" {
		base.LoadBalancer = v1.LoadBalancerConfig{
			Group:    p.LoadBalancerGroup,
			GroupKey: p.LoadBalancerGroupKey,
		}
	}

	// 解析 AllowUsers（逗号分隔）
	var allowUsers []string
	if p.AllowUsers != "" {
		for _, u := range strings.Split(p.AllowUsers, ",") {
			u = strings.TrimSpace(u)
			if u != "" {
				allowUsers = append(allowUsers, u)
			}
		}
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
		if p.Locations != "" {
			cfg.Locations = strings.Split(p.Locations, ",")
		}
		if p.HostHeaderRewrite != "" {
			cfg.HostHeaderRewrite = p.HostHeaderRewrite
		}
		if p.HTTPUser != "" {
			cfg.HTTPUser = p.HTTPUser
			cfg.HTTPPassword = p.HTTPPassword
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
			Secretkey:       p.SecretKey,
			AllowUsers:      allowUsers,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		return cfg, nil

	case "xtcp":
		cfg := &v1.XTCPProxyConfig{
			ProxyBaseConfig: base,
			Secretkey:       p.SecretKey,
			AllowUsers:      allowUsers,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		return cfg, nil

	case "sudp":
		cfg := &v1.SUDPProxyConfig{
			ProxyBaseConfig: base,
			Secretkey:       p.SecretKey,
			AllowUsers:      allowUsers,
		}
		cfg.LocalIP = p.LocalIP
		cfg.LocalPort = p.LocalPort
		return cfg, nil

	case "tcpmux":
		multiplexer := p.Multiplexer
		if multiplexer == "" {
			multiplexer = "httpconnect"
		}
		cfg := &v1.TCPMuxProxyConfig{
			ProxyBaseConfig: base,
			Multiplexer:     multiplexer,
			HTTPUser:        p.HTTPUser,
			HTTPPassword:    p.HTTPPassword,
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
	if _, err := validation.NewConfigValidator(nil).ValidateServerConfig(frpCfg); err != nil {
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

	m.log.Infof("[FRP服务][%s] 已启动，监听 %s:%d", cfg.Name, cfg.BindAddr, cfg.BindPort)
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
		m.log.Infof("[FRP服务][%s] 已停止", name)
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
			m.log.Warnf("[FRP服务][%s] 服务意外退出", name)
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
	if err := frpCfg.Complete(); err != nil {
		return nil, fmt.Errorf("初始化 FRP 服务端默认配置失败: %w", err)
	}

	// ===== 监听地址 =====
	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	frpCfg.BindAddr = bindAddr
	frpCfg.BindPort = cfg.BindPort

	// 代理监听地址
	if cfg.ProxyBindAddr != "" {
		frpCfg.ProxyBindAddr = cfg.ProxyBindAddr
	}

	// KCP 监听端口（UDP）
	if cfg.KCPBindPort > 0 {
		frpCfg.KCPBindPort = cfg.KCPBindPort
	}

	// QUIC 监听端口（UDP）
	if cfg.QUICBindPort > 0 {
		frpCfg.QUICBindPort = cfg.QUICBindPort
	}

	// ===== 虚拟主机 =====
	// HTTP 虚拟主机端口
	if cfg.VhostHTTPPort > 0 {
		frpCfg.VhostHTTPPort = cfg.VhostHTTPPort
	}
	// HTTP 响应超时
	if cfg.VhostHTTPTimeout > 0 {
		frpCfg.VhostHTTPTimeout = int64(cfg.VhostHTTPTimeout)
	}
	// HTTPS 虚拟主机端口
	if cfg.VhostHTTPSPort > 0 {
		frpCfg.VhostHTTPSPort = cfg.VhostHTTPSPort
	}
	// tcpmux httpconnect 端口
	if cfg.TcpmuxHTTPConnectPort > 0 {
		frpCfg.TCPMuxHTTPConnectPort = cfg.TcpmuxHTTPConnectPort
	}
	if cfg.TcpmuxPassthrough {
		frpCfg.TCPMuxPassthrough = cfg.TcpmuxPassthrough
	}

	// ===== 子域名 =====
	if cfg.SubDomainHost != "" {
		frpCfg.SubDomainHost = cfg.SubDomainHost
	}

	// 自定义 404 页面
	if cfg.Custom404Page != "" {
		frpCfg.Custom404Page = cfg.Custom404Page
	}

	// ===== 认证 =====
	if cfg.Token != "" {
		frpCfg.Auth = v1.AuthServerConfig{
			Method: v1.AuthMethodToken,
			Token:  cfg.Token,
		}
	}

	// ===== Dashboard（WebServer）=====
	if cfg.DashboardPort > 0 {
		frpCfg.WebServer.Addr = cfg.DashboardAddr
		if frpCfg.WebServer.Addr == "" {
			frpCfg.WebServer.Addr = "0.0.0.0"
		}
		frpCfg.WebServer.Port = cfg.DashboardPort
		frpCfg.WebServer.User = cfg.DashboardUser
		frpCfg.WebServer.Password = cfg.DashboardPassword
		// AssetsDir 为空时，frp 使用 assets.FileSystem（已通过 Register 注册），
		// 可正常提供 Dashboard 静态页面
	}
	// Prometheus 监控（需同时启用 Dashboard）
	if cfg.EnablePrometheus && cfg.DashboardPort > 0 {
		frpCfg.EnablePrometheus = true
	}

	// ===== 限制 =====
	if cfg.MaxPortsPerClient > 0 {
		frpCfg.MaxPortsPerClient = int64(cfg.MaxPortsPerClient)
	}
	if cfg.UserConnTimeout > 0 {
		frpCfg.UserConnTimeout = int64(cfg.UserConnTimeout)
	}
	if cfg.UDPPacketSize > 0 {
		frpCfg.UDPPacketSize = int64(cfg.UDPPacketSize)
	}
	if cfg.NatholeAnalysisDataReserveHours > 0 {
		frpCfg.NatHoleAnalysisDataReserveHours = int64(cfg.NatholeAnalysisDataReserveHours)
	}
	// 详细错误信息（默认 true，只有显式设为 false 时才关闭）
	frpCfg.DetailedErrorsToClient = &cfg.DetailedErrorsToClient

	// ===== 日志 =====
	if cfg.LogLevel != "" {
		frpCfg.Log.Level = cfg.LogLevel
	}
	if cfg.LogFile != "" {
		frpCfg.Log.To = cfg.LogFile
	} else {
		frpCfg.Log.To = "console"
	}
	if cfg.LogMaxDays > 0 {
		frpCfg.Log.MaxDays = int64(cfg.LogMaxDays)
	}

	// ===== 传输层 =====
	if cfg.TransportMaxPoolCount > 0 {
		frpCfg.Transport.MaxPoolCount = int64(cfg.TransportMaxPoolCount)
	}
	if cfg.TransportHeartbeatTimeout != 0 {
		frpCfg.Transport.HeartbeatTimeout = int64(cfg.TransportHeartbeatTimeout)
	}
	if cfg.TransportTCPMuxKeepalive > 0 {
		frpCfg.Transport.TCPMuxKeepaliveInterval = int64(cfg.TransportTCPMuxKeepalive)
	}
	if cfg.TransportTCPKeepalive != 0 {
		frpCfg.Transport.TCPKeepAlive = int64(cfg.TransportTCPKeepalive)
	}
	// TLS 配置
	if cfg.TransportTLSForce {
		frpCfg.Transport.TLS.Force = cfg.TransportTLSForce
	}
	if cfg.TransportTLSCertFile != "" {
		frpCfg.Transport.TLS.CertFile = cfg.TransportTLSCertFile
	}
	if cfg.TransportTLSKeyFile != "" {
		frpCfg.Transport.TLS.KeyFile = cfg.TransportTLSKeyFile
	}
	if cfg.TransportTLSTrustedCAFile != "" {
		frpCfg.Transport.TLS.TrustedCaFile = cfg.TransportTLSTrustedCAFile
	}

	// ===== SSH 隧道网关 =====
	if cfg.SSHTunnelGatewayBindPort > 0 {
		frpCfg.SSHTunnelGateway.BindPort = cfg.SSHTunnelGatewayBindPort
		if cfg.SSHTunnelGatewayPrivateKeyFile != "" {
			frpCfg.SSHTunnelGateway.PrivateKeyFile = cfg.SSHTunnelGatewayPrivateKeyFile
		}
		if cfg.SSHTunnelGatewayAutoGenKeyPath != "" {
			frpCfg.SSHTunnelGateway.AutoGenPrivateKeyPath = cfg.SSHTunnelGatewayAutoGenKeyPath
		}
		if cfg.SSHTunnelGatewayAuthorizedKeys != "" {
			frpCfg.SSHTunnelGateway.AuthorizedKeysFile = cfg.SSHTunnelGatewayAuthorizedKeys
		}
	}

	return frpCfg, nil
}
