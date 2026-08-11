package ai

import (
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/robfig/cron/v3"
)

// ===== AI 定时任务管理 =====

// ListCronTasks 列出所有 AI 定时任务
func (m *Manager) ListCronTasks() ([]model.AiCronTask, error) {
	var list []model.AiCronTask
	err := m.db.Order("id desc").Find(&list).Error
	return list, err
}

// CreateCronTask 创建 AI 定时任务
func (m *Manager) CreateCronTask(t *model.AiCronTask) error {
	if err := m.db.Create(t).Error; err != nil {
		return err
	}
	if t.Enable {
		m.registerCronTask(t)
	}
	return nil
}

// UpdateCronTask 更新 AI 定时任务
func (m *Manager) UpdateCronTask(t *model.AiCronTask) error {
	// 先移除旧的调度
	m.unregisterCronTask(t.ID)

	if err := m.db.Save(t).Error; err != nil {
		return err
	}
	if t.Enable {
		m.registerCronTask(t)
	}
	return nil
}

// DeleteCronTask 删除 AI 定时任务
func (m *Manager) DeleteCronTask(id uint) error {
	m.unregisterCronTask(id)
	// 删除相关日志
	m.db.Where("task_id = ?", id).Delete(&model.AiCronLog{})
	return m.db.Delete(&model.AiCronTask{}, id).Error
}

// EnableCronTask 启用定时任务
func (m *Manager) EnableCronTask(id uint) error {
	var task model.AiCronTask
	if err := m.db.First(&task, id).Error; err != nil {
		return err
	}
	task.Enable = true
	task.Status = "running"
	m.db.Save(&task)
	m.registerCronTask(&task)
	return nil
}

// DisableCronTask 禁用定时任务
func (m *Manager) DisableCronTask(id uint) error {
	m.unregisterCronTask(id)
	return m.db.Model(&model.AiCronTask{}).Where("id = ?", id).
		Updates(map[string]interface{}{"enable": false, "status": "stopped"}).Error
}

// RunCronTaskNow 立即执行一次
func (m *Manager) RunCronTaskNow(id uint) error {
	var task model.AiCronTask
	if err := m.db.First(&task, id).Error; err != nil {
		return err
	}
	go m.executeCronTask(&task)
	return nil
}

// ListCronLogs 获取定时任务执行日志
func (m *Manager) ListCronLogs(taskID uint, limit int) ([]model.AiCronLog, error) {
	var logs []model.AiCronLog
	query := m.db.Where("task_id = ?", taskID).Order("id desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&logs).Error
	return logs, err
}

// startAllCronTasks 启动所有已启用的 AI 定时任务
func (m *Manager) startAllCronTasks() {
	var tasks []model.AiCronTask
	m.db.Where("enable = ?", true).Find(&tasks)
	for i := range tasks {
		m.registerCronTask(&tasks[i])
	}
	if len(tasks) > 0 {
		m.log.Infof("已加载 %d 个 AI 定时任务", len(tasks))
	}
}

// registerCronTask 注册定时任务到 cron 调度器
func (m *Manager) registerCronTask(task *model.AiCronTask) {
	entryID, err := m.cron.AddFunc(task.CronExpr, func() {
		m.executeCronTask(task)
	})
	if err != nil {
		m.log.Errorf("注册 AI 定时任务 [%s] 失败: %v", task.Name, err)
		m.db.Model(&model.AiCronTask{}).Where("id = ?", task.ID).
			Updates(map[string]interface{}{"status": "error", "last_run_result": err.Error()})
		return
	}
	m.cronEntries.Store(task.ID, entryID)
	m.db.Model(&model.AiCronTask{}).Where("id = ?", task.ID).Update("status", "running")
}

// unregisterCronTask 从 cron 调度器移除任务
func (m *Manager) unregisterCronTask(taskID uint) {
	if val, ok := m.cronEntries.LoadAndDelete(taskID); ok {
		m.cron.Remove(val.(cron.EntryID))
	}
}

// executeCronTask 执行 AI 定时任务
func (m *Manager) executeCronTask(task *model.AiCronTask) {
	startTime := time.Now()

	// 获取最新任务数据
	var latestTask model.AiCronTask
	if err := m.db.First(&latestTask, task.ID).Error; err != nil {
		return
	}

	provider, err := m.GetProvider(latestTask.ProviderID)
	if err != nil {
		m.saveCronLog(task.ID, latestTask.Prompt, "", latestTask.ModelName, 0, 0, startTime, false, "获取 API 来源失败: "+err.Error())
		return
	}

	req := &ChatCompletionRequest{
		Model: latestTask.ModelName,
		Messages: []ChatMessage{
			{Role: "user", Content: latestTask.Prompt},
		},
		MaxTokens: latestTask.MaxTokens,
	}

	resp, err := m.ChatCompletion(provider, req)
	if err != nil {
		m.saveCronLog(task.ID, latestTask.Prompt, "", latestTask.ModelName, 0, 0, startTime, false, err.Error())
		m.db.Model(&model.AiCronTask{}).Where("id = ?", task.ID).
			Updates(map[string]interface{}{
				"last_run_time":   time.Now(),
				"last_run_result": "执行失败: " + err.Error(),
			})
		return
	}

	result := ""
	if len(resp.Choices) > 0 {
		result = resp.Choices[0].Message.Content
	}

	m.saveCronLog(task.ID, latestTask.Prompt, result, latestTask.ModelName,
		resp.Usage.PromptTokens, resp.Usage.CompletionTokens, startTime, true, "")

	now := time.Now()
	m.db.Model(&model.AiCronTask{}).Where("id = ?", task.ID).
		Updates(map[string]interface{}{
			"last_run_time":   &now,
			"last_run_result": result,
			"last_run_tokens": resp.Usage.TotalTokens,
		})
}

// saveCronLog 保存执行日志
func (m *Manager) saveCronLog(taskID uint, prompt, result, modelName string, tokensPrompt, tokensComplete int, startTime time.Time, success bool, errMsg string) {
	duration := time.Since(startTime).Milliseconds()
	log := &model.AiCronLog{
		TaskID:         taskID,
		Prompt:         prompt,
		Result:         result,
		ModelName:      modelName,
		TokensPrompt:   tokensPrompt,
		TokensComplete: tokensComplete,
		DurationMs:     duration,
		Success:        success,
		ErrorMsg:       errMsg,
	}
	m.db.Create(log)
}
