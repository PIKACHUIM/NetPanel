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
	TriggerCount    int // 连续触发次数
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
	// 探测失败告警：目标为探测配置（probe:N），按（探测×执行服务器）组合评估
	if alert.AlertType == "probe" {
		a.checkProbeAlert(alert)
		return
	}

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
	event := "recover"
	if triggered {
		event = "trigger"
	}
	a.handleAlertState(alert, alertTarget{kind: "server", targetID: serverID}, event, alertContent)
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

// alertTarget 告警对象
// kind=server：阈值/离线告警，targetID 为被监控的服务器 ID
// kind=probe：探测失败告警，targetID 为执行探测的服务器 ID，probeID 为探测配置 ID
type alertTarget struct {
	kind     string
	targetID uint
	probeID  uint
}

// handleAlertState 处理告警状态（去重、静默限流、恢复）
// event: "trigger" 达到触发条件 / "recover" 达到恢复条件 / "" 依据不足，维持现状
func (a *AlertEngine) handleAlertState(alert model.MonitorAlert, obj alertTarget, event string, content string) {
	stateKey := fmt.Sprintf("%d:%s:%d", alert.ID, obj.kind, obj.targetID)
	if obj.kind == "probe" {
		stateKey = fmt.Sprintf("%d:probe:%d:%d", alert.ID, obj.probeID, obj.targetID)
	}

	stateVal, _ := a.alertStates.LoadOrStore(stateKey, &AlertState{
		AlertID:  alert.ID,
		ServerID: obj.targetID,
	})
	state := stateVal.(*AlertState)

	state.mu.Lock()
	defer state.mu.Unlock()

	now := time.Now()

	switch event {
	case "trigger":
		state.TriggerCount++
		state.LastTriggerTime = now

		if !state.IsTriggered {
			// 首次触发，创建告警记录
			record := &model.MonitorAlertRecord{
				AlertID:      alert.ID,
				ServerID:     obj.targetID,
				TriggerTime:  now,
				Severity:     alert.Severity,
				AlertContent: content,
			}
			a.db.Create(record)
			state.RecordID = record.ID
			state.IsTriggered = true

			// 发送通知
			a.sendNotification(alert, obj.targetID, content)
			state.LastNotifyTime = now

			log.Printf("[AlertEngine] 触发告警: %s, 对象: %s:%d, 内容: %s\n", alert.Name, obj.kind, obj.targetID, content)
		} else if now.Sub(state.LastNotifyTime).Seconds() >= float64(alert.RateLimit) {
			// 已触发，静默期后重复通知
			a.sendNotification(alert, obj.targetID, content)
			state.LastNotifyTime = now
		}
	case "recover":
		if state.IsTriggered {
			// 告警恢复
			recoverTime := now
			a.db.Model(&model.MonitorAlertRecord{}).
				Where("id = ?", state.RecordID).
				Update("recover_time", recoverTime)

			state.IsTriggered = false
			state.TriggerCount = 0

			log.Printf("[AlertEngine] 告警恢复: %s, 对象: %s:%d\n", alert.Name, obj.kind, obj.targetID)
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

// TriggerProbeAlert 探测引擎在连续失败达阈值时主动调用，无需等待检查周期
func (a *AlertEngine) TriggerProbeAlert(probe model.MonitorProbe, serverID uint) {
	var alerts []model.MonitorAlert
	a.db.Where("enable = ? AND alert_type = ?", true, "probe").Find(&alerts)

	for _, alert := range alerts {
		matched := false
		for _, id := range a.resolveProbeTargets(alert.TargetServers) {
			if id == probe.ID {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if state, content := a.probeState(probe, serverID); state == "trigger" {
			a.handleAlertState(alert, alertTarget{kind: "probe", targetID: serverID, probeID: probe.ID}, state, content)
		}
	}
}

// checkProbeAlert 检查探测失败告警规则，覆盖规则目标中的每个（探测×执行服务器）组合
func (a *AlertEngine) checkProbeAlert(alert model.MonitorAlert) {
	for _, probeID := range a.resolveProbeTargets(alert.TargetServers) {
		var probe model.MonitorProbe
		if err := a.db.First(&probe, probeID).Error; err != nil {
			log.Printf("[AlertEngine] 探测配置不存在: %d\n", probeID)
			continue
		}
		var serverIDs []uint
		if err := json.Unmarshal([]byte(probe.ServerIDs), &serverIDs); err != nil {
			log.Printf("[AlertEngine] 解析探测执行服务器失败: probe=%d err=%v\n", probeID, err)
			continue
		}
		for _, serverID := range serverIDs {
			state, content := a.probeState(probe, serverID)
			a.handleAlertState(alert, alertTarget{kind: "probe", targetID: serverID, probeID: probeID}, state, content)
		}
	}
}

// resolveProbeTargets 解析 "probe:N" 形式的探测目标
func (a *AlertEngine) resolveProbeTargets(targets string) []uint {
	var raw []string
	if err := json.Unmarshal([]byte(targets), &raw); err != nil {
		log.Printf("[AlertEngine] 解析探测目标失败: %v\n", err)
		return nil
	}

	var probeIDs []uint
	for _, t := range raw {
		var id uint
		if _, err := fmt.Sscanf(t, "probe:%d", &id); err == nil {
			probeIDs = append(probeIDs, id)
		}
	}
	return probeIDs
}

// probeState 评估（探测×执行服务器）当前状态
// 返回 "trigger"（连续失败达 FailThreshold）、"recover"（连续成功达 RecoverThreshold）或 ""（依据不足，维持现状）
func (a *AlertEngine) probeState(probe model.MonitorProbe, serverID uint) (string, string) {
	limit := probe.FailThreshold
	if probe.RecoverThreshold > limit {
		limit = probe.RecoverThreshold
	}
	if limit <= 0 {
		return "", ""
	}

	var recent []model.MonitorProbeResult
	a.db.Where("probe_id = ? AND server_id = ?", probe.ID, serverID).
		Order("timestamp DESC").Limit(limit).Find(&recent)

	// 最近 FailThreshold 次全部失败 → 触发
	if probe.FailThreshold > 0 && len(recent) >= probe.FailThreshold {
		allFailed := true
		for _, r := range recent[:probe.FailThreshold] {
			if r.Success {
				allFailed = false
				break
			}
		}
		if allFailed {
			return "trigger", fmt.Sprintf("探测 %s（%s %s:%d）连续失败 %d 次",
				probe.Name, probe.ProbeType, probe.TargetAddr, probe.TargetPort, probe.FailThreshold)
		}
	}

	// 最近 RecoverThreshold 次全部成功 → 恢复
	recoverN := probe.RecoverThreshold
	if recoverN <= 0 {
		recoverN = 1
	}
	if len(recent) >= recoverN {
		allSuccess := true
		for _, r := range recent[:recoverN] {
			if !r.Success {
				allSuccess = false
				break
			}
		}
		if allSuccess {
			return "recover", ""
		}
	}

	return "", ""
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
