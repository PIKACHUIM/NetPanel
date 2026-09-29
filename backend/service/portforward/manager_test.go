package portforward

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func newTestLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// startEchoServer 启动一个回显 TCP 服务（测试辅助）
func startEchoServer(t *testing.T) (addr string, stop func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("启动回显服务失败: %v", err)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// 回显一次后关闭，让代理 dst→src 方向读到 EOF 并结算流量
				buf := make([]byte, 512)
				n, _ := c.Read(buf)
				c.Write(buf[:n])
			}(conn)
		}
	}()
	return ln.Addr().String(), func() { ln.Close() }
}

func TestNewTCPProxyDefaultMaxConns(t *testing.T) {
	p := newTCPProxy("127.0.0.1", "127.0.0.1", 0, 0, 0, newTestLogger())
	if p.maxConns != 256 {
		t.Errorf("maxConns<=0 时应默认 256，实际 %d", p.maxConns)
	}
	p = newTCPProxy("127.0.0.1", "127.0.0.1", 0, 0, 10, newTestLogger())
	if p.maxConns != 10 {
		t.Errorf("maxConns 应保留传入值 10，实际 %d", p.maxConns)
	}
}

func TestTCPProxyForwarding(t *testing.T) {
	target, stopTarget := startEchoServer(t)
	defer stopTarget()

	thost, tport, _ := net.SplitHostPort(target)
	tportInt := atoi(tport)

	p := newTCPProxy("127.0.0.1", thost, 0, tportInt, 16, newTestLogger())
	// 监听端口为 0 时由内核分配，但 newTCPProxy 固定使用传入值，
	// 因此这里改用随机可用端口探测
	listenPort := freePort(t)
	p = newTCPProxy("127.0.0.1", thost, listenPort, tportInt, 16, newTestLogger())

	if err := p.Start(); err != nil {
		t.Fatalf("启动代理失败: %v", err)
	}
	if p.GetStatus() != "running" {
		t.Errorf("启动后状态应为 running，实际 %q", p.GetStatus())
	}

	// 二次 Start 应幂等
	if err := p.Start(); err != nil {
		t.Errorf("重复 Start 应幂等，实际报错: %v", err)
	}

	// 等监听就绪
	var conn net.Conn
	var dialErr error
	for i := 0; i < 50; i++ {
		conn, dialErr = net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(listenPort)), time.Second)
		if dialErr == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if dialErr != nil {
		t.Fatalf("连接代理失败: %v", dialErr)
	}

	payload := []byte("hello netpanel")
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write(payload); err != nil {
		t.Fatalf("写入失败: %v", err)
	}
	buf := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, buf); err != nil {
		t.Fatalf("读取回显失败: %v", err)
	}
	if string(buf) != string(payload) {
		t.Errorf("回显内容不符: %q", buf)
	}
	conn.Close()

	p.Stop()
	if p.GetStatus() == "running" {
		t.Error("Stop 后状态不应为 running")
	}
	// 重复 Stop 应安全
	p.Stop()
}

func TestTCPProxyStopIdempotent(t *testing.T) {
	p := newTCPProxy("127.0.0.1", "127.0.0.1", 1, 1, 16, newTestLogger())
	p.Stop() // 未启动时 Stop 不应 panic
}

func TestTCPTrafficCounters(t *testing.T) {
	target, stopTarget := startEchoServer(t)
	defer stopTarget()

	thost, tport, _ := net.SplitHostPort(target)
	listenPort := freePort(t)
	p := newTCPProxy("127.0.0.1", thost, listenPort, atoi(tport), 16, newTestLogger())
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	for i := 0; i < 50; i++ {
		if conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", itoa(listenPort)), time.Second); err == nil {
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			conn.Write([]byte("0123456789"))
			buf := make([]byte, 10)
			io.ReadFull(conn, buf)
			conn.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// copyData 仅在连接读到 EOF 后才累加流量：关闭回显服务使 dst 方向 EOF
	stopTarget()

	// 给计数器留出结算窗口
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if p.GetTrafficIn() == 10 && p.GetTrafficOut() == 10 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("流量计数不符: in=%d out=%d，期望各 10", p.GetTrafficIn(), p.GetTrafficOut())
}

// ---- 小工具函数（避免引入 strconv 冲突的简写） ----

func atoi(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("申请端口失败: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
