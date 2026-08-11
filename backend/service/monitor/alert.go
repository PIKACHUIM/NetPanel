package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// AlertEngine 告警规则引擎
type AlertEngine struct {
	db      *gorm.DB
	manager *Manager
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// 告警状态缓存
	alertStates sync.Map // key: "alert_id:server_id", value: *AlertState
}

// AlertState 告警状态
type AlertState struct {
	AlertID         uint
	ServerID        uint
	IsTriggered     bool
	TriggerCount    int       // 连续触发次数
	LastTriggerTime time.Time
	LastNotifyTime  time.Time
	RecordID        uint // 当前告警记录 ID
	mu              sync.RWMutex
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	Operator string  `json:"operator"` // gt/lt/eq/gte/lte
	Value    float64 `json:"value"`
	Duration int     `json:"duration"` // 持续时间（秒）
}

// NewAlertEngine 创建告警引擎
func NewAlertEngine(db *gorm.DB, manager *Manager) *AlertEngine {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &AlertEngine{
		db:      db,
		manager: manager,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动告警引擎
func (a *AlertEngine) Start() {
	log.Println("[AlertEngine] 启动告警引擎...")
	
	// 启动告警检查协程
	a.wg.Add(1)
	go a.alertChecker()
	
	log.Println("[AlertEngine] 告警引擎启动完成")
}

// Stop 停止告警引擎
func (a *AlertEngine) Stop() {
	log.Println("[AlertEngine] 停止告警引擎...")
	
	a.cancel()
	a.wg.Wait()
	
	log.Println("[AlertEngine] 告警引擎已停止")
}

// alertChecker 告警检查器
func (a *AlertEngine) alertChecker() {
	defer a.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.checkAlerts()
		}
	}
}

// checkAlerts 检查所有告警规则
func (a *AlertEngine) checkAlerts() {
	var alerts []model.MonitorAlert
	a.db.Where("enable = ?", true).Find(&alerts)
	
	for _, alert := range alerts {
		a.checkAlert(alert)
	}
}

// checkAlert 检查单个告警规则
func (a *AlertEngine) checkAlert(alert model.MonitorAlert) {
	// 解析目标服务器
	var targets []string
	if err := json.Unmarshal([]byte(alert.TargetServers), &targets); err != nil {
		log.Printf("[AlertEngine] 解析目标服务器失败: %v\n", err)
		return
	}
	
	// 获取目标服务器列表
	serverIDs := a.resolveTargets(targets)
	
	// 检查每个服务器
	for _, serverID := range serverIDs {
		a.checkServerAlert(alert, serverID)
	}
}

// resolveTargets 解析目标服务器
func (a *AlertEngine) resolveTargets(targets []string) []uint {
	var serverIDs []uint
	
	for _, target := range targets {
		var id uint
		var groupName string
		
		// 解析目标：server:1 或 group:default
		if _, err := fmt.Sscanf(target, "server:%d", &id); err == nil {
			serverIDs = append(serverIDs, id)
		} else if _, err := fmt.Sscanf(target, "group:%s", &groupName); err == nil {
			// 查询组内所有服务器
			var servers []model.MonitorServer
			a.db.Where("group_name = ? AND enable = ?", groupName, true).Find(&servers)
			for _, s := range servers {
				serverIDs = append(serverIDs, s.ID)
			}
		}
	}
	
	return serverIDs
}

// checkServerAlert 检查服务器告警
func (a *AlertEngine) checkServerAlert(alert model.MonitorAlert, serverID uint) {
	// 解析阈值配置
	var config ThresholdConfig
	if err := json.Unmarshal([]byte(alert.ThresholdConfig), &config); err != nil {
		log.Printf("[AlertEngine] 解析阈值配置失败: %v\n", err)
		return
	}
	
	// 根据告警类型检查
	triggered := false
	var alertContent string
	
	switch alert.AlertType {
	case "cpu":
		triggered, alertContent = a.checkCPUAlert(serverID, config)
	case "memory":
		triggered, alertContent = a.checkMemoryAlert(serverID, config)
	case "disk":
		triggered, alertContent = a.checkDiskAlert(serverID, config)
	case "network":
		triggered, alertContent = a.checkNetworkAlert(serverID, config)
	case "process":
		triggered, alertContent = a.checkProcessAlert(serverID, config)
	case "offline":
		triggered, alertContent = a.checkOfflineAlert(serverID)
	}
	
	// 处理告警状态
	a.handleAlertState(alert, serverID, triggered, alertContent)
}

// checkCPUAlert 检查 CPU 告警
func (a *AlertEngine) checkCPUAlert(serverID uint, config ThresholdConfig) (bool, string) {
	metric, err := a.manager.GetLatestMetrics(serverID)
	if err != nil {
		return false, ""
	}
	
	if a.compareValue(metric.CPUUsage, config.Operator, config.Value) {
		return true, fmt.Sprintf("CPU 使用率 %.2f%% %s %.2f%%", metric.CPUUsage, a.operatorText(config.Operator), config.Value)
	}
	
	return false, ""
}

// checkMemoryAlert 检查内存告警
func (a *AlertEngine) checkMemoryAlert(serverID uint, config ThresholdConfig) (bool, string) {
	metric, err := a.manager.GetLatestMetrics(serverID)
	if err != nil {
		return false, ""
	}
	
	if a.compareValue(metric.MemUsage, config.Operator, config.Value) {
		return true, fmt.Sprintf("内存使用率 %.2f%% %s %.2f%%", metric.MemUsage, a.operatorText(config.Operator), config.Value)
	}
	
	return false, ""
}

// checkDiskAlert 检查硬盘告警
func (a *AlertEngine) checkDiskAlert(serverID uint, config ThresholdConfig) (bool, string) {
	metric, err := a.manager.GetLatestMetrics(serverID)
	if err != nil {
		return false, ""
	}
	
	if a.compareValue(metric.DiskUsage, config.Operator, config.Value) {
		return true, fmt.Sprintf("硬盘使用率 %.2f%% %s %.2f%%", metric.DiskUsage, a.operatorText(config.Operator), config.Value)
	}
	
	return false, ""
}

// checkNetworkAlert 检查网络告警
func (a *AlertEngine) checkNetworkAlert(serverID uint, config ThresholdConfig) (bool, string) {
	metric, err := a.manager.GetLatestMetrics(serverID)
	if err != nil {
		return false, ""
	}
	
	totalTraffic := float64(metric.NetSent + metric.NetRecv)
	if a.compareValue(totalTraffic, config.Operator, config.Value) {
		return true, fmt.Sprintf("网络流量 %.2f bytes/s %s %.2f", totalTraffic, a.operatorText(config.Operator), config.Value)
	}
	
	return false, ""
}

// checkProcessAlert 检查进程告警
func (a *AlertEngine) checkProcessAlert(serverID uint, config ThresholdConfig) (bool, string) {
	metric, err := a.manager.GetLatestMetrics(serverID)
	if err != nil {
		return false, ""
	}
	
	if a.compareValue(float64(metric.ProcessCount), config.Operator, config.Value) {
		return true, fmt.Sprintf("进程数 %d %s %.0f", metric.ProcessCount, a.operatorText(config.Operator), config.Value)
	}
	
	return false, ""
}

// checkOfflineAlert 检查离线告警
func (a *AlertEngine) checkOfflineAlert(serverID uint) (bool, string) {
	online, err := a.manager.GetServerStatus(serverID)
	if err != nil {
		return false, ""
	}
	
	if !online {
		return true, "服务器离线"
	}
	
	return false, ""
}

// compareValue 比较值
func (a *AlertEngine) compareValue(actual float64, operator string, threshold float64) bool {
	switch operator {
	case "gt":
		return actual > threshold
	case "lt":
		return actual < threshold
	case "eq":
		return actual == threshold
	case "gte":
		return actual >= threshold
	case "lte":
		return actual <= threshold
	default:
		return false
	}
}

// operatorText 操作符文本
func (a *AlertEngine) operatorText(operator string) string {
	switch operator {
	case "gt":
		return ">"
	case "lt":
		return "<"
	case "eq":
		return "="
	case "gte":
		return ">="
	case "lte":
		return "<="
	default:
		return operator
	}
}

// handleAlertState 处理告警状态
func (a *AlertEngine) handleAlertState(alert model.MonitorAlert, serverID uint, triggered bool, content string) {
	stateKey := fmt.Sprintf("%d:%d", alert.ID, serverID)
	
	stateVal, _ := a.alertStates.LoadOrStore(stateKey, &AlertState{
		AlertID:  alert.ID,
		ServerID: serverID,
	})
	state := stateVal.(*AlertState)
	
	state.mu.Lock()
	defer state.mu.Unlock()
	
	now := time.Now()
	
	if triggered {
		state.TriggerCount++
		state.LastTriggerTime = now
		
		// 检查是否需要发送告警
		if !state.IsTriggered {
			// 首次触发，创建告警记录
			record := &model.MonitorAlertRecord{
				AlertID:      alert.ID,
				ServerID:     serverID,
				TriggerTime:  now,
				Severity:     alert.Severity,
				AlertContent: content,
			}
			a.db.Create(record)
			state.RecordID = record.ID
			state.IsTriggered = true
			
			// 发送通知
			a.sendNotification(alert, serverID, content)
			state.LastNotifyTime = now
			
			log.Printf("[AlertEngine] 触发告警: %s, 服务器: %d, 内容: %s\n", alert.Name, serverID, content)
		} else {
			// 已触发，检查是否需要重复通知
			if now.Sub(state.LastNotifyTime).Seconds() >= float64(alert.RateLimit) {
				a.sendNotification(alert, serverID, content)
				state.LastNotifyTime = now
			}
		}
	} else {
		// 未触发
		if state.IsTriggered {
			// 告警恢复
			recoverTime := now
			a.db.Model(&model.MonitorAlertRecord{}).
				Where("id = ?", state.RecordID).
				Update("recover_time", recoverTime)
			
			state.IsTriggered = false
			state.TriggerCount = 0
			
			log.Printf("[AlertEngine] 告警恢复: %s, 服务器: %d\n", alert.Name, serverID)
		}
	}
}

// sendNotification 发送通知
func (a *AlertEngine) sendNotification(alert model.MonitorAlert, serverID uint, content string) {
	// 解析通知渠道
	var channelIDs []uint
	if err := json.Unmarshal([]byte(alert.NotifyChannels), &channelIDs); err != nil {
		log.Printf("[AlertEngine] 解析通知渠道失败: %v\n", err)
		return
	}
	
	// 获取服务器信息
	var server model.MonitorServer
	if err := a.db.First(&server, serverID).Error; err != nil {
		return
	}
	
	// 构造通知内容
	message := fmt.Sprintf("[%s] %s\n服务器: %s\n告警内容: %s\n时间: %s",
		alert.Severity, alert.Name, server.Name, content, time.Now().Format("2006-01-02 15:04:05"))
	
	// 发送通知
	for _, channelID := range channelIDs {
		go a.manager.Notification.Send(channelID, alert.Name, message)
	}
}

// TriggerOfflineAlert 触发离线告警
func (a *AlertEngine) TriggerOfflineAlert(serverID uint) {
	var alerts []model.MonitorAlert
	a.db.Where("enable = ? AND alert_type = ?", true, "offline").Find(&alerts)
	
	for _, alert := range alerts {
		a.checkServerAlert(alert, serverID)
	}
}

// ListAlerts 列出告警规则
func (a *AlertEngine) ListAlerts(enable *bool) ([]model.MonitorAlert, error) {
	query := a.db.Model(&model.MonitorAlert{})
	
	if enable != nil {
		query = query.Where("enable = ?", *enable)
	}
	
	var alerts []model.MonitorAlert
	err := query.Order("id ASC").Find(&alerts).Error
	return alerts, err
}

// CreateAlert 创建告警规则
func (a *AlertEngine) CreateAlert(alert *model.MonitorAlert) error {
	return a.db.Create(alert).Error
}

// UpdateAlert 更新告警规则
func (a *AlertEngine) UpdateAlert(alert *model.MonitorAlert) error {
	return a.db.Save(alert).Error
}

// DeleteAlert 删除告警规则
func (a *AlertEngine) DeleteAlert(id uint) error {
	return a.db.Transaction(func(tx *gorm.DB) error {
		// 删除告警规则
		if err := tx.Delete(&model.MonitorAlert{}, id).Error; err != nil {
			return err
		}
		
		// 删除告警记录
		tx.Where("alert_id = ?", id).Delete(&model.MonitorAlertRecord{})
		
		return nil
	})
}

// GetAlertRecords 获取告警记录
func (a *AlertEngine) GetAlertRecords(alertID, serverID uint, start, end time.Time, limit int) ([]model.MonitorAlertRecord, error) {
	query := a.db.Model(&model.MonitorAlertRecord{})
	
	if alertID > 0 {
		query = query.Where("alert_id = ?", alertID)
	}
	
	if serverID > 0 {
		query = query.Where("server_id = ?", serverID)
	}
	
	if !start.IsZero() && !end.IsZero() {
		query = query.Where("trigger_time BETWEEN ? AND ?", start, end)
	}
	
	if limit > 0 {
		query = query.Limit(limit)
	}
	
	var records []model.MonitorAlertRecord
	err := query.Order("trigger_time DESC").Find(&records).Error
	return records, err
}
