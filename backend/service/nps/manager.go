package nps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	npsClient "github.com/djylb/nps/client"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// clientEntry 客户端运行实例
type clientEntry struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// serverEntry 服务端运行实例（通过子进程方式，因为beego是全局单例）
type serverEntry struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// Manager NPS 管理器
// 服务端：每个实例生成独立配置文件，通过 beego 加载后在独立 goroutine 中运行
// 客户端：直接调用 nps client 包的 NewRPClient，支持多实例并发
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	dataDir string
	servers sync.Map // map[uint]*serverEntry
	clients sync.Map // map[uint]*clientEntry
}

// NewManager 创建 NPS 管理器
func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{db: db, log: log, dataDir: dataDir}
}

// StartAll 启动所有已启用的 NPS 实例
func (m *Manager) StartAll() {
	var servers []model.NpsServerConfig
	m.db.Where("enable = ?", true).Find(&servers)
	for _, s := range servers {
		if err := m.StartServer(s.ID); err != nil {
			m.log.Errorf("[NPS服务端][%s] 启动失败: %v", s.Name, err)
		}
	}

	var clients []model.NpsClientConfig
	m.db.Where("enable = ?", true).Find(&clients)
	for _, c := range clients {
		if err := m.StartClient(c.ID); err != nil {
			m.log.Errorf("[NPS客户端][%s] 启动失败: %v", c.Name, err)
		}
	}
}

// StopAll 停止所有 NPS 实例
func (m *Manager) StopAll() {
	m.servers.Range(func(key, value interface{}) bool {
		entry := value.(*serverEntry)
		entry.cancel()
		select {
		case <-entry.done:
		case <-time.After(5 * time.Second):
		}
		return true
	})
	m.clients.Range(func(key, value interface{}) bool {
		entry := value.(*clientEntry)
		entry.cancel()
		select {
		case <-entry.done:
		case <-time.After(5 * time.Second):
		}
		return true
	})
}

// ===== 服务端 =====

// StartServer 启动指定 NPS 服务端
// NPS 服务端依赖 beego 全局配置，每个实例生成独立配置文件并在独立 goroutine 中运行
func (m *Manager) StartServer(id uint) error {
	m.StopServer(id)

	var cfg model.NpsServerConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("NPS 服务端配置不存在: %w", err)
	}
	if !cfg.Enable {
		return fmt.Errorf("NPS 服务端 [%s] 未启用", cfg.Name)
	}

	// 生成配置文件
	confPath, err := m.writeServerConfig(&cfg)
	if err != nil {
		m.setServerError(id, err.Error())
		return fmt.Errorf("生成 NPS 服务端配置失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	entry := &serverEntry{cancel: cancel, done: done}
	m.servers.Store(id, entry)

	go func() {
		defer func() {
			close(done)
			m.servers.Delete(id)
			m.db.Model(&model.NpsServerConfig{}).Where("id = ?", id).Update("status", "stopped")
			m.log.Infof("[NPS服务端][%d] 已停止", id)
		}()

		if err := runNpsServer(ctx, confPath); err != nil {
			m.log.Errorf("[NPS服务端][%s] 运行错误: %v", cfg.Name, err)
			m.setServerError(id, err.Error())
			return
		}
	}()

	m.db.Model(&model.NpsServerConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[NPS服务端][%s] 已启动，桥接端口: %d，Web端口: %d",
		cfg.Name, cfg.BridgePort, cfg.WebPort)
	return nil
}

// StopServer 停止指定 NPS 服务端
func (m *Manager) StopServer(id uint) {
	if val, ok := m.servers.Load(id); ok {
		entry := val.(*serverEntry)
		entry.cancel()
		select {
		case <-entry.done:
		case <-time.After(5 * time.Second):
		}
		m.servers.Delete(id)
	}
	m.db.Model(&model.NpsServerConfig{}).Where("id = ?", id).Update("status", "stopped")
}

// GetServerStatus 获取服务端运行状态
func (m *Manager) GetServerStatus(id uint) string {
	if _, ok := m.servers.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// setServerError 设置服务端错误状态
func (m *Manager) setServerError(id uint, errMsg string) {
	m.db.Model(&model.NpsServerConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": errMsg,
	})
}

// writeServerConfig 生成 nps.conf 配置文件，返回配置目录路径
func (m *Manager) writeServerConfig(cfg *model.NpsServerConfig) (string, error) {
	confDir := filepath.Join(m.dataDir, "nps", fmt.Sprintf("server_%d", cfg.ID))
	if err := os.MkdirAll(confDir, 0755); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}

	bindAddr := cfg.BindAddr
	if bindAddr == "" {
		bindAddr = "0.0.0.0"
	}
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	webUsername := cfg.WebUsername
	if webUsername == "" {
		webUsername = "admin"
	}
	webPassword := cfg.WebPassword
	if webPassword == "" {
		webPassword = "123456"
	}

	content := fmt.Sprintf(`appname = nps
httpport = %d
runmode = prod

bridge_port=%d
bridge_type=tcp
public_vkey=%s
log_level=%s
log=off

http_proxy_ip=%s
http_proxy_port=%d
https_proxy_port=%d

web_host=%s
web_username=%s
web_password=%s
web_port=%d
web_base_url=
web_open_ssl=false

disconnect_timeout=60
`,
		cfg.WebPort,
		cfg.BridgePort,
		cfg.AuthKey,
		logLevel,
		bindAddr,
		cfg.HTTPPort,
		cfg.HTTPSPort,
		bindAddr,
		webUsername,
		webPassword,
		cfg.WebPort,
	)

	confPath := filepath.Join(confDir, "nps.conf")
	if err := os.WriteFile(confPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入配置文件失败: %w", err)
	}
	return confDir, nil
}

// ===== 客户端 =====

// StartClient 启动指定 NPS 客户端（直接调用 nps client 包）
func (m *Manager) StartClient(id uint) error {
	m.StopClient(id)

	var cfg model.NpsClientConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("NPS 客户端配置不存在: %w", err)
	}
	if !cfg.Enable {
		return fmt.Errorf("NPS 客户端 [%s] 未启用", cfg.Name)
	}

	serverAddr := cfg.ServerAddr
	if serverAddr == "" {
		return fmt.Errorf("NPS 客户端 [%s] 服务器地址不能为空", cfg.Name)
	}
	serverPort := cfg.ServerPort
	if serverPort == 0 {
		serverPort = 8024
	}
	vkey := cfg.VkeyOrID
	if vkey == "" {
		vkey = cfg.AuthKey
	}
	if vkey == "" {
		return fmt.Errorf("NPS 客户端 [%s] vkey 或 auth_key 不能为空", cfg.Name)
	}

	connType := cfg.ConnType
	if connType == "" {
		connType = "tcp"
	}

	fullAddr := fmt.Sprintf("%s:%d", serverAddr, serverPort)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	entry := &clientEntry{cancel: cancel, done: done}
	m.clients.Store(id, entry)

	go func() {
		defer func() {
			close(done)
			m.clients.Delete(id)
			m.db.Model(&model.NpsClientConfig{}).Where("id = ?", id).Update("status", "stopped")
			m.log.Infof("[NPS客户端][%d] 已停止", id)
		}()

		// 设置客户端全局参数
		npsClient.AutoReconnect = true

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			m.log.Infof("[NPS客户端][%s] 连接服务器 %s vkey: %s type: %s",
				cfg.Name, fullAddr, vkey, connType)

			rpClient := npsClient.NewRPClient(fullAddr, vkey, connType, "", "", "", nil, 60, nil)
			rpClient.Start(ctx)

			select {
			case <-ctx.Done():
				return
			default:
				m.log.Infof("[NPS客户端][%s] 连接断开，5秒后重连...", cfg.Name)
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
			}
		}
	}()

	m.db.Model(&model.NpsClientConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[NPS客户端][%s] 已启动，连接 %s", cfg.Name, fullAddr)
	return nil
}

// StopClient 停止指定 NPS 客户端
func (m *Manager) StopClient(id uint) {
	if val, ok := m.clients.Load(id); ok {
		entry := val.(*clientEntry)
		entry.cancel()
		select {
		case <-entry.done:
		case <-time.After(5 * time.Second):
		}
		m.clients.Delete(id)
	}
	m.db.Model(&model.NpsClientConfig{}).Where("id = ?", id).Update("status", "stopped")
}

// GetClientStatus 获取客户端运行状态
func (m *Manager) GetClientStatus(id uint) string {
	if _, ok := m.clients.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// setClientError 设置客户端错误状态
func (m *Manager) setClientError(id uint, errMsg string) {
	m.db.Model(&model.NpsClientConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": errMsg,
	})
}