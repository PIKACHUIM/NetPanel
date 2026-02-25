package cron

import (
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager 计划任务管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	cron    *cron.Cron
	entryIDs sync.Map // map[uint]cron.EntryID
	mu      sync.Mutex
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	c := cron.New(cron.WithSeconds())
	c.Start()
	return &Manager{db: db, log: log, cron: c}
}

func (m *Manager) StartAll() {
	var tasks []model.CronTask
	m.db.Where("enable = ?", true).Find(&tasks)
	for i := range tasks {
		if err := m.AddTask(&tasks[i]); err != nil {
			m.log.Errorf("计划任务 [%s] 添加失败: %v", tasks[i].Name, err)
		}
	}
}

func (m *Manager) StopAll() {
	m.cron.Stop()
}

func (m *Manager) AddTask(task *model.CronTask) error {
	m.RemoveTask(task.ID)

	entryID, err := m.cron.AddFunc(task.CronExpr, func() {
		m.executeTask(task.ID)
	})
	if err != nil {
		return fmt.Errorf("添加计划任务失败: %w", err)
	}

	m.entryIDs.Store(task.ID, entryID)
	m.db.Model(&model.CronTask{}).Where("id = ?", task.ID).Update("status", "running")
	m.log.Infof("[Cron][%d] 任务 %s 已添加，表达式: %s", task.ID, task.Name, task.CronExpr)
	return nil
}

func (m *Manager) RemoveTask(id uint) {
	if val, ok := m.entryIDs.Load(id); ok {
		m.cron.Remove(val.(cron.EntryID))
		m.entryIDs.Delete(id)
	}
	m.db.Model(&model.CronTask{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) RunNow(id uint) error {
	var task model.CronTask
	if err := m.db.First(&task, id).Error; err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}
	go m.executeTask(id)
	return nil
}

func (m *Manager) executeTask(id uint) {
	var task model.CronTask
	if err := m.db.First(&task, id).Error; err != nil {
		return
	}

	m.log.Infof("[Cron][%d] 开始执行任务: %s", id, task.Name)
	now := time.Now()
	var result string
	var execErr error

	switch task.TaskType {
	case "shell":
		result, execErr = m.runShell(task.Command)
	case "http":
		result, execErr = m.runHTTP(task.HTTPURL, task.HTTPMethod, task.HTTPBody)
	default:
		result = "未知任务类型"
	}

	if execErr != nil {
		m.log.Errorf("[Cron][%d] 任务执行失败: %v", id, execErr)
		result = "错误: " + execErr.Error()
	} else {
		m.log.Infof("[Cron][%d] 任务执行成功", id)
	}

	m.db.Model(&model.CronTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"last_run_time":   now,
		"last_run_result": result,
	})
}

func (m *Manager) runShell(command string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (m *Manager) runHTTP(url, method, body string) (string, error) {
	if method == "" {
		method = "GET"
	}
	client := &http.Client{Timeout: 30 * time.Second}
	var req *http.Request
	var err error
	if body != "" {
		req, err = http.NewRequest(method, url, strings.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	return fmt.Sprintf("HTTP %d", resp.StatusCode), nil
}
