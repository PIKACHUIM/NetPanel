package portforward

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

// SOCKS5Proxy 实现一个 SOCKS5 代理服务器。
// 客户端连接本地监听端口后，通过 SOCKS5 协议告知目标地址，
// 代理服务器再将流量转发到该目标地址。
//
// 注意：此处实现的是"SOCKS5 服务器"模式，即本地监听端口作为 SOCKS5 入口，
// 而非将流量转发到另一个 SOCKS5 服务器（那种场景直接用 TCPProxy 即可）。
type SOCKS5Proxy struct {
	listenIP   string
	listenPort int
	maxConns   int64

	listener    net.Listener
	listenerMu  sync.Mutex
	currentConn int64
	trafficIn   int64
	trafficOut  int64
	log         *logrus.Logger
}

func newSOCKS5Proxy(listenIP string, listenPort int, maxConns int64, log *logrus.Logger) *SOCKS5Proxy {
	if maxConns <= 0 {
		maxConns = 256
	}
	return &SOCKS5Proxy{
		listenIP:   listenIP,
		listenPort: listenPort,
		maxConns:   maxConns,
		log:        log,
	}
}

func (p *SOCKS5Proxy) Start() error {
	p.listenerMu.Lock()
	defer p.listenerMu.Unlock()
	if p.listener != nil {
		return nil
	}

	addr := fmt.Sprintf("%s:%d", p.listenIP, p.listenPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("[SOCKS5] 监听 %s 失败: %w", addr, err)
	}
	p.listener = ln
	p.log.Infof("[端口转发][SOCKS5] 开始监听 %s", addr)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				if strings.Contains(err.Error(), "use of closed network connection") {
					break
				}
				p.log.Errorf("[SOCKS5] Accept 错误: %v", err)
				continue
			}
			if atomic.LoadInt64(&p.currentConn) >= p.maxConns {
				p.log.Warnf("[SOCKS5] 超出最大连接数 %d，拒绝连接", p.maxConns)
				conn.Close()
				continue
			}
			go p.handleConn(conn)
		}
	}()
	return nil
}

func (p *SOCKS5Proxy) Stop() {
	p.listenerMu.Lock()
	defer p.listenerMu.Unlock()
	if p.listener != nil {
		p.listener.Close()
		p.listener = nil
		p.log.Infof("[端口转发][SOCKS5] 停止监听 %s:%d", p.listenIP, p.listenPort)
	}
}

func (p *SOCKS5Proxy) GetStatus() string {
	p.listenerMu.Lock()
	defer p.listenerMu.Unlock()
	if p.listener != nil {
		return "running"
	}
	return "stopped"
}

func (p *SOCKS5Proxy) GetTrafficIn() int64  { return atomic.LoadInt64(&p.trafficIn) }
func (p *SOCKS5Proxy) GetTrafficOut() int64 { return atomic.LoadInt64(&p.trafficOut) }

// handleConn 处理单个 SOCKS5 连接
func (p *SOCKS5Proxy) handleConn(conn net.Conn) {
	atomic.AddInt64(&p.currentConn, 1)
	defer func() {
		atomic.AddInt64(&p.currentConn, -1)
		conn.Close()
	}()

	// ---- 阶段1：协商认证方法 ----
	// +----+----------+----------+
	// | VER | NMETHODS | METHODS |
	// +----+----------+----------+
	// |  1  |    1     | 1~255   |
	// +----+----------+----------+
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		p.log.Debugf("[SOCKS5] 读取握手头失败: %v", err)
		return
	}
	if header[0] != 0x05 {
		p.log.Warnf("[SOCKS5] 非 SOCKS5 协议，版本字节: 0x%02x", header[0])
		return
	}
	nMethods := int(header[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	// 回复：无需认证（0x00）
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	// ---- 阶段2：解析请求 ----
	// +----+-----+-------+------+----------+----------+
	// | VER | CMD | RSV | ATYP | DST.ADDR | DST.PORT |
	// +----+-----+-------+------+----------+----------+
	// |  1  |  1  | 0x00 |  1   | Variable |    2     |
	// +----+-----+-------+------+----------+----------+
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(conn, reqHeader); err != nil {
		return
	}
	if reqHeader[0] != 0x05 {
		return
	}
	cmd := reqHeader[1]
	atyp := reqHeader[3]

	var targetAddr string
	switch atyp {
	case 0x01: // IPv4
		ipv4 := make([]byte, 4)
		if _, err := io.ReadFull(conn, ipv4); err != nil {
			return
		}
		targetAddr = net.IP(ipv4).String()
	case 0x03: // 域名
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return
		}
		domain := make([]byte, int(lenBuf[0]))
		if _, err := io.ReadFull(conn, domain); err != nil {
			return
		}
		targetAddr = string(domain)
	case 0x04: // IPv6
		ipv6 := make([]byte, 16)
		if _, err := io.ReadFull(conn, ipv6); err != nil {
			return
		}
		targetAddr = net.IP(ipv6).String()
	default:
		// 不支持的地址类型
		conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck
		return
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return
	}
	targetPort := binary.BigEndian.Uint16(portBuf)
	fullTarget := fmt.Sprintf("%s:%d", targetAddr, targetPort)

	// 目前只支持 CONNECT（TCP 代理）
	if cmd != 0x01 {
		// 不支持 BIND / UDP ASSOCIATE
		conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck
		return
	}

	// ---- 阶段3：连接目标 ----
	dst, err := net.Dial("tcp", fullTarget)
	if err != nil {
		p.log.Errorf("[SOCKS5] 连接目标 %s 失败: %v", fullTarget, err)
		// 回复：连接被拒绝
		conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck
		return
	}
	defer dst.Close()

	// 回复：成功
	// +----+-----+-------+------+----------+----------+
	// | VER | REP | RSV | ATYP |  BND.ADDR | BND.PORT |
	// +----+-----+-------+------+----------+----------+
	conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}) //nolint:errcheck

	p.log.Debugf("[SOCKS5] 建立隧道: %s -> %s", conn.RemoteAddr(), fullTarget)

	// ---- 阶段4：双向透明转发 ----
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		n, _ := io.Copy(dst, conn)
		atomic.AddInt64(&p.trafficIn, n)
	}()
	go func() {
		defer wg.Done()
		n, _ := io.Copy(conn, dst)
		atomic.AddInt64(&p.trafficOut, n)
	}()
	wg.Wait()
}
