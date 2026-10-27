package stun

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func testLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// fakeSTUNServer 回环上模拟 STUN 服务器：对 Binding Request 回应携带
// 请求方真实地址的 Binding Response（XOR-MAPPED-ADDRESS）
func fakeSTUNServer(t *testing.T) *net.UDPAddr {
	t.Helper()
	pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("启动假 STUN 服务器失败: %v", err)
	}
	go func() {
		buf := make([]byte, 1500)
		for {
			n, src, err := pc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			if n < 20 {
				continue
			}
			resp := buildBindingResponse(buf[8:20], src)
			_, _ = pc.WriteToUDP(resp, src)
		}
	}()
	t.Cleanup(func() { pc.Close() })
	return pc.LocalAddr().(*net.UDPAddr)
}

// buildBindingResponse 构造带 XOR-MAPPED-ADDRESS 的成功 Binding Response
func buildBindingResponse(tid []byte, src *net.UDPAddr) []byte {
	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], msgTypeBindingResponse)
	binary.BigEndian.PutUint32(msg[4:8], stunMagicCookie)
	copy(msg[8:20], tid)

	// XOR-MAPPED-ADDRESS 属性：type(2) + len(2) + value(8)
	attr := make([]byte, 12)
	binary.BigEndian.PutUint16(attr[0:2], attrXORMappedAddress)
	binary.BigEndian.PutUint16(attr[2:4], 8)
	attr[4] = 0x00
	attr[5] = 0x01 // family IPv4
	binary.BigEndian.PutUint16(attr[6:8], uint16(src.Port)^uint16(stunMagicCookie>>16))
	ip := src.IP.To4()
	attr[8] = ip[0] ^ 0x21
	attr[9] = ip[1] ^ 0x12
	attr[10] = ip[2] ^ 0xA4
	attr[11] = ip[3] ^ 0x42
	msg = append(msg, attr...)
	binary.BigEndian.PutUint16(msg[2:4], uint16(len(msg)-20))
	return msg
}

// fakeUDPEcho 回环目标：收到的数据原样写回发送方
func fakeUDPEcho(t *testing.T) *net.UDPAddr {
	t.Helper()
	pc, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("启动假目标失败: %v", err)
	}
	go func() {
		buf := make([]byte, 2048)
		for {
			n, src, err := pc.ReadFromUDP(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteToUDP(buf[:n], src)
		}
	}()
	t.Cleanup(func() { pc.Close() })
	return pc.LocalAddr().(*net.UDPAddr)
}

func TestUDPForwardKeepaliveAndRelay(t *testing.T) {
	stunAddr := fakeSTUNServer(t)
	targetAddr := fakeUDPEcho(t)

	f, err := newUDPForward(0, stunAddr, targetAddr, NATTypeFullCone, testLogger())
	if err != nil {
		t.Fatalf("创建 UDP 转发器失败: %v", err)
	}
	defer f.Close()
	// 测试目标在回环上，需放行本机地址拦截（生产默认拒绝）
	f.allowLoopback = true

	// 回环无 NAT：STUN 返回的映射地址应等于本 socket 地址
	info, err := f.Keepalive()
	if err != nil {
		t.Fatalf("Keepalive 失败: %v", err)
	}
	if info.IP != "127.0.0.1" || info.Port != f.LocalPort() {
		t.Errorf("映射地址错误: got %s:%d, want 127.0.0.1:%d", info.IP, info.Port, f.LocalPort())
	}
	if info.NATType != NATTypeFullCone {
		t.Errorf("NAT 类型错误: got %s", info.NATType)
	}

	// 转发：外部 peer 经映射入口发包 -> 目标收到；目标响应 -> peer 收到
	peer, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()

	entry := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: f.LocalPort()}
	if _, err := peer.WriteToUDP([]byte("ping"), entry); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 2048)
	if err := peer.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	n, _, err := peer.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("等待目标响应超时: %v", err)
	}
	if string(buf[:n]) != "ping" {
		t.Errorf("转发链路数据错误: got %q", string(buf[:n]))
	}
}

func TestUDPForwardKeepaliveTimeout(t *testing.T) {
	// 不响应的 STUN 服务器地址（回环黑洞端口由 ListenUDP 占用但不响应）
	blackhole, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer blackhole.Close()

	f, err := newUDPForward(0, blackhole.LocalAddr().(*net.UDPAddr), nil, NATTypeUnknown, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := f.Keepalive(); err == nil {
		t.Error("无响应时 Keepalive 应超时报错")
	}
}

func TestUDPForwardKeepaliveIgnoresStaleResponse(t *testing.T) {
	stunAddr := fakeSTUNServer(t)
	f, err := newUDPForward(0, stunAddr, nil, NATTypeUnknown, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// 注入一条与本次事务无关的旧响应，Keepalive 应跳过它并等到真响应
	stale := buildBindingResponse(make([]byte, 12), &net.UDPAddr{IP: net.IPv4(9, 9, 9, 9), Port: 1})
	select {
	case f.respCh <- stale:
	default:
		t.Fatal("注入旧响应失败")
	}

	info, err := f.Keepalive()
	if err != nil {
		t.Fatalf("Keepalive 应忽略旧响应并成功: %v", err)
	}
	if info.Port != f.LocalPort() {
		t.Errorf("应等到本次事务的响应: got port %d, want %d", info.Port, f.LocalPort())
	}
}

// fakeTCPSTUNServer 回环上模拟支持 TCP 的 STUN 服务器
func fakeTCPSTUNServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动假 TCP STUN 服务器失败: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 20)
				if _, err := io.ReadFull(c, buf); err != nil {
					return
				}
				resp := buildBindingResponse(buf[8:20], &net.UDPAddr{IP: net.IPv4(9, 9, 9, 9), Port: 44444})
				_, _ = c.Write(resp)
			}(conn)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return ln.Addr().String()
}

func TestTCPForwardKeepaliveAndRelay(t *testing.T) {
	stunAddr := fakeTCPSTUNServer(t)

	// 目标：TCP echo
	targetLn, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer targetLn.Close()
	go func() {
		for {
			conn, err := targetLn.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(conn)
		}
	}()

	f, err := newTCPForward(t.Context(), 0, targetLn.Addr().String(), stunAddr, NATTypeUnknown, testLogger())
	if err != nil {
		t.Fatalf("创建 TCP 转发器失败: %v", err)
	}
	defer f.Close()

	// 保活：绑定监听端口拨号（SO_REUSEPORT），应拿到服务器回显的映射地址
	info, err := f.Keepalive()
	if err != nil {
		t.Fatalf("TCP Keepalive 失败: %v", err)
	}
	if info == nil {
		t.Fatal("TCP Keepalive 应返回映射地址（测试服务器支持 STUN-over-TCP）")
	}
	if info.IP != "9.9.9.9" || info.Port != 44444 {
		t.Errorf("映射地址错误: got %s:%d", info.IP, info.Port)
	}

	// 转发：连接本地监听端口 -> 数据经转发到 echo 目标原样返回
	conn, err := net.DialTimeout("tcp4", f.ln.Addr().String(), 3*time.Second)
	if err != nil {
		t.Fatalf("连接转发入口失败: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 5)
	if err := conn.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("等待 echo 响应超时: %v", err)
	}
	if string(buf) != "hello" {
		t.Errorf("转发链路数据错误: got %q", string(buf))
	}
}
