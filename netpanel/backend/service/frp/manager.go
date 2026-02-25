package frp

import (
	"context"
	"fmt"
	"sync"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// clientEntry FRP 客户端运行实例
type clientEntry struct {
	cancel context.CancelFunc
}

// serverEntry FRP 服务端运行实例
type serverEntry struct {
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

func (m *Manager) StartAll() {
	// 启动所有已启用的客户端
	var clients []model.FrpcConfig
	m.db.Where("enable = ?", true).Find(&clients)
	for _, c := range clients {
		if err := m.StartClient(c.ID); err != nil {
			m.log.Errorf("FRP 客户端 [%s] 启动失败: %v", c.Name, err)
		}
	}

	// 启动所有已启用的服务端
	var servers []model.FrpsConfig
	m.db.Where("enable = ?", true).Find(&servers)
	for _, s := range servers {
		if err := m.StartServer(s.ID); err != nil {
			m.log.Errorf("FRP 服务端 [%s] 启动失败: %v", s.Name, err)
		}
	}
}

func (m *Manager) StopAll() {
	m.clients.Range(func(key, value interface{}) bool {
		entry := value.(*clientEntry)
		entry.cancel()
		return true
	})
	m.servers.Range(func(key, value interface{}) bool {
		entry := value.(*serverEntry)
		entry.cancel()
		return true
	})
}

// ===== 客户端 =====

func (m *Manager) StartClient(id uint) error {
	m.StopClient(id)

	var cfg model.FrpcConfig
	if err := m.db.Preload("Proxies").First(&cfg, id).Error; err != nil {
		return fmt.Errorf("FRP 客户端配置不存在: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &clientEntry{cancel: cancel}
	m.clients.Store(id, entry)

	go m.runClient(ctx, id, &cfg)

	m.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[FRP客户端][%d] 已启动，连接 %s:%d", id, cfg.ServerAddr, cfg.ServerPort)
	return nil
}

func (m *Manager) StopClient(id uint) {
	if val, ok := m.clients.Load(id); ok {
		entry := val.(*clientEntry)
		entry.cancel()
		m.clients.Delete(id)
	}
	m.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) RestartClient(id uint) error {
	m.StopClient(id)
	return m.StartClient(id)
}

func (m *Manager) GetClientStatus(id uint) string {
	if _, ok := m.clients.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// runClient 运行 FRP 客户端
// 注意：实际集成需要引用 frp 库，这里提供框架
func (m *Manager) runClient(ctx context.Context, id uint, cfg *model.FrpcConfig) {
	defer func() {
		m.clients.Delete(id)
		m.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("status", "stopped")
	}()

	// TODO: 集成 frp client 库
	// 示例：使用 frp 的 client.NewService() 创建服务
	// frpCfg := v1.ClientCommonConfig{...}
	// svc, err := client.NewService(frpCfg)
	// if err != nil { ... }
	// go svc.Run(ctx)
	// <-ctx.Done()
	// svc.Close()

	m.log.Infof("[FRP客户端][%d] 运行中（待集成 frp 库）", id)
	<-ctx.Done()
	m.log.Infof("[FRP客户端][%d] 已停止", id)
}

// ===== 服务端 =====

func (m *Manager) StartServer(id uint) error {
	m.StopServer(id)

	var cfg model.FrpsConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("FRP 服务端配置不存在: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &serverEntry{cancel: cancel}
	m.servers.Store(id, entry)

	go m.runServer(ctx, id, &cfg)

	m.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[FRP服务端][%d] 已启动，监听端口 %d", id, cfg.BindPort)
	return nil
}

func (m *Manager) StopServer(id uint) {
	if val, ok := m.servers.Load(id); ok {
		entry := val.(*serverEntry)
		entry.cancel()
		m.servers.Delete(id)
	}
	m.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetServerStatus(id uint) string {
	if _, ok := m.servers.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// runServer 运行 FRP 服务端
func (m *Manager) runServer(ctx context.Context, id uint, cfg *model.FrpsConfig) {
	defer func() {
		m.servers.Delete(id)
		m.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("status", "stopped")
	}()

	// TODO: 集成 frp server 库
	// frpCfg := v1.ServerConfig{...}
	// svc, err := server.NewService(frpCfg)
	// if err != nil { ... }
	// go svc.Run(ctx)
	// <-ctx.Done()
	// svc.Close()

	m.log.Infof("[FRP服务端][%d] 运行中（待集成 frp 库）", id)
	<-ctx.Done()
	m.log.Infof("[FRP服务端][%d] 已停止", id)
}
