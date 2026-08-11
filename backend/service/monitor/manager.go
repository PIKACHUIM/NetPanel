package monitor

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"gorm.io/gorm"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/netpanel/netpanel/model"
)

// Manager 监控服务管理器
type Manager struct {
	DB          *gorm.DB
	grpcServer  *grpc.Server
	grpcAddr    string
	grpcPort    int
	
	// Agent 连接池
	agentConnections sync.Map // key: server_id, value: *AgentConnection
	
	// 子模块（公开以便 Handler 访问）
	Collector    *Collector
	ProbeEngine  *ProbeEngine
	TaskEngine   *TaskEngine
	AlertEngine  *AlertEngine
	TerminalSrv  *TerminalServer
	Notification *NotificationManager
	
	// 上下文控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// 配置
	heartbeatTimeout time.Duration
	metricsInterval  time.Duration
}

// AgentConnection Agent 连接信息
type AgentConnection struct {
	ServerID      string
	Token         string
	ConnectedAt   time.Time
	LastHeartbeat time.Time
	Stream        interface{} // gRPC 流
	mu            sync.RWMutex
}

// NewManager 创建监控管理器
func NewManager(db *gorm.DB) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	
	m := &Manager{
		DB:               db,
		grpcAddr:         "0.0.0.0",
		grpcPort:         50051,
		ctx:              ctx,
		cancel:           cancel,
		heartbeatTimeout: 60 * time.Second,
		metricsInterval:  30 * time.Second,
	}
	
	// 初始化子模块
	m.Collector = NewCollector(db, m)
	m.ProbeEngine = NewProbeEngine(db, m)
	m.TaskEngine = NewTaskEngine(db, m)
	m.AlertEngine = NewAlertEngine(db, m)
	m.TerminalSrv = NewTerminalServer(db, m)
	m.Notification = NewNotificationManager(db)
	
	return m
}

// Start 启动监控服务
func (m *Manager) Start() error {
	log.Println("[Monitor] 启动监控服务...")
	
	// 启动 gRPC 服务器
	if err := m.startGRPCServer(); err != nil {
		return fmt.Errorf("启动 gRPC 服务失败: %w", err)
	}
	
	// 启动心跳检测
	m.wg.Add(1)
	go m.heartbeatChecker()
	
	// 启动子模块
	m.ProbeEngine.Start()
	m.TaskEngine.Start()
	m.AlertEngine.Start()
	
	log.Println("[Monitor] 监控服务启动成功")
	return nil
}

// Stop 停止监控服务
func (m *Manager) Stop() {
	log.Println("[Monitor] 停止监控服务...")
	
	m.cancel()
	
	// 停止 gRPC 服务器
	if m.grpcServer != nil {
		m.grpcServer.GracefulStop()
	}
	
	// 停止子模块
	m.ProbeEngine.Stop()
	m.TaskEngine.Stop()
	m.AlertEngine.Stop()
	
	m.wg.Wait()
	log.Println("[Monitor] 监控服务已停止")
}

// startGRPCServer 启动 gRPC 服务器
func (m *Manager) startGRPCServer() error {
	addr := fmt.Sprintf("%s:%d", m.grpcAddr, m.grpcPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口失败: %w", err)
	}
	
	// gRPC 服务器配置
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			Time:    30 * time.Second,
			Timeout: 10 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             10 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.MaxRecvMsgSize(10 * 1024 * 1024), // 10MB
		grpc.MaxSendMsgSize(10 * 1024 * 1024),
	}
	
	m.grpcServer = grpc.NewServer(opts...)
	
	// 注册 gRPC 服务（暂时注释，等 proto 编译后再取消）
	// pb.RegisterMonitorAgentServer(m.grpcServer, NewGRPCServer(m))
	
	// 启动 gRPC 服务器
	go func() {
		log.Printf("[Monitor] gRPC 服务器监听在 %s\n", addr)
		if err := m.grpcServer.Serve(lis); err != nil {
			log.Printf("[Monitor] gRPC 服务器错误: %v\n", err)
		}
	}()
	
	return nil
}

// heartbeatChecker 心跳检测器
func (m *Manager) heartbeatChecker() {
	defer m.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkHeartbeats()
		}
	}
}

// checkHeartbeats 检查所有 Agent 心跳
func (m *Manager) checkHeartbeats() {
	now := time.Now()
	
	m.agentConnections.Range(func(key, value interface{}) bool {
		conn := value.(*AgentConnection)
		conn.mu.RLock()
		lastHeartbeat := conn.LastHeartbeat
		conn.mu.RUnlock()
		
		// 检查心跳超时
		if now.Sub(lastHeartbeat) > m.heartbeatTimeout {
			log.Printf("[Monitor] 服务器 %s 心跳超时，标记为离线\n", conn.ServerID)
			m.setServerOffline(conn.ServerID)
			m.agentConnections.Delete(key)
		}
		
		return true
	})
}

// setServerOffline 设置服务器为离线状态
func (m *Manager) setServerOffline(serverID string) {
	var server model.MonitorServer
	if err := m.DB.Where("id = ?", serverID).First(&server).Error; err != nil {
		return
	}
	
	now := time.Now()
	m.DB.Model(&server).Updates(map[string]interface{}{
		"is_online":      false,
		"last_heartbeat": now,
	})
	
	// 触发离线告警
	m.AlertEngine.TriggerOfflineAlert(server.ID)
}

// GetServerStatus 获取服务器状态
func (m *Manager) GetServerStatus(serverID uint) (bool, error) {
	var server model.MonitorServer
	if err := m.DB.First(&server, serverID).Error; err != nil {
		return false, err
	}
	return server.IsOnline, nil
}

// GetLatestMetrics 获取最新监控指标
func (m *Manager) GetLatestMetrics(serverID uint) (*model.MonitorMetric, error) {
	var metric model.MonitorMetric
	err := m.DB.Where("server_id = ?", serverID).
		Order("timestamp DESC").
		First(&metric).Error
	
	if err != nil {
		return nil, err
	}
	return &metric, nil
}

// GetMetricsHistory 获取指标历史数据
func (m *Manager) GetMetricsHistory(serverID uint, start, end time.Time) ([]model.MonitorMetric, error) {
	var metrics []model.MonitorMetric
	err := m.DB.Where("server_id = ? AND timestamp BETWEEN ? AND ?", serverID, start, end).
		Order("timestamp ASC").
		Find(&metrics).Error
	
	return metrics, err
}

// ListServers 列出所有服务器
func (m *Manager) ListServers(enable *bool, groupName string) ([]model.MonitorServer, error) {
	query := m.DB.Model(&model.MonitorServer{})
	
	if enable != nil {
		query = query.Where("enable = ?", *enable)
	}
	
	if groupName != "" {
		query = query.Where("group_name = ?", groupName)
	}
	
	var servers []model.MonitorServer
	err := query.Order("id ASC").Find(&servers).Error
	return servers, err
}

// CreateServer 创建服务器
func (m *Manager) CreateServer(server *model.MonitorServer) error {
	return m.DB.Create(server).Error
}

// UpdateServer 更新服务器
func (m *Manager) UpdateServer(server *model.MonitorServer) error {
	return m.DB.Save(server).Error
}

// DeleteServer 删除服务器
func (m *Manager) DeleteServer(id uint) error {
	return m.DB.Transaction(func(tx *gorm.DB) error {
		// 删除服务器
		if err := tx.Delete(&model.MonitorServer{}, id).Error; err != nil {
			return err
		}
		
		// 删除相关监控数据
		tx.Where("server_id = ?", id).Delete(&model.MonitorMetric{})
		tx.Where("server_id = ?", id).Delete(&model.MonitorProbeResult{})
		tx.Where("server_id = ?", id).Delete(&model.MonitorTaskLog{})
		tx.Where("server_id = ?", id).Delete(&model.MonitorAlertRecord{})
		tx.Where("server_id = ?", id).Delete(&model.MonitorDDNSBinding{})
		tx.Where("server_id = ?", id).Delete(&model.MonitorTunnelBinding{})
		
		return nil
	})
}

// SyncFromMeshNode 从组网节点同步为监控服务器
func (m *Manager) SyncFromMeshNode(nodeID uint) error {
	var node model.MeshNode
	if err := m.DB.First(&node, nodeID).Error; err != nil {
		return err
	}
	
	// 检查是否已存在
	var existingServer model.MonitorServer
	err := m.DB.Where("mesh_node_id = ?", nodeID).First(&existingServer).Error
	
	if err == gorm.ErrRecordNotFound {
		// 创建新的监控服务器
		server := &model.MonitorServer{
			Name:        node.Name,
			DisplayName: node.Name,
			Enable:      node.Enable,
			AccessType:  "agent", // 假设通过 Agent 模式
			MeshNodeID:  node.ID,
			IsOnline:    node.IsOnline,
			Remark:      fmt.Sprintf("从组网节点 %s 同步", node.Name),
		}
		
		return m.DB.Create(server).Error
	}
	
	return err
}

// ExecuteCommandOnServer 在服务器上执行命令
func (m *Manager) ExecuteCommandOnServer(serverID uint, command string, timeout int) (string, error) {
	var server model.MonitorServer
	if err := m.DB.First(&server, serverID).Error; err != nil {
		return "", err
	}
	
	// 根据接入类型执行命令
	switch server.AccessType {
	case "agent":
		return m.executeCommandViaAgent(server, command, timeout)
	case "ssh":
		return m.Collector.executeCommandViaSSH(server, command, timeout)
	default:
		return "", fmt.Errorf("不支持的接入类型: %s", server.AccessType)
	}
}

// executeCommandViaAgent 通过 Agent 执行命令
func (m *Manager) executeCommandViaAgent(server model.MonitorServer, command string, timeout int) (string, error) {
	// 从连接池获取 Agent 连接
	conn, ok := m.agentConnections.Load(fmt.Sprintf("%d", server.ID))
	if !ok {
		return "", fmt.Errorf("服务器未连接")
	}
	
	agentConn := conn.(*AgentConnection)
	_ = agentConn // 后续实现 gRPC 命令调用
	
	// TODO: 通过 gRPC ExecuteCommand 执行命令
	return "", fmt.Errorf("Agent 模式命令执行待实现")
}
