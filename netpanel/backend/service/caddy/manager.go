package caddy

import (
	"fmt"
	"sync"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager Caddy 网站服务管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	dataDir string
	running sync.Map // map[uint]bool
}

func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{db: db, log: log, dataDir: dataDir}
}

func (m *Manager) StartAll() {
	var sites []model.CaddySite
	m.db.Where("enable = ?", true).Find(&sites)
	for _, s := range sites {
		if err := m.Start(s.ID); err != nil {
			m.log.Errorf("Caddy 站点 [%s] 启动失败: %v", s.Name, err)
		}
	}
}

func (m *Manager) StopAll() {
	m.running.Range(func(key, value interface{}) bool {
		m.Stop(key.(uint))
		return true
	})
}

func (m *Manager) Start(id uint) error {
	var site model.CaddySite
	if err := m.db.First(&site, id).Error; err != nil {
		return fmt.Errorf("站点不存在: %w", err)
	}

	// TODO: 集成 Caddy 库，动态加载站点配置
	// 通过 caddy.Run() 或 Admin API 动态添加路由
	m.running.Store(id, true)
	m.db.Model(&model.CaddySite{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[Caddy][%d] 站点 %s 已启动", id, site.Domain)
	return nil
}

func (m *Manager) Stop(id uint) {
	m.running.Delete(id)
	m.db.Model(&model.CaddySite{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.running.Load(id); ok {
		return "running"
	}
	return "stopped"
}
