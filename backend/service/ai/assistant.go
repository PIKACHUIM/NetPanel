package ai

import "github.com/netpanel/netpanel/model"

// ===== 助理管理 =====

// ListAssistants 列出所有助理
func (m *Manager) ListAssistants() ([]model.AiAssistant, error) {
	var list []model.AiAssistant
	err := m.db.Order("id desc").Find(&list).Error
	return list, err
}

// CreateAssistant 创建助理
func (m *Manager) CreateAssistant(a *model.AiAssistant) error {
	return m.db.Create(a).Error
}

// UpdateAssistant 更新助理
func (m *Manager) UpdateAssistant(a *model.AiAssistant) error {
	return m.db.Save(a).Error
}

// DeleteAssistant 删除助理
func (m *Manager) DeleteAssistant(id uint) error {
	return m.db.Delete(&model.AiAssistant{}, id).Error
}

// GetAssistant 获取单个助理
func (m *Manager) GetAssistant(id uint) (*model.AiAssistant, error) {
	var a model.AiAssistant
	err := m.db.First(&a, id).Error
	return &a, err
}
