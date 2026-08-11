package ai

import (
	"fmt"

	"github.com/netpanel/netpanel/model"
)

// ===== 插件管理 =====

// ListPlugins 列出所有插件
func (m *Manager) ListPlugins(pluginType string) ([]model.AiPlugin, error) {
	var list []model.AiPlugin
	query := m.db.Order("id desc")
	if pluginType != "" {
		query = query.Where("type = ?", pluginType)
	}
	err := query.Find(&list).Error
	return list, err
}

// CreatePlugin 创建插件
func (m *Manager) CreatePlugin(p *model.AiPlugin) error {
	return m.db.Create(p).Error
}

// UpdatePlugin 更新插件
func (m *Manager) UpdatePlugin(p *model.AiPlugin) error {
	return m.db.Save(p).Error
}

// DeletePlugin 删除插件（系统内置不可删除）
func (m *Manager) DeletePlugin(id uint) error {
	var plugin model.AiPlugin
	if err := m.db.First(&plugin, id).Error; err != nil {
		return err
	}
	if plugin.IsSystem {
		return fmt.Errorf("系统内置插件不可删除")
	}
	return m.db.Delete(&model.AiPlugin{}, id).Error
}

// TogglePlugin 切换插件启用状态
func (m *Manager) TogglePlugin(id uint, active bool) error {
	return m.db.Model(&model.AiPlugin{}).Where("id = ?", id).Update("is_active", active).Error
}

// GetPlugin 获取单个插件
func (m *Manager) GetPlugin(id uint) (*model.AiPlugin, error) {
	var p model.AiPlugin
	err := m.db.First(&p, id).Error
	return &p, err
}
