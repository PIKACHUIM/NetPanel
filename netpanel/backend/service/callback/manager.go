package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// TriggerEvent 触发事件
type TriggerEvent struct {
	Type      string // "stun_ip_change"
	SourceID  uint
	NewIP     string
	NewPort   int
	OldIP     string
	OldPort   int
}

// Manager 回调任务管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	eventCh chan TriggerEvent
	stopCh  chan struct{}
	once    sync.Once
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{
		db:      db,
		log:     log,
		eventCh: make(chan TriggerEvent, 64),
		stopCh:  make(chan struct{}),
	}
}

func (m *Manager) Start() {
	m.once.Do(func() {
		go m.processEvents()
	})
}

func (m *Manager) Stop() {
	close(m.stopCh)
}

// Trigger 触发回调事件
func (m *Manager) Trigger(event TriggerEvent) {
	select {
	case m.eventCh <- event:
	default:
		m.log.Warn("[回调] 事件队列已满，丢弃事件")
	}
}

func (m *Manager) processEvents() {
	for {
		select {
		case <-m.stopCh:
			return
		case event := <-m.eventCh:
			m.handleEvent(event)
		}
	}
}

func (m *Manager) handleEvent(event TriggerEvent) {
	// 查找匹配的回调任务
	var tasks []model.CallbackTask
	m.db.Where("enable = ? AND trigger_type = ?", true, event.Type).Find(&tasks)

	for _, task := range tasks {
		// 检查触发来源是否匹配
		if task.TriggerSourceID != 0 && task.TriggerSourceID != event.SourceID {
			continue
		}
		go m.executeTask(&task, &event)
	}
}

func (m *Manager) executeTask(task *model.CallbackTask, event *TriggerEvent) {
	m.log.Infof("[回调] 执行任务 [%s]，触发事件: %s", task.Name, event.Type)

	// 获取回调账号
	var account model.CallbackAccount
	if err := m.db.First(&account, task.AccountID).Error; err != nil {
		m.log.Errorf("[回调] 账号不存在: %v", err)
		return
	}

	var err error
	switch account.AccountType {
	case "webhook":
		err = m.executeWebhook(&account, task, event)
	case "cf_origin":
		err = m.executeCFOrigin(&account, task, event)
	case "ali_esa":
		err = m.executeAliESA(&account, task, event)
	case "tencent_eo":
		err = m.executeTencentEO(&account, task, event)
	default:
		err = fmt.Errorf("不支持的账号类型: %s", account.AccountType)
	}

	if err != nil {
		m.log.Errorf("[回调] 任务 [%s] 执行失败: %v", task.Name, err)
		m.db.Model(&model.CallbackTask{}).Where("id = ?", task.ID).Update("last_error", err.Error())
	} else {
		m.log.Infof("[回调] 任务 [%s] 执行成功", task.Name)
		m.db.Model(&model.CallbackTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"last_run_time": time.Now(),
			"last_error":    "",
		})
	}
}

// executeWebhook 执行 Webhook 回调
func (m *Manager) executeWebhook(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(account.Config), &cfg); err != nil {
		return fmt.Errorf("解析账号配置失败: %w", err)
	}

	url := cfg["url"]
	method := cfg["method"]
	if method == "" {
		method = "POST"
	}

	payload := map[string]interface{}{
		"event":    event.Type,
		"new_ip":   event.NewIP,
		"new_port": event.NewPort,
		"old_ip":   event.OldIP,
		"old_port": event.OldPort,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Webhook 返回错误状态码: %d", resp.StatusCode)
	}
	return nil
}

// executeCFOrigin 更新 Cloudflare 回源端口
func (m *Manager) executeCFOrigin(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	// TODO: 调用 Cloudflare API 更新回源规则端口
	return fmt.Errorf("CF 回源端口更新待实现")
}

// executeAliESA 更新阿里云 ESA
func (m *Manager) executeAliESA(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	// TODO: 调用阿里云 ESA API
	return fmt.Errorf("阿里云 ESA 更新待实现")
}

// executeTencentEO 更新腾讯云 EO
func (m *Manager) executeTencentEO(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	// TODO: 调用腾讯云 EO API
	return fmt.Errorf("腾讯云 EO 更新待实现")
}

// TestAccount 测试回调账号连通性
func (m *Manager) TestAccount(id uint) error {
	var account model.CallbackAccount
	if err := m.db.First(&account, id).Error; err != nil {
		return fmt.Errorf("账号不存在: %w", err)
	}

	testEvent := &TriggerEvent{
		Type:    "test",
		NewIP:   "1.2.3.4",
		NewPort: 12345,
	}

	switch account.AccountType {
	case "webhook":
		return m.executeWebhook(&account, nil, testEvent)
	default:
		return fmt.Errorf("账号类型 %s 暂不支持测试", account.AccountType)
	}
}
