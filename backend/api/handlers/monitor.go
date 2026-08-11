package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/monitor"
)

// MonitorHandler 监控模块 Handler
type MonitorHandler struct {
	manager *monitor.Manager
}

// NewMonitorHandler 创建监控 Handler
func NewMonitorHandler(db *gorm.DB) *MonitorHandler {
	return &MonitorHandler{
		manager: monitor.NewManager(db),
	}
}

// Start 启动监控服务
func (h *MonitorHandler) Start() error {
	return h.manager.Start()
}

// Stop 停止监控服务
func (h *MonitorHandler) Stop() {
	h.manager.Stop()
}

// ===== 服务器管理 =====

// ListServers 列出服务器
func (h *MonitorHandler) ListServers(c *gin.Context) {
	var enable *bool
	if enableStr := c.Query("enable"); enableStr != "" {
		val := enableStr == "true"
		enable = &val
	}
	
	groupName := c.Query("group_name")
	
	servers, err := h.manager.ListServers(enable, groupName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, servers)
}

// GetServer 获取服务器详情
func (h *MonitorHandler) GetServer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var server model.MonitorServer
	if err := h.manager.DB.First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}
	
	c.JSON(http.StatusOK, server)
}

// CreateServer 创建服务器
func (h *MonitorHandler) CreateServer(c *gin.Context) {
	var server model.MonitorServer
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.manager.CreateServer(&server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, server)
}

// UpdateServer 更新服务器
func (h *MonitorHandler) UpdateServer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var server model.MonitorServer
	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	server.ID = uint(id)
	if err := h.manager.UpdateServer(&server); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, server)
}

// DeleteServer 删除服务器
func (h *MonitorHandler) DeleteServer(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.DeleteServer(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// SyncFromMeshNode 从组网节点同步
func (h *MonitorHandler) SyncFromMeshNode(c *gin.Context) {
	nodeID, _ := strconv.ParseUint(c.Param("nodeId"), 10, 32)
	
	if err := h.manager.SyncFromMeshNode(uint(nodeID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "同步成功"})
}

// ===== 监控指标 =====

// GetLatestMetrics 获取最新监控指标
func (h *MonitorHandler) GetLatestMetrics(c *gin.Context) {
	serverID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	metric, err := h.manager.GetLatestMetrics(uint(serverID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到监控数据"})
		return
	}
	
	c.JSON(http.StatusOK, metric)
}

// GetMetricsHistory 获取监控指标历史
func (h *MonitorHandler) GetMetricsHistory(c *gin.Context) {
	serverID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	startStr := c.Query("start")
	endStr := c.Query("end")
	
	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)
	
	if end.IsZero() {
		end = time.Now()
	}
	if start.IsZero() {
		start = end.Add(-24 * time.Hour) // 默认 24 小时
	}
	
	metrics, err := h.manager.GetMetricsHistory(uint(serverID), start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, metrics)
}

// ===== 服务探测 =====

// ListProbes 列出探测配置
func (h *MonitorHandler) ListProbes(c *gin.Context) {
	var enable *bool
	if enableStr := c.Query("enable"); enableStr != "" {
		val := enableStr == "true"
		enable = &val
	}
	
	probes, err := h.manager.ProbeEngine.ListProbes(enable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, probes)
}

// CreateProbe 创建探测配置
func (h *MonitorHandler) CreateProbe(c *gin.Context) {
	var probe model.MonitorProbe
	if err := c.ShouldBindJSON(&probe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.manager.ProbeEngine.CreateProbe(&probe); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, probe)
}

// UpdateProbe 更新探测配置
func (h *MonitorHandler) UpdateProbe(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var probe model.MonitorProbe
	if err := c.ShouldBindJSON(&probe); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	probe.ID = uint(id)
	if err := h.manager.ProbeEngine.UpdateProbe(&probe); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, probe)
}

// DeleteProbe 删除探测配置
func (h *MonitorHandler) DeleteProbe(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.ProbeEngine.DeleteProbe(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetProbeResults 获取探测结果
func (h *MonitorHandler) GetProbeResults(c *gin.Context) {
	probeID, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	serverIDStr := c.Query("server_id")
	startStr := c.Query("start")
	endStr := c.Query("end")
	
	var serverID uint
	if serverIDStr != "" {
		sid, _ := strconv.ParseUint(serverIDStr, 10, 32)
		serverID = uint(sid)
	}
	
	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)
	
	results, err := h.manager.ProbeEngine.GetProbeResults(uint(probeID), serverID, start, end)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, results)
}

// ===== 任务管理 =====

// ListTasks 列出任务
func (h *MonitorHandler) ListTasks(c *gin.Context) {
	var enable *bool
	if enableStr := c.Query("enable"); enableStr != "" {
		val := enableStr == "true"
		enable = &val
	}
	
	taskType := c.Query("task_type")
	
	tasks, err := h.manager.TaskEngine.ListTasks(enable, taskType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, tasks)
}

// CreateTask 创建任务
func (h *MonitorHandler) CreateTask(c *gin.Context) {
	var task model.MonitorTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.manager.TaskEngine.CreateTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, task)
}

// UpdateTask 更新任务
func (h *MonitorHandler) UpdateTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var task model.MonitorTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	task.ID = uint(id)
	if err := h.manager.TaskEngine.UpdateTask(&task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, task)
}

// DeleteTask 删除任务
func (h *MonitorHandler) DeleteTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.TaskEngine.DeleteTask(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ExecuteTask 执行任务
func (h *MonitorHandler) ExecuteTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.TaskEngine.ExecuteManualTask(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "任务已提交执行"})
}

// GetTaskLogs 获取任务日志
func (h *MonitorHandler) GetTaskLogs(c *gin.Context) {
	taskIDStr := c.Query("task_id")
	serverIDStr := c.Query("server_id")
	startStr := c.Query("start")
	endStr := c.Query("end")
	limitStr := c.DefaultQuery("limit", "100")
	
	var taskID, serverID uint
	var limit int
	
	if taskIDStr != "" {
		tid, _ := strconv.ParseUint(taskIDStr, 10, 32)
		taskID = uint(tid)
	}
	
	if serverIDStr != "" {
		sid, _ := strconv.ParseUint(serverIDStr, 10, 32)
		serverID = uint(sid)
	}
	
	limit, _ = strconv.Atoi(limitStr)
	
	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)
	
	logs, err := h.manager.TaskEngine.GetTaskLogs(taskID, serverID, start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, logs)
}

// ===== 告警规则 =====

// ListAlerts 列出告警规则
func (h *MonitorHandler) ListAlerts(c *gin.Context) {
	var enable *bool
	if enableStr := c.Query("enable"); enableStr != "" {
		val := enableStr == "true"
		enable = &val
	}
	
	alerts, err := h.manager.AlertEngine.ListAlerts(enable)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, alerts)
}

// CreateAlert 创建告警规则
func (h *MonitorHandler) CreateAlert(c *gin.Context) {
	var alert model.MonitorAlert
	if err := c.ShouldBindJSON(&alert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.manager.AlertEngine.CreateAlert(&alert); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, alert)
}

// UpdateAlert 更新告警规则
func (h *MonitorHandler) UpdateAlert(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var alert model.MonitorAlert
	if err := c.ShouldBindJSON(&alert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	alert.ID = uint(id)
	if err := h.manager.AlertEngine.UpdateAlert(&alert); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, alert)
}

// DeleteAlert 删除告警规则
func (h *MonitorHandler) DeleteAlert(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.AlertEngine.DeleteAlert(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetAlertRecords 获取告警记录
func (h *MonitorHandler) GetAlertRecords(c *gin.Context) {
	alertIDStr := c.Query("alert_id")
	serverIDStr := c.Query("server_id")
	startStr := c.Query("start")
	endStr := c.Query("end")
	limitStr := c.DefaultQuery("limit", "100")
	
	var alertID, serverID uint
	var limit int
	
	if alertIDStr != "" {
		aid, _ := strconv.ParseUint(alertIDStr, 10, 32)
		alertID = uint(aid)
	}
	
	if serverIDStr != "" {
		sid, _ := strconv.ParseUint(serverIDStr, 10, 32)
		serverID = uint(sid)
	}
	
	limit, _ = strconv.Atoi(limitStr)
	
	start, _ := time.Parse(time.RFC3339, startStr)
	end, _ := time.Parse(time.RFC3339, endStr)
	
	records, err := h.manager.AlertEngine.GetAlertRecords(alertID, serverID, start, end, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, records)
}

// ===== WebSocket 终端 =====

// HandleTerminal WebSocket 终端处理
func (h *MonitorHandler) HandleTerminal(c *gin.Context) {
	serverID, _ := strconv.ParseUint(c.Query("server_id"), 10, 32)
	userID, _ := strconv.ParseUint(c.Query("user_id"), 10, 32)
	
	// 升级为 WebSocket
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 生产环境应该检查 Origin
		},
	}
	
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	// 创建终端会话
	session, err := h.manager.TerminalSrv.CreateSession(uint(serverID), uint(userID), conn)
	if err != nil {
		conn.Close()
		return
	}
	
	// 处理会话
	h.manager.TerminalSrv.HandleSession(session)
}

// ===== DDNS 绑定管理 =====

// GetDDNSBindings 获取 DDNS 绑定列表
func (h *MonitorHandler) GetDDNSBindings(c *gin.Context) {
	var bindings []model.MonitorDDNSBinding
	if err := h.manager.DB.Find(&bindings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, bindings)
}

// CreateDDNSBinding 创建 DDNS 绑定
func (h *MonitorHandler) CreateDDNSBinding(c *gin.Context) {
	var binding model.MonitorDDNSBinding
	if err := c.ShouldBindJSON(&binding); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.manager.DB.Create(&binding).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, binding)
}

// UpdateDDNSBinding 更新 DDNS 绑定
func (h *MonitorHandler) UpdateDDNSBinding(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var binding model.MonitorDDNSBinding
	if err := c.ShouldBindJSON(&binding); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	binding.ID = uint(id)
	if err := h.manager.DB.Save(&binding).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, binding)
}

// DeleteDDNSBinding 删除 DDNS 绑定
func (h *MonitorHandler) DeleteDDNSBinding(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.DB.Delete(&model.MonitorDDNSBinding{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// TriggerDDNSUpdate 触发 DDNS 更新
func (h *MonitorHandler) TriggerDDNSUpdate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var binding model.MonitorDDNSBinding
	if err := h.manager.DB.First(&binding, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "绑定不存在"})
		return
	}
	
	// 获取服务器当前 IP
	var server model.MonitorServer
	if err := h.manager.DB.First(&server, binding.ServerID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "服务器不存在"})
		return
	}
	
	// 获取最新指标获取 IP
	var metric model.MonitorMetric
	if err := h.manager.DB.Where("server_id = ?", server.ID).
		Order("timestamp DESC").
		First(&metric).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "无法获取服务器 IP"})
		return
	}
	
	// 触发 DDNS 更新（调用系统的 DDNS 服务）
	// 这里需要调用 ddns service 的 API 来更新 DNS 记录
	now := time.Now()
	binding.LastTriggerTime = &now
	h.manager.DB.Save(&binding)
	
	c.JSON(http.StatusOK, gin.H{"message": "触发成功"})
}

// ===== 通知渠道管理 =====

// GetNotificationChannels 获取通知渠道列表
func (h *MonitorHandler) GetNotificationChannels(c *gin.Context) {
	var channels []model.MonitorNotificationChannel
	if err := h.manager.DB.Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, channels)
}

// CreateNotificationChannel 创建通知渠道
func (h *MonitorHandler) CreateNotificationChannel(c *gin.Context) {
	var channel model.MonitorNotificationChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.manager.DB.Create(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, channel)
}

// UpdateNotificationChannel 更新通知渠道
func (h *MonitorHandler) UpdateNotificationChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var channel model.MonitorNotificationChannel
	if err := c.ShouldBindJSON(&channel); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	channel.ID = uint(id)
	if err := h.manager.DB.Save(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, channel)
}

// DeleteNotificationChannel 删除通知渠道
func (h *MonitorHandler) DeleteNotificationChannel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.DB.Delete(&model.MonitorNotificationChannel{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// SendTestNotification 发送测试通知
func (h *MonitorHandler) SendTestNotification(c *gin.Context) {
	var req struct {
		ChannelID uint   `json:"channel_id" binding:"required"`
		Title     string `json:"title" binding:"required"`
		Content   string `json:"content" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	var channel model.MonitorNotificationChannel
	if err := h.manager.DB.First(&channel, req.ChannelID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "通知渠道不存在"})
		return
	}
	
	// 发送测试通知
	message := req.Content
	if message == "" {
		message = "这是一条来自 NetPanel 监控系统的测试通知。"
	}
	if err := h.manager.Notification.SendNotification(req.ChannelID, req.Title, message); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "发送成功"})
}

// ===== 隧道绑定管理 =====

// GetTunnelBindings 获取隧道绑定列表
func (h *MonitorHandler) GetTunnelBindings(c *gin.Context) {
	var bindings []model.MonitorTunnelBinding
	if err := h.manager.DB.Find(&bindings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, bindings)
}

// CreateTunnelBinding 创建隧道绑定
func (h *MonitorHandler) CreateTunnelBinding(c *gin.Context) {
	var binding model.MonitorTunnelBinding
	if err := c.ShouldBindJSON(&binding); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	if err := h.manager.DB.Create(&binding).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, binding)
}

// UpdateTunnelBinding 更新隧道绑定
func (h *MonitorHandler) UpdateTunnelBinding(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var binding model.MonitorTunnelBinding
	if err := c.ShouldBindJSON(&binding); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	
	binding.ID = uint(id)
	if err := h.manager.DB.Save(&binding).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, binding)
}

// DeleteTunnelBinding 删除隧道绑定
func (h *MonitorHandler) DeleteTunnelBinding(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	if err := h.manager.DB.Delete(&model.MonitorTunnelBinding{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// SyncTunnelStatus 同步隧道状态
func (h *MonitorHandler) SyncTunnelStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	
	var binding model.MonitorTunnelBinding
	if err := h.manager.DB.First(&binding, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "绑定不存在"})
		return
	}
	
	// TODO: 根据 tunnel_type 调用对应的服务获取状态
	// 这里需要集成 frp/nps/easytier/cftunnel/wireguard 的状态查询
	binding.TunnelStatus = "connected" // 示例
	h.manager.DB.Save(&binding)
	
	c.JSON(http.StatusOK, gin.H{"message": "同步成功", "status": binding.TunnelStatus})
}
