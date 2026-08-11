package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// TaskEngine 任务调度引擎
type TaskEngine struct {
	db      *gorm.DB
	manager *Manager
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Cron 调度器
	cronScheduler *cron.Cron
	cronEntries   map[uint]cron.EntryID // 任务ID -> Cron EntryID
	mu            sync.RWMutex
}

// NewTaskEngine 创建任务引擎
func NewTaskEngine(db *gorm.DB, manager *Manager) *TaskEngine {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &TaskEngine{
		db:            db,
		manager:       manager,
		ctx:           ctx,
		cancel:        cancel,
		cronScheduler: cron.New(cron.WithSeconds()),
		cronEntries:   make(map[uint]cron.EntryID),
	}
}

// Start 启动任务引擎
func (t *TaskEngine) Start() {
	log.Println("[TaskEngine] 启动任务引擎...")
	
	// 启动 Cron 调度器
	t.cronScheduler.Start()
	
	// 加载所有启用的 Cron 任务
	var tasks []model.MonitorTask
	t.db.Where("enable = ? AND task_type = ?", true, "cron").Find(&tasks)
	
	for _, task := range tasks {
		if err := t.ScheduleCronTask(task); err != nil {
			log.Printf("[TaskEngine] 调度任务失败 (%s): %v\n", task.Name, err)
		}
	}
	
	log.Printf("[TaskEngine] 任务引擎启动完成，加载了 %d 个计划任务\n", len(tasks))
}

// Stop 停止任务引擎
func (t *TaskEngine) Stop() {
	log.Println("[TaskEngine] 停止任务引擎...")
	
	t.cancel()
	
	// 停止 Cron 调度器
	ctx := t.cronScheduler.Stop()
	<-ctx.Done()
	
	t.wg.Wait()
	log.Println("[TaskEngine] 任务引擎已停止")
}

// ScheduleCronTask 调度 Cron 任务
func (t *TaskEngine) ScheduleCronTask(task model.MonitorTask) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// 如果已存在，先移除
	if entryID, ok := t.cronEntries[task.ID]; ok {
		t.cronScheduler.Remove(entryID)
		delete(t.cronEntries, task.ID)
	}
	
	// 添加到 Cron 调度器
	entryID, err := t.cronScheduler.AddFunc(task.CronExpr, func() {
		t.ExecuteTask(task.ID)
	})
	
	if err != nil {
		return fmt.Errorf("添加 Cron 任务失败: %w", err)
	}
	
	t.cronEntries[task.ID] = entryID
	log.Printf("[TaskEngine] 调度任务: %s (Cron: %s)\n", task.Name, task.CronExpr)
	
	return nil
}

// UnscheduleCronTask 取消调度 Cron 任务
func (t *TaskEngine) UnscheduleCronTask(taskID uint) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if entryID, ok := t.cronEntries[taskID]; ok {
		t.cronScheduler.Remove(entryID)
		delete(t.cronEntries, taskID)
		log.Printf("[TaskEngine] 取消调度任务: %d\n", taskID)
	}
}

// ExecuteTask 执行任务
func (t *TaskEngine) ExecuteTask(taskID uint) {
	var task model.MonitorTask
	if err := t.db.First(&task, taskID).Error; err != nil {
		log.Printf("[TaskEngine] 任务不存在: %d\n", taskID)
		return
	}
	
	if !task.Enable {
		log.Printf("[TaskEngine] 任务已禁用: %s\n", task.Name)
		return
	}
	
	log.Printf("[TaskEngine] 开始执行任务: %s\n", task.Name)
	
	// 解析目标服务器列表
	var serverIDs []uint
	if err := json.Unmarshal([]byte(task.ServerIDs), &serverIDs); err != nil {
		log.Printf("[TaskEngine] 解析服务器列表失败: %v\n", err)
		return
	}
	
	// 更新最后执行时间
	now := time.Now()
	t.db.Model(&task).Update("last_run_time", now)
	
	// 执行任务
	if task.Concurrent {
		// 并发执行
		var wg sync.WaitGroup
		for _, serverID := range serverIDs {
			wg.Add(1)
			go func(sid uint) {
				defer wg.Done()
				t.executeTaskOnServer(task, sid)
			}(serverID)
		}
		wg.Wait()
	} else {
		// 串行执行
		for _, serverID := range serverIDs {
			t.executeTaskOnServer(task, serverID)
		}
	}
	
	log.Printf("[TaskEngine] 任务执行完成: %s\n", task.Name)
}

// executeTaskOnServer 在指定服务器上执行任务
func (t *TaskEngine) executeTaskOnServer(task model.MonitorTask, serverID uint) {
	startTime := time.Now()
	
	// 创建任务日志
	taskLog := &model.MonitorTaskLog{
		TaskID:    task.ID,
		ServerID:  serverID,
		StartTime: startTime,
		Status:    "running",
	}
	t.db.Create(taskLog)
	
	// 执行命令
	output, err := t.manager.ExecuteCommandOnServer(serverID, task.Command, task.Timeout)
	
	endTime := time.Now()
	taskLog.EndTime = &endTime
	
	if err != nil {
		taskLog.Status = "failed"
		taskLog.Stderr = err.Error()
		taskLog.ExitCode = 1
	} else {
		taskLog.Status = "success"
		taskLog.Stdout = output
		taskLog.ExitCode = 0
	}
	
	// 更新任务日志
	t.db.Save(taskLog)
	
	log.Printf("[TaskEngine] 任务 %s 在服务器 %d 上执行完成，状态: %s\n", task.Name, serverID, taskLog.Status)
}

// ListTasks 列出任务
func (t *TaskEngine) ListTasks(enable *bool, taskType string) ([]model.MonitorTask, error) {
	query := t.db.Model(&model.MonitorTask{})
	
	if enable != nil {
		query = query.Where("enable = ?", *enable)
	}
	
	if taskType != "" {
		query = query.Where("task_type = ?", taskType)
	}
	
	var tasks []model.MonitorTask
	err := query.Order("id ASC").Find(&tasks).Error
	return tasks, err
}

// CreateTask 创建任务
func (t *TaskEngine) CreateTask(task *model.MonitorTask) error {
	if err := t.db.Create(task).Error; err != nil {
		return err
	}
	
	// 如果是 Cron 任务且已启用，调度它
	if task.Enable && task.TaskType == "cron" {
		if err := t.ScheduleCronTask(*task); err != nil {
			return err
		}
	}
	
	return nil
}

// UpdateTask 更新任务
func (t *TaskEngine) UpdateTask(task *model.MonitorTask) error {
	if err := t.db.Save(task).Error; err != nil {
		return err
	}
	
	// 重新调度 Cron 任务
	if task.TaskType == "cron" {
		t.UnscheduleCronTask(task.ID)
		if task.Enable {
			if err := t.ScheduleCronTask(*task); err != nil {
				return err
			}
		}
	}
	
	return nil
}

// DeleteTask 删除任务
func (t *TaskEngine) DeleteTask(id uint) error {
	// 取消调度
	t.UnscheduleCronTask(id)
	
	return t.db.Transaction(func(tx *gorm.DB) error {
		// 删除任务
		if err := tx.Delete(&model.MonitorTask{}, id).Error; err != nil {
			return err
		}
		
		// 删除任务日志
		tx.Where("task_id = ?", id).Delete(&model.MonitorTaskLog{})
		
		return nil
	})
}

// GetTaskLogs 获取任务日志
func (t *TaskEngine) GetTaskLogs(taskID, serverID uint, start, end time.Time, limit int) ([]model.MonitorTaskLog, error) {
	query := t.db.Model(&model.MonitorTaskLog{})
	
	if taskID > 0 {
		query = query.Where("task_id = ?", taskID)
	}
	
	if serverID > 0 {
		query = query.Where("server_id = ?", serverID)
	}
	
	if !start.IsZero() && !end.IsZero() {
		query = query.Where("start_time BETWEEN ? AND ?", start, end)
	}
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	var logs []model.MonitorTaskLog
	err := query.Order("start_time DESC").Find(&logs).Error
	return logs, err
}

// ExecuteManualTask 手动执行任务
func (t *TaskEngine) ExecuteManualTask(taskID uint) error {
	go t.ExecuteTask(taskID)
	return nil
}

// TriggerTask 触发任务（用于触发型任务）
func (t *TaskEngine) TriggerTask(triggerEvent string, serverID uint) {
	var tasks []model.MonitorTask
	t.db.Where("enable = ? AND task_type = ? AND trigger_event = ?", true, "trigger", triggerEvent).Find(&tasks)
	
	for _, task := range tasks {
		// 检查服务器是否在目标列表中
		var serverIDs []uint
		if err := json.Unmarshal([]byte(task.ServerIDs), &serverIDs); err != nil {
			continue
		}
		
		inList := false
		for _, sid := range serverIDs {
			if sid == serverID {
				inList = true
				break
			}
		}
		
		if inList || len(serverIDs) == 0 {
			// 执行任务
			go t.ExecuteTask(task.ID)
		}
	}
}
