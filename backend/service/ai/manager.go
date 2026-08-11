package ai

import (
	"sync"

	"github.com/netpanel/netpanel/model"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager AI 服务总管理器
type Manager struct {
	db   *gorm.DB
	log  *logrus.Logger
	cron *cron.Cron

	// 运行中的定时任务 entryID 映射
	cronEntries sync.Map // map[uint]cron.EntryID
}

// NewManager 创建 AI 管理器
func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{
		db:   db,
		log:  log,
		cron: cron.New(cron.WithSeconds()),
	}
}

// Start 启动 AI 服务（加载已启用的定时任务）
func (m *Manager) Start() {
	m.cron.Start()
	m.startAllCronTasks()
	m.log.Info("AI 服务管理器已启动")
}

// Stop 停止 AI 服务
func (m *Manager) Stop() {
	m.cron.Stop()
	m.log.Info("AI 服务管理器已停止")
}

// ===== Provider 相关 =====

// ListProviders 列出所有 API 来源
func (m *Manager) ListProviders() ([]model.AiProvider, error) {
	var providers []model.AiProvider
	err := m.db.Order("id desc").Find(&providers).Error
	return providers, err
}

// CreateProvider 创建 API 来源
func (m *Manager) CreateProvider(p *model.AiProvider) error {
	return m.db.Create(p).Error
}

// UpdateProvider 更新 API 来源
func (m *Manager) UpdateProvider(p *model.AiProvider) error {
	return m.db.Save(p).Error
}

// DeleteProvider 删除 API 来源
func (m *Manager) DeleteProvider(id uint) error {
	return m.db.Delete(&model.AiProvider{}, id).Error
}

// GetProvider 获取单个 Provider
func (m *Manager) GetProvider(id uint) (*model.AiProvider, error) {
	var p model.AiProvider
	err := m.db.First(&p, id).Error
	return &p, err
}
