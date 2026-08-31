package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/api/middleware"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/cftunnel"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/monitor"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/wireguard"
)

// MonitorHandler 监控模块 Handler
type MonitorHandler struct {
	manager      *monitor.Manager
	frpMgr       *frp.Manager
	npsMgr       *nps.Manager
	easytierMgr  *easytier.Manager
	cftunnelMgr  *cftunnel.Manager
	wireguardMgr *wireguard.Manager
}

// NewMonitorHandler 创建监控 Handler
func NewMonitorHandler(db *gorm.DB, frpMgr *frp.Manager, npsMgr *nps.Manager, easytierMgr *easytier.Manager, cftunnelMgr *cftunnel.Manager, wireguardMgr *wireguard.Manager) *MonitorHandler {
	return &MonitorHandler{
		manager:      monitor.NewManager(db),
		frpMgr:       frpMgr,
		npsMgr:       npsMgr,
		easytierMgr:  easytierMgr,
		cftunnelMgr:  cftunnelMgr,
		wireguardMgr: wireguardMgr,
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

// terminalOriginAllowed 校验 WebSocket 握手的 Origin 是否与当前服务同源，
// 或在 NETPANEL_ALLOWED_ORIGINS 白名单内，防止跨站 WebSocket 劫持（CSWSH）。
func terminalOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		// 非浏览器客户端（如 CLI）不带 Origin，此时依赖 token 鉴权
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, allowed := range strings.Split(os.Getenv("NETPANEL_ALLOWED_ORIGINS"), ",") {
		if v := strings.TrimSpace(allowed); v != "" && v == origin {
			return true
		}
	}
	return false
}

// HandleTerminal WebSocket 终端处理。
//
// 安全说明：原实现从 query 读取 user_id 且不校验任何凭据，任何人访问
// /ws/terminal?server_id=1 即可获得受管主机的交互式 shell（凭据由服务端从
// 数据库读取），且 CheckOrigin 恒为 true。现改为：
//  1. 握手阶段强制校验 token（query 传入，因浏览器 WebSocket 无法自定义请求头）；
//  2. 用户身份取自令牌声明，不信任客户端传入的 user_id；
//  3. 仅管理员可开启远程终端；
//  4. 校验 Origin 同源。
func (h *MonitorHandler) HandleTerminal(c *gin.Context) {
	// WebSocket 握手失败时不能用 c.JSON（连接尚未升级），统一返回 HTTP 错误码
	token := c.Query("token")
	if token == "" {
		c.String(http.StatusUnauthorized, "缺少 token")
		return
	}
	claims, err := middleware.ParseToken(token)
	if err != nil {
		c.String(http.StatusUnauthorized, "token 无效或已过期")
		return
	}
	if !claims.IsAdmin {
		c.String(http.StatusForbidden, "需要管理员权限")
		return
	}

	serverID, err := strconv.ParseUint(c.Query("server_id"), 10, 32)
	if err != nil || serverID == 0 {
		c.String(http.StatusBadRequest, "server_id 非法")
		return
	}

	upgrader := websocket.Upgrader{CheckOrigin: terminalOriginAllowed}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Upgrade 失败时响应已写入，此处仅记录审计日志
		logger.WriteLog("warn", "monitor", fmt.Sprintf("终端 WebSocket 升级失败: %v", err))
		return
	}

	logger.WriteLog("info", "monitor", fmt.Sprintf("用户 %s 打开服务器 [%d] 远程终端", claims.Username, serverID))

	// 用户身份取自令牌，避免客户端伪造 user_id
	session, err := h.manager.TerminalSrv.CreateSession(uint(serverID), claims.UserID, conn)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("会话创建失败: "+err.Error()))
		conn.Close()
		return
	}

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

	// 根据 tunnel_type 调用对应服务的状态查询
	binding.TunnelStatus = h.queryTunnelStatus(binding.TunnelType, binding.TunnelID)
	h.manager.DB.Save(&binding)

	c.JSON(http.StatusOK, gin.H{"message": "同步成功", "status": binding.TunnelStatus})
}

// queryTunnelStatus 查询隧道状态并映射为 connected/disconnected/unknown
func (h *MonitorHandler) queryTunnelStatus(tunnelType string, tunnelID uint) string {
	var status string
	switch tunnelType {
	case "frp":
		status = h.frpMgr.GetClientStatus(tunnelID)
	case "nps":
		status = h.npsMgr.GetClientStatus(tunnelID)
	case "easytier":
		status = h.easytierMgr.GetClientStatus(tunnelID)
	case "cftunnel":
		// GetStatus 始终返回进程运行状态；配置记录缺失时的 error 不影响状态映射
		if st, _ := h.cftunnelMgr.GetStatus(tunnelID); st != nil {
			if s, ok := st["status"].(string); ok {
				status = s
			}
		}
	case "wireguard":
		status = h.wireguardMgr.GetStatus(tunnelID)
	default:
		return "unknown"
	}

	switch status {
	case "running":
		return "connected"
	case "stopped":
		return "disconnected"
	default:
		return "unknown"
	}
}
