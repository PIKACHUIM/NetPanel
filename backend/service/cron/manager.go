package cron

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/cert"
	"github.com/netpanel/netpanel/service/ddns"
	"github.com/netpanel/netpanel/service/wol"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SyncDNSRecordFunc 同步 DNS 解析记录的回调函数类型
type SyncDNSRecordFunc func(ctx context.Context, domainInfoID uint) (int, error)

// Manager 计划任务管理器
type Manager struct {
	db       *gorm.DB
	log      *logrus.Logger
	cron     *cron.Cron
	entryIDs sync.Map // map[uint]cron.EntryID
	mu       sync.Mutex
	certMgr  *cert.Manager
	ddnsMgr  *ddns.Manager
	wolMgr   *wol.Manager
	syncDNSRecordFunc SyncDNSRecordFunc // 由外部注入的 DNS 解析记录同步函数
}

func NewManager(db *gorm.DB, log *logrus.Logger, certMgr *cert.Manager, ddnsMgr *ddns.Manager, wolMgr *wol.Manager) *Manager {
	c := cron.New(cron.WithSeconds())
	c.Start()
	return &Manager{db: db, log: log, cron: c, certMgr: certMgr, ddnsMgr: ddnsMgr, wolMgr: wolMgr}
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
	case "renew_cert":
		result, execErr = m.runRenewCert(task.TargetID)
	case "update_ddns":
		result, execErr = m.runUpdateDDNS(task.TargetID)
	case "wol":
		result, execErr = m.runWOL(task.TargetID)
	case "sync_dns_record":
		result, execErr = m.runSyncDNSRecord(task.TargetID)
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

// shellTaskEnabled 是否允许执行 shell 类型的计划任务。
//
// 安全说明：shell 任务以面板进程权限（通常为 root/Administrator）执行任意命令，
// 且配置持久化后重启仍会执行，等价于一个可远程写入的后门。
// 因此默认关闭，仅在显式设置 NETPANEL_ENABLE_SHELL_TASK=1 时启用。
func shellTaskEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("NETPANEL_ENABLE_SHELL_TASK")))
	return v == "1" || v == "true" || v == "yes"
}

// ShellTaskEnabled 供 API 层做前置校验与提示。
func ShellTaskEnabled() bool { return shellTaskEnabled() }

const (
	// shellTimeout 单次 shell 任务的最长执行时间，防止 goroutine 与进程被永久占用
	shellTimeout = 5 * time.Minute
	// shellMaxOutput 输出上限（字节），防止 `yes` 之类命令耗尽内存
	shellMaxOutput = 256 * 1024
)

func (m *Manager) runShell(command string) (string, error) {
	if !shellTaskEnabled() {
		return "", fmt.Errorf("shell 任务已禁用：如需启用请设置环境变量 NETPANEL_ENABLE_SHELL_TASK=1")
	}
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("命令为空")
	}

	// 超时控制：原实现使用 exec.Command + CombinedOutput，无超时且无输出上限
	ctx, cancel := context.WithTimeout(context.Background(), shellTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	// 限制输出大小，避免大量输出直接读入内存
	var buf bytes.Buffer
	limited := &limitedWriter{w: &buf, remaining: shellMaxOutput}
	cmd.Stdout = limited
	cmd.Stderr = limited

	err := cmd.Run()
	out := buf.String()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("命令执行超时（超过 %s）", shellTimeout)
	}
	if limited.truncated {
		out += "\n...[输出已截断]"
	}
	return out, err
}

// limitedWriter 限制写入总量的 io.Writer，超出后丢弃剩余内容并标记截断。
type limitedWriter struct {
	w         io.Writer
	remaining int
	truncated bool
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.remaining <= 0 {
		l.truncated = true
		return len(p), nil // 声称已写入，避免命令因写失败而提前中断
	}
	if len(p) > l.remaining {
		l.truncated = true
		if _, err := l.w.Write(p[:l.remaining]); err != nil {
			return 0, err
		}
		l.remaining = 0
		return len(p), nil
	}
	n, err := l.w.Write(p)
	l.remaining -= n
	return n, err
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

// runRenewCert 续签 SSL 证书
func (m *Manager) runRenewCert(targetID uint) (string, error) {
	if m.certMgr == nil {
		return "", fmt.Errorf("证书管理器未初始化")
	}
	if targetID == 0 {
		return "", fmt.Errorf("未指定证书 ID")
	}
	var certRecord model.DomainCert
	if err := m.db.First(&certRecord, targetID).Error; err != nil {
		return "", fmt.Errorf("证书不存在(ID=%d): %w", targetID, err)
	}
	if err := m.certMgr.Apply(targetID); err != nil {
		return "", fmt.Errorf("证书续签失败: %w", err)
	}
	return fmt.Sprintf("证书 [%s] 续签成功", certRecord.Name), nil
}

// runUpdateDDNS 更新 DDNS 记录
func (m *Manager) runUpdateDDNS(targetID uint) (string, error) {
	if m.ddnsMgr == nil {
		return "", fmt.Errorf("DDNS 管理器未初始化")
	}
	if targetID == 0 {
		return "", fmt.Errorf("未指定 DDNS 任务 ID")
	}
	var ddnsTask model.DDNSTask
	if err := m.db.First(&ddnsTask, targetID).Error; err != nil {
		return "", fmt.Errorf("DDNS 任务不存在(ID=%d): %w", targetID, err)
	}
	if err := m.ddnsMgr.RunNow(targetID); err != nil {
		return "", fmt.Errorf("DDNS 更新失败: %w", err)
	}
	return fmt.Sprintf("DDNS 任务 [%s] 已触发更新", ddnsTask.Name), nil
}

// runWOL 执行网络唤醒
func (m *Manager) runWOL(targetID uint) (string, error) {
	if m.wolMgr == nil {
		return "", fmt.Errorf("WOL 管理器未初始化")
	}
	if targetID == 0 {
		return "", fmt.Errorf("未指定 WOL 设备 ID")
	}
	var device model.WolDevice
	if err := m.db.First(&device, targetID).Error; err != nil {
		return "", fmt.Errorf("WOL 设备不存在(ID=%d): %w", targetID, err)
	}
	if err := m.wolMgr.Wake(targetID); err != nil {
		return "", fmt.Errorf("网络唤醒失败: %w", err)
	}
	return fmt.Sprintf("WOL 设备 [%s] (MAC: %s) 唤醒包已发送", device.Name, device.MACAddress), nil
}

// SetSyncDNSRecordFunc 设置 DNS 解析记录同步回调函数
func (m *Manager) SetSyncDNSRecordFunc(fn SyncDNSRecordFunc) {
	m.syncDNSRecordFunc = fn
}

// runSyncDNSRecord 同步 DNS 解析记录
func (m *Manager) runSyncDNSRecord(targetID uint) (string, error) {
	if m.syncDNSRecordFunc == nil {
		return "", fmt.Errorf("DNS 解析记录同步函数未初始化")
	}
	if targetID == 0 {
		return "", fmt.Errorf("未指定域名 ID")
	}
	var domainInfo model.DomainInfo
	if err := m.db.First(&domainInfo, targetID).Error; err != nil {
		return "", fmt.Errorf("域名不存在(ID=%d): %w", targetID, err)
	}
	count, err := m.syncDNSRecordFunc(context.Background(), targetID)
	if err != nil {
		return "", fmt.Errorf("DNS 解析记录同步失败: %w", err)
	}
	return fmt.Sprintf("域名 [%s] 解析记录同步成功，共 %d 条记录", domainInfo.Name, count), nil
}