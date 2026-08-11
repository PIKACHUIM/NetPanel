package ai

import (
	"encoding/json"
	"fmt"

	"github.com/netpanel/netpanel/model"
)

// ===== 对话管理 =====

// ListConversations 列出所有对话
func (m *Manager) ListConversations() ([]model.AiConversation, error) {
	var list []model.AiConversation
	err := m.db.Order("updated_at desc").Find(&list).Error
	return list, err
}

// CreateConversation 创建对话
func (m *Manager) CreateConversation(conv *model.AiConversation) error {
	return m.db.Create(conv).Error
}

// UpdateConversation 更新对话
func (m *Manager) UpdateConversation(conv *model.AiConversation) error {
	return m.db.Save(conv).Error
}

// DeleteConversation 删除对话及其消息
func (m *Manager) DeleteConversation(id uint) error {
	if err := m.db.Where("conversation_id = ?", id).Delete(&model.AiMessage{}).Error; err != nil {
		return err
	}
	return m.db.Delete(&model.AiConversation{}, id).Error
}

// GetConversation 获取对话详情
func (m *Manager) GetConversation(id uint) (*model.AiConversation, error) {
	var conv model.AiConversation
	err := m.db.First(&conv, id).Error
	return &conv, err
}

// ===== 消息管理 =====

// ListMessages 列出对话的所有消息
func (m *Manager) ListMessages(conversationID uint) ([]model.AiMessage, error) {
	var msgs []model.AiMessage
	err := m.db.Where("conversation_id = ?", conversationID).Order("id asc").Find(&msgs).Error
	return msgs, err
}

// AddMessage 添加消息
func (m *Manager) AddMessage(msg *model.AiMessage) error {
	if err := m.db.Create(msg).Error; err != nil {
		return err
	}
	// 更新对话消息计数
	m.db.Model(&model.AiConversation{}).Where("id = ?", msg.ConversationID).
		UpdateColumn("message_count", m.db.Raw("message_count + 1"))
	return nil
}

// BuildChatMessages 构建完整的对话消息列表（包含系统提示和历史消息）
func (m *Manager) BuildChatMessages(conversationID uint, assistantID uint, userInput string) ([]ChatMessage, error) {
	var messages []ChatMessage

	// 如果关联了助理，获取 system prompt
	if assistantID > 0 {
		var assistant model.AiAssistant
		if err := m.db.First(&assistant, assistantID).Error; err == nil && assistant.SystemPrompt != "" {
			messages = append(messages, ChatMessage{
				Role:    "system",
				Content: assistant.SystemPrompt,
			})
		}
	}

	// 加载历史消息
	var historyMsgs []model.AiMessage
	m.db.Where("conversation_id = ?", conversationID).Order("id asc").Find(&historyMsgs)
	for _, msg := range historyMsgs {
		messages = append(messages, ChatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// 添加当前用户输入
	messages = append(messages, ChatMessage{
		Role:    "user",
		Content: userInput,
	})

	return messages, nil
}

// SendMessage 发送消息（非流式）
func (m *Manager) SendMessage(conversationID uint, userInput string) (*model.AiMessage, error) {
	conv, err := m.GetConversation(conversationID)
	if err != nil {
		return nil, fmt.Errorf("获取对话失败: %w", err)
	}

	provider, err := m.GetProvider(conv.ProviderID)
	if err != nil {
		return nil, fmt.Errorf("获取 API 来源失败: %w", err)
	}

	chatMsgs, err := m.BuildChatMessages(conversationID, conv.AssistantID, userInput)
	if err != nil {
		return nil, err
	}

	// 保存用户消息
	userMsg := &model.AiMessage{
		ConversationID: conversationID,
		Role:           "user",
		Content:        userInput,
	}
	m.AddMessage(userMsg)

	// 调用 API
	req := &ChatCompletionRequest{
		Model:    conv.ModelName,
		Messages: chatMsgs,
	}
	resp, err := m.ChatCompletion(provider, req)
	if err != nil {
		return nil, fmt.Errorf("AI 请求失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI 返回空响应")
	}

	// 保存助手回复
	assistantMsg := &model.AiMessage{
		ConversationID:   conversationID,
		Role:             "assistant",
		Content:          resp.Choices[0].Message.Content,
		TokensPrompt:     resp.Usage.PromptTokens,
		TokensCompletion: resp.Usage.CompletionTokens,
	}
	m.AddMessage(assistantMsg)

	// 更新对话消息计数
	m.db.Model(&model.AiConversation{}).Where("id = ?", conversationID).
		Update("message_count", len(chatMsgs)+1)

	return assistantMsg, nil
}

// ExportConversation 导出对话为 JSON
func (m *Manager) ExportConversation(id uint) ([]byte, error) {
	conv, err := m.GetConversation(id)
	if err != nil {
		return nil, err
	}
	msgs, err := m.ListMessages(id)
	if err != nil {
		return nil, err
	}

	export := map[string]interface{}{
		"conversation": conv,
		"messages":     msgs,
	}
	return json.MarshalIndent(export, "", "  ")
}

// ImportConversation 导入对话
func (m *Manager) ImportConversation(data []byte) (*model.AiConversation, error) {
	var importData struct {
		Conversation model.AiConversation `json:"conversation"`
		Messages     []model.AiMessage    `json:"messages"`
	}
	if err := json.Unmarshal(data, &importData); err != nil {
		return nil, fmt.Errorf("解析导入数据失败: %w", err)
	}

	// 创建新对话（重置 ID）
	conv := importData.Conversation
	conv.ID = 0
	if err := m.db.Create(&conv).Error; err != nil {
		return nil, err
	}

	// 导入消息
	for _, msg := range importData.Messages {
		msg.ID = 0
		msg.ConversationID = conv.ID
		m.db.Create(&msg)
	}

	return &conv, nil
}
