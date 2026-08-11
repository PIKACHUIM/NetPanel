package monitor

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// TerminalServer 终端服务
type TerminalServer struct {
	db      *gorm.DB
	manager *Manager
	
	// WebSocket 连接池
	sessions sync.Map // key: session_id, value: *TerminalSession
}

// TerminalSession 终端会话
type TerminalSession struct {
	SessionID string
	ServerID  uint
	UserID    uint
	Conn      *websocket.Conn
	
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.Mutex
}

// NewTerminalServer 创建终端服务
func NewTerminalServer(db *gorm.DB, manager *Manager) *TerminalServer {
	return &TerminalServer{
		db:      db,
		manager: manager,
	}
}

// CreateSession 创建终端会话
func (t *TerminalServer) CreateSession(serverID, userID uint, conn *websocket.Conn) (*TerminalSession, error) {
	// 验证服务器是否存在
	var server model.MonitorServer
	if err := t.db.First(&server, serverID).Error; err != nil {
		return nil, fmt.Errorf("服务器不存在: %w", err)
	}
	
	// 生成会话 ID
	sessionID := fmt.Sprintf("%d-%d-%d", serverID, userID, time.Now().Unix())
	
	ctx, cancel := context.WithCancel(context.Background())
	
	session := &TerminalSession{
		SessionID: sessionID,
		ServerID:  serverID,
		UserID:    userID,
		Conn:      conn,
		ctx:       ctx,
		cancel:    cancel,
	}
	
	t.sessions.Store(sessionID, session)
	
	log.Printf("[Terminal] 创建会话: %s, 服务器: %d, 用户: %d\n", sessionID, serverID, userID)
	
	return session, nil
}

// CloseSession 关闭终端会话
func (t *TerminalServer) CloseSession(sessionID string) {
	if val, ok := t.sessions.Load(sessionID); ok {
		session := val.(*TerminalSession)
		session.cancel()
		session.Conn.Close()
		t.sessions.Delete(sessionID)
		
		log.Printf("[Terminal] 关闭会话: %s\n", sessionID)
	}
}

// SendToSession 向会话发送数据
func (t *TerminalServer) SendToSession(sessionID string, data []byte) error {
	val, ok := t.sessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("会话不存在: %s", sessionID)
	}
	
	session := val.(*TerminalSession)
	session.mu.Lock()
	defer session.mu.Unlock()
	
	return session.Conn.WriteMessage(websocket.BinaryMessage, data)
}

// HandleSession 处理终端会话
func (t *TerminalServer) HandleSession(session *TerminalSession) {
	defer t.CloseSession(session.SessionID)
	
	// 获取服务器信息
	var server model.MonitorServer
	if err := t.db.First(&server, session.ServerID).Error; err != nil {
		log.Printf("[Terminal] 获取服务器信息失败: %v\n", err)
		return
	}
	
	// 根据接入类型处理
	switch server.AccessType {
	case "ssh":
		t.handleSSHSession(session, server)
	case "agent":
		t.handleAgentSession(session, server)
	default:
		log.Printf("[Terminal] 不支持的接入类型: %s\n", server.AccessType)
	}
}

// handleSSHSession 处理 SSH 会话
func (t *TerminalServer) handleSSHSession(session *TerminalSession, server model.MonitorServer) {
	// 创建 SSH 客户端
	client, err := t.manager.Collector.getSSHClient(server)
	if err != nil {
		log.Printf("[Terminal] SSH 连接失败: %v\n", err)
		t.SendToSession(session.SessionID, []byte(fmt.Sprintf("SSH 连接失败: %v\r\n", err)))
		return
	}
	
	// 创建 SSH 会话
	sshSession, err := client.NewSession()
	if err != nil {
		log.Printf("[Terminal] 创建 SSH 会话失败: %v\n", err)
		return
	}
	defer sshSession.Close()
	
	// 设置终端模式
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	
	if err := sshSession.RequestPty("xterm-256color", 40, 80, modes); err != nil {
		log.Printf("[Terminal] 请求 PTY 失败: %v\n", err)
		return
	}
	
	// 连接输入输出
	sshStdin, _ := sshSession.StdinPipe()
	sshStdout, _ := sshSession.StdoutPipe()
	sshStderr, _ := sshSession.StderrPipe()
	
	// 启动 shell
	if err := sshSession.Shell(); err != nil {
		log.Printf("[Terminal] 启动 Shell 失败: %v\n", err)
		return
	}
	
	// WebSocket -> SSH
	go func() {
		for {
			_, data, err := session.Conn.ReadMessage()
			if err != nil {
				return
			}
			sshStdin.Write(data)
		}
	}()
	
	// SSH -> WebSocket
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sshStdout.Read(buf)
			if err != nil {
				return
			}
			t.SendToSession(session.SessionID, buf[:n])
		}
	}()
	
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := sshStderr.Read(buf)
			if err != nil {
				return
			}
			t.SendToSession(session.SessionID, buf[:n])
		}
	}()
	
	// 等待会话结束
	sshSession.Wait()
}

// handleAgentSession 处理 Agent 会话
func (t *TerminalServer) handleAgentSession(session *TerminalSession, server model.MonitorServer) {
	// TODO: 通过 gRPC Terminal 流实现 Agent 终端
	log.Printf("[Terminal] Agent 模式终端暂未实现\n")
	t.SendToSession(session.SessionID, []byte("Agent 模式终端功能开发中...\r\n"))
}
