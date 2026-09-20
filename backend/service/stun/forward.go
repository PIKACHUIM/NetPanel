// proxy 模式转发实现。
//
// STUN 穿透（代理模式）的本质：用同一个 socket 周期性向 STUN 服务器发包
// 维持 NAT 映射（外部 IP:Port -> 本地 socket），外部访问者发往该映射地址的
// 流量会到达本 socket，由面板转发给真正的目标服务。
//
// 此前的实现每轮检测都新建临时 socket：NAT 外部端口逐轮漂移、打好的
// "洞"从不被业务流量使用。这里改为持久转发器：
//   - udpForward：keepalive 与业务流量共用同一 UDP socket，单一读取者按
//     来源分流（STUN 响应 → 保活；其它来源 → 转发目标）；
//   - tcpForward：本地 TCP 监听 + SO_REUSEPORT 保活拨号（映射是否可用
//     取决于 NAT 对 TCP 的端点无关映射支持）。
package stun

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	// keepaliveTimeout 单次保活等待 STUN 响应的超时
	keepaliveTimeout = 3 * time.Second
	// udpSessionIdleTimeout UDP 会话空闲回收时间
	udpSessionIdleTimeout = 5 * time.Minute
	// udpSessionSweepInterval 会话回收扫描间隔
	udpSessionSweepInterval = time.Minute
	// udpMaxSessions 单个转发器的并发 peer 会话上限。
	// 超限时新来源会被丢弃（而非无限制 DialUDP），避免 UDP 洪泛
	// 无限创建 socket + goroutine。
	udpMaxSessions = 1024
	// udpRateLimitPerPeer 每个来源每秒允许的包数上限。
	// 用于抑制伪造源地址的 UDP 洪泛。
	udpRateLimitPerPeer = 100
	// udpRateLimitWindow 速率限制的滑动窗口粒度
	udpRateLimitWindow = time.Second
)

// forwarder proxy 模式的持久转发器（UDP/TCP）
type forwarder interface {
	// Keepalive 发送一轮保活并返回最新映射地址。地址未知时返回
	// (nil, nil)（映射已维持但无法获取地址，如 STUN-over-TCP 不被支持）。
	Keepalive() (*NATInfo, error)
	LocalPort() int
	Close()
}

// ===== UDP 转发 =====

// udpForward UDP 保活 + 转发。target 为 nil 时仅保活探测（不转发）。
type udpForward struct {
	conn     *net.UDPConn
	stunAddr *net.UDPAddr
	target   *net.UDPAddr
	natType  NATType

	respCh chan []byte // 保活协程消费的 STUN 响应（按事务 ID 匹配）

	mu             sync.Mutex
	sessions       map[string]*net.UDPConn // 远端 peer -> 连接目标的 socket
	lastSeen       map[string]time.Time
	peerPacketCnt  map[string]int          // 速率限制：peer -> 当前窗口包数
	peerWindowStart map[string]time.Time   // 速率限制窗口起始时间

	closeOnce sync.Once
	log       *logrus.Logger
}

// newUDPForward 创建 UDP 转发器。listenPort 为 0 时由系统分配本地端口。
func newUDPForward(listenPort int, stunAddr, target *net.UDPAddr, natType NATType, log *logrus.Logger) (*udpForward, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: listenPort})
	if err != nil {
		return nil, fmt.Errorf("绑定本地 UDP 端口 %d 失败: %w", listenPort, err)
	}
	f := &udpForward{
		conn:            conn,
		stunAddr:        stunAddr,
		target:          target,
		natType:         natType,
		respCh:          make(chan []byte, 16),
		sessions:        make(map[string]*net.UDPConn),
		lastSeen:        make(map[string]time.Time),
		peerPacketCnt:   make(map[string]int),
		peerWindowStart: make(map[string]time.Time),
		log:             log,
	}
	go f.readLoop()
	return f, nil
}

func (f *udpForward) LocalPort() int {
	if addr, ok := f.conn.LocalAddr().(*net.UDPAddr); ok {
		return addr.Port
	}
	return 0
}

// startSweeper 启动空闲会话回收（ctx 取消时停止）
func (f *udpForward) startSweeper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(udpSessionSweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				f.sweepSessions()
			}
		}
	}()
}

// readLoop 单一读取者：STUN 响应交给保活，其余来源转发到目标
func (f *udpForward) readLoop() {
	buf := make([]byte, 65535)
	for {
		n, src, err := f.conn.ReadFromUDP(buf)
		if err != nil {
			return // socket 已关闭
		}
		if f.stunAddr != nil && src.IP.Equal(f.stunAddr.IP) && src.Port == f.stunAddr.Port &&
			n >= 20 && binary.BigEndian.Uint16(buf[0:2]) == msgTypeBindingResponse {
			pkt := make([]byte, n)
			copy(pkt, buf[:n])
			select {
			case f.respCh <- pkt:
			default: // 队列满丢弃，保活超时后会重试
			}
			continue
		}
		if f.target != nil {
			f.relayToTarget(src, buf[:n])
		}
	}
}

// relayToTarget 把外部 peer 的包发往目标（响应由 targetReadLoop 写回 peer）
func (f *udpForward) relayToTarget(peer *net.UDPAddr, pkt []byte) {
	tc := f.sessionFor(peer)
	if tc == nil {
		return
	}
	if _, err := tc.Write(pkt); err != nil {
		f.log.Debugf("[STUN服务][UDP转发] 写入目标失败: %v", err)
	}
}

// sessionFor 取/建 peer 对应的目标 socket（每 peer 一条，响应写回原 peer）
func (f *udpForward) sessionFor(peer *net.UDPAddr) *net.UDPConn {
	key := peer.String()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSeen[key] = time.Now()

	if c, ok := f.sessions[key]; ok {
		return c
	}

	// 会话总数上限：防止 UDP 洪泛无限制创建 socket + goroutine
	if len(f.sessions) >= udpMaxSessions {
		f.log.Warnf("[STUN服务][UDP转发] 会话数达到上限 %d，丢弃新来源 %s", udpMaxSessions, key)
		return nil
	}

	// 每来源速率限制：抑制伪造源地址的 UDP 洪泛
	now := time.Now()
	if ws, ok := f.peerWindowStart[key]; ok && now.Sub(ws) < udpRateLimitWindow {
		if f.peerPacketCnt[key] >= udpRateLimitPerPeer {
			f.log.Debugf("[STUN服务][UDP转发] 来源 %s 超过速率限制（%d/s），丢包", key, udpRateLimitPerPeer)
			return nil
		}
		f.peerPacketCnt[key]++
	} else {
		f.peerWindowStart[key] = now
		f.peerPacketCnt[key] = 1
	}

	// 本机/回环地址阻断：避免把 127.0.0.1:2019（Caddy Admin API 无鉴权）或
	// 127.0.0.1:18090（MCP 强制回环）等"仅本机可访问"的服务直接放到公网。
	// 用户配置目标地址时已有表单提示，此处再加一道防线（目标端口转发到本机回环
	// 地址是最危险的误用场景，如 target=127.0.0.1:2019）。
	if f.target != nil && (f.target.IP.IsLoopback() || f.target.IP.IsUnspecified()) {
		f.log.Warnf("[STUN服务][UDP转发] 拒绝转发到本机地址 %s（安全风险：公网可直达本机敏感服务）", f.target)
		return nil
	}

	c, err := net.DialUDP("udp4", nil, f.target)
	if err != nil {
		f.log.Warnf("[STUN服务][UDP转发] 连接目标 %s 失败: %v", f.target, err)
		return nil
	}
	f.sessions[key] = c
	go f.targetReadLoop(c, peer)
	return c
}

// targetReadLoop 把目标响应写回来源 peer；出错时回收会话
func (f *udpForward) targetReadLoop(c *net.UDPConn, peer *net.UDPAddr) {
	buf := make([]byte, 65535)
	for {
		n, err := c.Read(buf)
		if err != nil {
			f.removeSession(peer.String())
			return
		}
		if _, err := f.conn.WriteToUDP(buf[:n], peer); err != nil {
			f.removeSession(peer.String())
			return
		}
	}
}

func (f *udpForward) removeSession(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if c, ok := f.sessions[key]; ok {
		c.Close()
		delete(f.sessions, key)
	}
	delete(f.lastSeen, key)
}

// sweepSessions 回收空闲超时的会话，避免 peer 表无限增长
func (f *udpForward) sweepSessions() {
	f.mu.Lock()
	defer f.mu.Unlock()
	now := time.Now()
	for key, c := range f.sessions {
		if now.Sub(f.lastSeen[key]) > udpSessionIdleTimeout {
			c.Close()
			delete(f.sessions, key)
			delete(f.lastSeen, key)
		}
	}
}

// Keepalive 发送 Binding Request 维持 NAT 映射并返回映射地址。
// 响应按事务 ID 匹配，迟到的旧响应会被跳过。
func (f *udpForward) Keepalive() (*NATInfo, error) {
	req := buildBindingRequest(false, false)
	tid := req[8:20]
	if _, err := f.conn.WriteToUDP(req, f.stunAddr); err != nil {
		return nil, fmt.Errorf("发送 STUN 保活失败: %w", err)
	}
	deadline := time.Now().Add(keepaliveTimeout)
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			return nil, fmt.Errorf("STUN 响应超时")
		}
		select {
		case pkt := <-f.respCh:
			if len(pkt) < 20 || !bytes.Equal(pkt[8:20], tid) {
				continue // 迟到的旧响应，继续等待本次事务的响应
			}
			msg, err := parseSTUNResponse(pkt)
			if err != nil {
				return nil, fmt.Errorf("解析 STUN 响应失败: %w", err)
			}
			ip, port, err := getMappedAddress(msg)
			if err != nil {
				return nil, fmt.Errorf("解析映射地址失败: %w", err)
			}
			return &NATInfo{IP: ip, Port: port, NATType: f.natType}, nil
		case <-time.After(remain):
			return nil, fmt.Errorf("STUN 响应超时")
		}
	}
}

func (f *udpForward) Close() {
	f.closeOnce.Do(func() {
		// 关闭主 socket 会让 readLoop 退出（ReadFromUDP 返回错误）
		f.conn.Close()

		// 同时关闭所有 peer 的目标 socket，释放 fd 并让 targetReadLoop 退出。
		// 此前只关主 socket，sessions 里的每个 *net.UDPConn 与
		// targetReadLoop 都不会退出——每次 Stop/Start（包括保存配置触发的
		// 重建）都会泄漏 N 个 fd + N 个 goroutine（N = 该轮出现过的 peer 数）。
		f.mu.Lock()
		for key, c := range f.sessions {
			c.Close()
			delete(f.sessions, key)
			delete(f.lastSeen, key)
			delete(f.peerPacketCnt, key)
			delete(f.peerWindowStart, key)
		}
		f.mu.Unlock()
	})
}

// ===== TCP 转发 =====

// tcpForward TCP 保活 + 转发：本地监听接受经 NAT 映射进入的连接并转发目标。
// 保活通过绑定监听端口（SO_REUSEPORT）向 STUN 服务器拨号维持映射；
// 前提是 NAT 对 TCP 具备端点无关映射（EIM），否则映射不可达，建议改用
// UDP 或 direct（UPnP）模式。
type tcpForward struct {
	ln       net.Listener
	stunAddr string
	natType  NATType
	log      *logrus.Logger
}

// newTCPForward 创建 TCP 转发器
func newTCPForward(ctx context.Context, listenPort int, target, stunAddr string, natType NATType, log *logrus.Logger) (*tcpForward, error) {
	lc := net.ListenConfig{Control: controlReusePort}
	ln, err := lc.Listen(ctx, "tcp4", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		return nil, fmt.Errorf("监听本地 TCP 端口 %d 失败: %w", listenPort, err)
	}
	f := &tcpForward{ln: ln, stunAddr: stunAddr, natType: natType, log: log}
	go f.acceptLoop(target)
	log.Infof("[STUN服务][TCP转发] 监听 :%d -> %s（映射可用性取决于 NAT 对 TCP 的支持）", f.LocalPort(), target)
	return f, nil
}

func (f *tcpForward) LocalPort() int {
	if addr, ok := f.ln.Addr().(*net.TCPAddr); ok {
		return addr.Port
	}
	return 0
}

func (f *tcpForward) acceptLoop(target string) {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return // 监听已关闭
		}
		go f.handle(conn, target)
	}
}

// handle 单连接转发：双向拷贝，任一方关闭时结束
func (f *tcpForward) handle(conn net.Conn, target string) {
	defer conn.Close()
	upstream, err := net.DialTimeout("tcp4", target, 5*time.Second)
	if err != nil {
		f.log.Warnf("[STUN服务][TCP转发] 连接目标 %s 失败: %v", target, err)
		return
	}
	defer upstream.Close()

	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(upstream, conn)
		if tc, ok := upstream.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(conn, upstream)
		if tc, ok := conn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
}

// Keepalive 从监听端口向 STUN 服务器发起 TCP 连接维持映射，并尝试
// STUN-over-TCP 获取映射地址（服务器不支持时仅维持映射，返回 nil 地址）。
func (f *tcpForward) Keepalive() (*NATInfo, error) {
	d := net.Dialer{
		Timeout:   keepaliveTimeout,
		LocalAddr: f.ln.Addr(),
		Control:   controlReusePort,
	}
	conn, err := d.Dial("tcp4", f.stunAddr)
	if err != nil {
		return nil, fmt.Errorf("TCP 保活连接失败: %w", err)
	}
	defer conn.Close()

	req := buildBindingRequest(false, false)
	tid := append([]byte(nil), req[8:20]...)
	_ = conn.SetDeadline(time.Now().Add(keepaliveTimeout))
	if _, err := conn.Write(req); err != nil {
		return nil, nil // 映射已由拨号维持
	}
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil || n < 20 || !bytes.Equal(buf[8:20], tid) {
		return nil, nil // 服务器不支持 STUN-over-TCP，映射已维持
	}
	msg, err := parseSTUNResponse(buf[:n])
	if err != nil || msg.msgType != msgTypeBindingResponse {
		return nil, nil
	}
	ip, port, err := getMappedAddress(msg)
	if err != nil {
		return nil, nil
	}
	return &NATInfo{IP: ip, Port: port, NATType: f.natType}, nil
}

func (f *tcpForward) Close() {
	_ = f.ln.Close()
}
