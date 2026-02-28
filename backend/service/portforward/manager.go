package portforward

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== 缓冲池 =====

var bufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024)
		return &buf
	},
}

// ===== 全局限制 =====

var (
	globalTCPMaxConns    int64 = 1024
	globalTCPCurrentConn int64 = 0
)

// ===== Proxy 接口 =====

type Proxy interface {
	Start() error
	Stop()
	GetStatus() string
	GetTrafficIn() int64
	GetTrafficOut() int64
}

// ===== TCP Proxy =====

type TCPProxy struct {
	listenIP   string
	listenPort int
	targetAddr string
	targetPort int
	maxConns   int64

	listener    net.Listener
	listenerMu  sync.Mutex
	currentConn int64
	trafficIn   int64
	trafficOut  int64
	log         *logrus.Logger
}

func newTCPProxy(listenIP, targetAddr string, listenPort, targetPort int, maxConns int64, log *logrus.Logger) *TCPProxy {
	if maxConns <= 0 {
		maxConns = 256
	}
	return &TCPProxy{
		listenIP:   listenIP,
		listenPort: listenPort,
		targetAddr: targetAddr,
		targetPort: targetPort,
		maxConns:   maxConns,
		log:        log,
	}
}

func (p *TCPProxy) Start() error {
	p.listenerMu.Lock()
	defer p.listenerMu.Unlock()
	if p.listener != nil {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", p.listenIP, p.listenPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听 %s 失败: %w", addr, err)
	}
	p.listener = ln
	p.log.Infof("[端口转发][TCP] 开始监听 %s -> %s:%d", addr, p.targetAddr, p.targetPort)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				p.log.Errorf("[TCP] Accept 错误: %v", err)
				continue
			}
			if atomic.LoadInt64(&p.currentConn) >= p.maxConns {
				p.log.Warnf("[TCP] 超出最大连接数 %d，拒绝连接", p.maxConns)
				conn.Close()
				continue
			}
			go p.handleConn(conn)
		}
	}()
	return nil
}

func (p *TCPProxy) Stop() {
	p.listenerMu.Lock()
	defer p.listenerMu.Unlock()
	if p.listener != nil {
		p.listener.Close()
		p.listener = nil
		p.log.Infof("[端口转发][TCP] 停止监听 %s:%d", p.listenIP, p.listenPort)
	}
}

func (p *TCPProxy) handleConn(src net.Conn) {
	atomic.AddInt64(&p.currentConn, 1)
	atomic.AddInt64(&globalTCPCurrentConn, 1)
	defer func() {
		atomic.AddInt64(&p.currentConn, -1)
		atomic.AddInt64(&globalTCPCurrentConn, -1)
		src.Close()
	}()

	dst, err := net.Dial("tcp", fmt.Sprintf("%s:%d", p.targetAddr, p.targetPort))
	if err != nil {
		p.log.Errorf("连接目标[TCP] %s:%d 失败: %v", p.targetAddr, p.targetPort, err)
		return
	}
	defer dst.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		n := p.copyData(dst, src)
		atomic.AddInt64(&p.trafficIn, n)
	}()
	go func() {
		defer wg.Done()
		n := p.copyData(src, dst)
		atomic.AddInt64(&p.trafficOut, n)
	}()
	wg.Wait()
}

func (p *TCPProxy) copyData(dst net.Conn, src net.Conn) int64 {
	bufPtr := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufPtr)
	buf := *bufPtr

	var total int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			if nw > 0 {
				total += int64(nw)
			}
			if ew != nil {
				break
			}
		}
		if er != nil {
			break
		}
	}
	return total
}

func (p *TCPProxy) GetStatus() string {
	p.listenerMu.Lock()
	defer p.listenerMu.Unlock()
	if p.listener != nil {
		return "running"
	}
	return "stopped"
}

func (p *TCPProxy) GetTrafficIn() int64  { return atomic.LoadInt64(&p.trafficIn) }
func (p *TCPProxy) GetTrafficOut() int64 { return atomic.LoadInt64(&p.trafficOut) }

// ===== UDP Proxy =====

type UDPProxy struct {
	listenIP   string
	listenPort int
	targetAddr string
	targetPort int

	conn   *net.UDPConn
	connMu sync.Mutex
	stopCh chan struct{}

	trafficIn  int64
	trafficOut int64
	log        *logrus.Logger
}

func newUDPProxy(listenIP, targetAddr string, listenPort, targetPort int, log *logrus.Logger) *UDPProxy {
	return &UDPProxy{
		listenIP:   listenIP,
		listenPort: listenPort,
		targetAddr: targetAddr,
		targetPort: targetPort,
		log:        log,
	}
}

func (p *UDPProxy) Start() error {
	p.connMu.Lock()
	defer p.connMu.Unlock()
	if p.conn != nil {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", p.listenIP, p.listenPort)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("UDP 监听 %s 失败: %w", addr, err)
	}
	p.conn = conn
	p.stopCh = make(chan struct{})
	p.log.Infof("[端口转发][UDP] 开始监听 %s -> %s:%d", addr, p.targetAddr, p.targetPort)

	go p.serve()
	return nil
}

func (p *UDPProxy) serve() {
	buf := make([]byte, 65507)
	sessions := sync.Map{}

	for {
		n, remoteAddr, err := p.conn.ReadFromUDP(buf)
		if err != nil {
			if strings.Contains(err.Error(), "use of closed network connection") {
				break
			}
			continue
		}

		data := make([]byte, n)
		copy(data, buf[:n])
		atomic.AddInt64(&p.trafficIn, int64(n))

		targetAddrStr := fmt.Sprintf("%s:%d", p.targetAddr, p.targetPort)
		key := remoteAddr.String()

		val, ok := sessions.Load(key)
		var targetConn *net.UDPConn
		if ok {
			targetConn = val.(*net.UDPConn)
		} else {
			tAddr, err := net.ResolveUDPAddr("udp", targetAddrStr)
			if err != nil {
				continue
			}
			targetConn, err = net.DialUDP("udp", nil, tAddr)
			if err != nil {
				continue
			}
			sessions.Store(key, targetConn)
			go func(rc *net.UDPAddr, tc *net.UDPConn) {
				rbuf := make([]byte, 65507)
				for {
					rn, _, rerr := tc.ReadFromUDP(rbuf)
					if rerr != nil {
						break
					}
					p.conn.WriteToUDP(rbuf[:rn], rc)
					atomic.AddInt64(&p.trafficOut, int64(rn))
				}
				sessions.Delete(rc.String())
			}(remoteAddr, targetConn)
		}
		targetConn.Write(data)
	}
}

func (p *UDPProxy) Stop() {
	p.connMu.Lock()
	defer p.connMu.Unlock()
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
		p.log.Infof("[端口转发][UDP] 停止监听 %s:%d", p.listenIP, p.listenPort)
	}
}

func (p *UDPProxy) GetStatus() string {
	p.connMu.Lock()
	defer p.connMu.Unlock()
	if p.conn != nil {
		return "running"
	}
	return "stopped"
}

func (p *UDPProxy) GetTrafficIn() int64  { return atomic.LoadInt64(&p.trafficIn) }
func (p *UDPProxy) GetTrafficOut() int64 { return atomic.LoadInt64(&p.trafficOut) }

// ===== Manager =====

type ruleEntry struct {
	proxies []Proxy
	logs    []string
	logsMu  sync.Mutex
}

// Manager 端口转发管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	entries sync.Map // map[uint]*ruleEntry
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// StartAll 启动所有已启用的规则
func (m *Manager) StartAll() {
	var rules []model.PortForwardRule
	m.db.Where("enable = ?", true).Find(&rules)
	for _, rule := range rules {
		if err := m.Start(rule.ID); err != nil {
			m.log.Errorf("端口转发 [%s] 启动失败: %v", rule.Name, err)
		}
	}
}

// StopAll 停止所有规则
func (m *Manager) StopAll() {
	m.entries.Range(func(key, value interface{}) bool {
		entry := value.(*ruleEntry)
		for _, p := range entry.proxies {
			p.Stop()
		}
		return true
	})
}

// Start 启动指定规则
func (m *Manager) Start(id uint) error {
	// 先停止旧的
	m.Stop(id)

	var rule model.PortForwardRule
	if err := m.db.First(&rule, id).Error; err != nil {
		return fmt.Errorf("规则不存在: %w", err)
	}

	entry := &ruleEntry{}

	// 根据 ListenPortType 决定使用哪种代理
	var p Proxy
	switch strings.ToLower(rule.ListenPortType) {
	case "udp":
		p = newUDPProxy(rule.ListenIP, rule.TargetAddress, rule.ListenPort, rule.TargetPort, m.log)
	case "http", "websocket":
		// HTTP 和 WebSocket 统一用反向代理，ReverseProxy 自动处理 Upgrade
		p = newHTTPProxy(rule.ListenIP, rule.TargetAddress, rule.ListenPort, rule.TargetPort, "http", "", "", m.log)
	case "https":
		// HTTPS：本地监听端口做 TLS 终止，转发到目标
		// 默认转发到 http://，如果目标端口类型也是 https 则转发到 https://
		targetScheme := "http"
		if strings.ToLower(rule.TargetPortType) == "https" {
			targetScheme = "https"
		}
		var certFile, keyFile string
		if rule.DomainCertID > 0 {
			var dc model.DomainCert
			if err := m.db.First(&dc, rule.DomainCertID).Error; err != nil {
				return fmt.Errorf("HTTPS 监听要求证书 ID=%d，但查询失败: %w", rule.DomainCertID, err)
			}
			if dc.CertFile == "" || dc.KeyFile == "" {
				return fmt.Errorf("HTTPS 监听要求证书 ID=%d，但证书文件路径为空（证书可能尚未签发）", rule.DomainCertID)
			}
			certFile = dc.CertFile
			keyFile = dc.KeyFile
		}
		p = newHTTPProxy(rule.ListenIP, rule.TargetAddress, rule.ListenPort, rule.TargetPort, targetScheme, certFile, keyFile, m.log)
	case "socks", "socks5":
		// SOCKS5 代理服务器：本地监听端口作为 SOCKS5 入口
		p = newSOCKS5Proxy(rule.ListenIP, rule.ListenPort, rule.MaxConnections, m.log)
	default:
		// tcp 及其他未知类型均走透明 TCP 转发
		p = newTCPProxy(rule.ListenIP, rule.TargetAddress, rule.ListenPort, rule.TargetPort, rule.MaxConnections, m.log)
	}
	if err := p.Start(); err != nil {
		m.db.Model(&model.PortForwardRule{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return err
	}
	entry.proxies = append(entry.proxies, p)

	m.entries.Store(id, entry)
	m.db.Model(&model.PortForwardRule{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[端口转发] 规则 [%d] 已启动", id)
	return nil
}

// Stop 停止指定规则
func (m *Manager) Stop(id uint) {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*ruleEntry)
		for _, p := range entry.proxies {
			p.Stop()
		}
		m.entries.Delete(id)
	}
	m.db.Model(&model.PortForwardRule{}).Where("id = ?", id).Update("status", "stopped")
}

// GetStatus 获取运行状态
func (m *Manager) GetStatus(id uint) string {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*ruleEntry)
		if len(entry.proxies) > 0 {
			return entry.proxies[0].GetStatus()
		}
	}
	return "stopped"
}

// GetLogs 获取日志
func (m *Manager) GetLogs(id uint) []string {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*ruleEntry)
		entry.logsMu.Lock()
		defer entry.logsMu.Unlock()
		result := make([]string, len(entry.logs))
		copy(result, entry.logs)
		return result
	}
	return nil
}
