package storage

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/webdav"
	"gorm.io/gorm"
)

type storageEntry struct {
	listener net.Listener
	server   *http.Server
}

// Manager 网络存储管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	entries sync.Map // map[uint]*storageEntry
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

func (m *Manager) StartAll() {
	var configs []model.StorageConfig
	m.db.Where("enable = ?", true).Find(&configs)
	for _, c := range configs {
		if err := m.Start(c.ID); err != nil {
			m.log.Errorf("网络存储 [%s] 启动失败: %v", c.Name, err)
		}
	}
}

func (m *Manager) StopAll() {
	m.entries.Range(func(key, value interface{}) bool {
		m.Stop(key.(uint))
		return true
	})
}

func (m *Manager) Start(id uint) error {
	m.Stop(id)

	var cfg model.StorageConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("存储配置不存在: %w", err)
	}

	switch cfg.Protocol {
	case "webdav":
		return m.startWebDAV(id, &cfg)
	case "sftp":
		return m.startSFTP(id, &cfg)
	case "smb":
		return m.startSMB(id, &cfg)
	default:
		return fmt.Errorf("不支持的协议: %s", cfg.Protocol)
	}
}

func (m *Manager) startWebDAV(id uint, cfg *model.StorageConfig) error {
	handler := &webdav.Handler{
		FileSystem: webdav.Dir(cfg.RootPath),
		LockSystem: webdav.NewMemLS(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 基础认证
		if cfg.Username != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != cfg.Username || pass != cfg.Password {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		handler.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.ListenPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("WebDAV 监听 %s 失败: %w", addr, err)
	}

	srv := &http.Server{Handler: mux}
	entry := &storageEntry{listener: ln, server: srv}
	m.entries.Store(id, entry)

	go func() {
		srv.Serve(ln)
		m.entries.Delete(id)
		m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("status", "stopped")
	}()

	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[WebDAV][%d] 已启动，监听 %s，根目录: %s", id, addr, cfg.RootPath)
	return nil
}

func (m *Manager) startSFTP(id uint, cfg *model.StorageConfig) error {
	// TODO: 集成 pkg/sftp 实现 SFTP 服务
	m.log.Infof("[SFTP][%d] SFTP 服务待实现", id)
	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": "SFTP 服务待实现",
	})
	return fmt.Errorf("SFTP 服务待实现")
}

func (m *Manager) startSMB(id uint, cfg *model.StorageConfig) error {
	// TODO: 集成 SMB 服务
	m.log.Infof("[SMB][%d] SMB 服务待实现", id)
	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": "SMB 服务待实现",
	})
	return fmt.Errorf("SMB 服务待实现")
}

func (m *Manager) Stop(id uint) {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*storageEntry)
		if entry.server != nil {
			entry.server.Close()
		}
		if entry.listener != nil {
			entry.listener.Close()
		}
		m.entries.Delete(id)
	}
	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.entries.Load(id); ok {
		return "running"
	}
	return "stopped"
}
