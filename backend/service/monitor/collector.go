package monitor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// Collector 数据采集器（SSH/HTTP 被动采集）
type Collector struct {
	db      *gorm.DB
	manager *Manager

	// dataDir 数据目录，用于存放 known_hosts
	dataDir string

	// SSH 连接池。sshMu 保护 sshClients 的并发读写：
	// 该 map 会被探测引擎的并发 goroutine、监控任务与终端会话同时访问。
	sshMu      sync.Mutex
	sshClients map[uint]*ssh.Client

	// knownHostsMu 串行化 known_hosts 文件的追加写入
	knownHostsMu sync.Mutex
}

// NewCollector 创建数据采集器
func NewCollector(db *gorm.DB, manager *Manager) *Collector {
	dataDir := "./data"
	if manager != nil && manager.DataDir != "" {
		dataDir = manager.DataDir
	}
	return &Collector{
		db:         db,
		manager:    manager,
		dataDir:    dataDir,
		sshClients: make(map[uint]*ssh.Client),
	}
}

// CloseAll 关闭所有 SSH 连接，供服务停止时回收资源。
func (c *Collector) CloseAll() {
	c.sshMu.Lock()
	defer c.sshMu.Unlock()
	for id, client := range c.sshClients {
		_ = client.Close()
		delete(c.sshClients, id)
	}
}

// CollectMetricsViaSSH 通过 SSH 采集监控指标
func (c *Collector) CollectMetricsViaSSH(server model.MonitorServer) (*model.MonitorMetric, error) {
	client, err := c.getSSHClient(server)
	if err != nil {
		return nil, fmt.Errorf("SSH 连接失败: %w", err)
	}
	
	metric := &model.MonitorMetric{
		ServerID:  server.ID,
		Timestamp: time.Now(),
	}
	
	// 采集 CPU 信息
	if cpuUsage, err := c.getCPUUsageSSH(client); err == nil {
		metric.CPUUsage = cpuUsage
	}
	
	// 采集内存信息
	if memInfo, err := c.getMemoryInfoSSH(client); err == nil {
		metric.MemTotal = memInfo.Total
		metric.MemUsed = memInfo.Used
		metric.MemAvailable = memInfo.Available
		metric.MemUsage = memInfo.Usage
	}
	
	// 采集硬盘信息
	if diskInfo, err := c.getDiskInfoSSH(client); err == nil {
		metric.DiskTotal = diskInfo.Total
		metric.DiskUsed = diskInfo.Used
		metric.DiskUsage = diskInfo.Usage
	}
	
	// 采集网络信息
	if netInfo, err := c.getNetworkInfoSSH(client); err == nil {
		metric.NetSent = netInfo.Sent
		metric.NetRecv = netInfo.Recv
	}
	
	// 采集进程信息
	if processCount, err := c.getProcessCountSSH(client); err == nil {
		metric.ProcessCount = processCount
	}
	
	return metric, nil
}

// getSSHClient 获取或创建 SSH 客户端。
//
// 并发安全：sshClients 会被探测引擎的并发 goroutine、监控任务与终端会话同时访问，
// 原实现无锁保护，属确定的并发 map 读写，会触发
// "fatal error: concurrent map writes" 使整个进程崩溃。
func (c *Collector) getSSHClient(server model.MonitorServer) (*ssh.Client, error) {
	// 检查现有连接
	c.sshMu.Lock()
	client, ok := c.sshClients[server.ID]
	c.sshMu.Unlock()
	if ok {
		// 测试连接是否有效
		session, err := client.NewSession()
		if err == nil {
			session.Close()
			return client, nil
		}
		// 连接失效：关闭并移除，避免文件描述符泄漏
		_ = client.Close()
		c.sshMu.Lock()
		delete(c.sshClients, server.ID)
		c.sshMu.Unlock()
	}

	hostKeyCallback, err := c.hostKeyCallback()
	if err != nil {
		return nil, err
	}

	// 创建新连接
	config := &ssh.ClientConfig{
		User:            server.SSHUser,
		Timeout:         10 * time.Second,
		HostKeyCallback: hostKeyCallback,
	}

	// 认证方式：优先私钥，其次密码
	switch {
	case server.SSHKeyFile != "":
		keyData, readErr := os.ReadFile(server.SSHKeyFile)
		if readErr != nil {
			return nil, fmt.Errorf("读取 SSH 私钥失败: %w", readErr)
		}
		signer, parseErr := ssh.ParsePrivateKey(keyData)
		if parseErr != nil {
			return nil, fmt.Errorf("解析 SSH 私钥失败（如已加密请使用未加密私钥）: %w", parseErr)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	case server.SSHPassword != "":
		config.Auth = []ssh.AuthMethod{ssh.Password(server.SSHPassword.String())}
	default:
		return nil, fmt.Errorf("未配置 SSH 认证信息")
	}

	newClient, err := ssh.Dial("tcp", server.SSHAddr, config)
	if err != nil {
		return nil, err
	}

	c.sshMu.Lock()
	// 双检：并发场景下可能已有其他 goroutine 建立了连接
	if existing, ok := c.sshClients[server.ID]; ok {
		c.sshMu.Unlock()
		_ = newClient.Close()
		return existing, nil
	}
	c.sshClients[server.ID] = newClient
	c.sshMu.Unlock()
	return newClient, nil
}

// hostKeyCallback 构造主机指纹校验回调。
//
// 原实现使用 ssh.InsecureIgnoreHostKey()，完全放弃主机身份校验，
// 使监控通道可被中间人攻击（攻击者可窃取 SSH 凭据）。
// 现改为基于 known_hosts 校验：
//   - 已记录的主机：指纹不匹配时拒绝连接；
//   - 未记录的新主机：首次连接时记录指纹（TOFU，Trust On First Use）；
//   - 设置 NETPANEL_SSH_STRICT_HOST_KEY=1 时，未记录的主机也一律拒绝。
func (c *Collector) hostKeyCallback() (ssh.HostKeyCallback, error) {
	path := filepath.Join(c.dataDir, "known_hosts")
	if err := os.MkdirAll(c.dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	// 确保文件存在，knownhosts.New 对不存在的文件会报错
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if f, createErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600); createErr == nil {
			f.Close()
		}
	}

	verify, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("加载 known_hosts 失败: %w", err)
	}

	strict := os.Getenv("NETPANEL_SSH_STRICT_HOST_KEY") == "1"

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := verify(hostname, remote, key)
		if err == nil {
			return nil
		}
		// 指纹不匹配（已记录但对不上）：一律拒绝，这是中间人攻击的典型特征
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) && len(keyErr.Want) > 0 {
			return fmt.Errorf("SSH 主机密钥校验失败（可能存在中间人攻击）: %s", hostname)
		}
		// 未记录的新主机
		if strict {
			return fmt.Errorf("未知的 SSH 主机 %s；严格模式下需先手动写入 known_hosts", hostname)
		}
		return c.appendKnownHost(path, hostname, key)
	}, nil
}

// appendKnownHost 将新主机指纹追加到 known_hosts（首次连接信任）。
func (c *Collector) appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	c.knownHostsMu.Lock()
	defer c.knownHostsMu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("写入 known_hosts 失败: %w", err)
	}
	defer f.Close()

	line := knownhosts.Line([]string{hostname}, key)
	if _, err := f.WriteString(line + "\n"); err != nil {
		return fmt.Errorf("写入 known_hosts 失败: %w", err)
	}
	return nil
}

// executeCommandViaSSH 通过 SSH 执行命令
func (c *Collector) executeCommandViaSSH(server model.MonitorServer, command string, timeout int) (string, error) {
	client, err := c.getSSHClient(server)
	if err != nil {
		return "", err
	}
	
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	
	var stdout bytes.Buffer
	session.Stdout = &stdout
	
	// 设置超时
	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()
	
	select {
	case err := <-done:
		if err != nil {
			return stdout.String(), err
		}
		return stdout.String(), nil
	case <-time.After(time.Duration(timeout) * time.Second):
		session.Signal(ssh.SIGKILL)
		return stdout.String(), fmt.Errorf("命令执行超时")
	}
}

// getCPUUsageSSH 获取 CPU 使用率
func (c *Collector) getCPUUsageSSH(client *ssh.Client) (float64, error) {
	session, err := client.NewSession()
	if err != nil {
		return 0, err
	}
	defer session.Close()
	
	// 使用 top 命令获取 CPU 使用率
	output, err := session.CombinedOutput("top -bn1 | grep 'Cpu(s)' | awk '{print $2}' | cut -d'%' -f1")
	if err != nil {
		return 0, err
	}
	
	var usage float64
	fmt.Sscanf(string(output), "%f", &usage)
	return usage, nil
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total     uint64
	Used      uint64
	Available uint64
	Usage     float64
}

// getMemoryInfoSSH 获取内存信息
func (c *Collector) getMemoryInfoSSH(client *ssh.Client) (*MemoryInfo, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	
	output, err := session.CombinedOutput("free -b | grep Mem")
	if err != nil {
		return nil, err
	}
	
	var info MemoryInfo
	fields := strings.Fields(string(output))
	if len(fields) >= 7 {
		fmt.Sscanf(fields[1], "%d", &info.Total)
		fmt.Sscanf(fields[2], "%d", &info.Used)
		fmt.Sscanf(fields[6], "%d", &info.Available)
		
		if info.Total > 0 {
			info.Usage = float64(info.Used) / float64(info.Total) * 100
		}
	}
	
	return &info, nil
}

// DiskInfo 硬盘信息
type DiskInfo struct {
	Total uint64
	Used  uint64
	Usage float64
}

// getDiskInfoSSH 获取硬盘信息
func (c *Collector) getDiskInfoSSH(client *ssh.Client) (*DiskInfo, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	
	output, err := session.CombinedOutput("df -B1 / | tail -1")
	if err != nil {
		return nil, err
	}
	
	var info DiskInfo
	fields := strings.Fields(string(output))
	if len(fields) >= 5 {
		fmt.Sscanf(fields[1], "%d", &info.Total)
		fmt.Sscanf(fields[2], "%d", &info.Used)
		
		if info.Total > 0 {
			info.Usage = float64(info.Used) / float64(info.Total) * 100
		}
	}
	
	return &info, nil
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	Sent uint64
	Recv uint64
}

// getNetworkInfoSSH 获取网络信息
func (c *Collector) getNetworkInfoSSH(client *ssh.Client) (*NetworkInfo, error) {
	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer session.Close()
	
	// 简化版：读取第一个非 lo 网卡的流量
	output, err := session.CombinedOutput("cat /proc/net/dev | grep -v 'lo:' | grep ':' | head -1")
	if err != nil {
		return nil, err
	}
	
	var info NetworkInfo
	fields := strings.Fields(string(output))
	if len(fields) >= 10 {
		fmt.Sscanf(fields[1], "%d", &info.Recv)
		fmt.Sscanf(fields[9], "%d", &info.Sent)
	}
	
	return &info, nil
}

// getProcessCountSSH 获取进程数
func (c *Collector) getProcessCountSSH(client *ssh.Client) (int, error) {
	session, err := client.NewSession()
	if err != nil {
		return 0, err
	}
	defer session.Close()
	
	output, err := session.CombinedOutput("ps aux | wc -l")
	if err != nil {
		return 0, err
	}
	
	var count int
	fmt.Sscanf(string(output), "%d", &count)
	return count - 1, nil // 减去标题行
}

// ProbeHTTP HTTP 探测
func (c *Collector) ProbeHTTP(probeURL string, timeout int) (bool, int64, int, error) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	
	start := time.Now()
	resp, err := client.Get(probeURL)
	responseTime := time.Since(start).Milliseconds()
	
	if err != nil {
		return false, responseTime, 0, err
	}
	defer resp.Body.Close()
	
	// 读取响应体（但不使用）
	io.Copy(io.Discard, resp.Body)
	
	success := resp.StatusCode >= 200 && resp.StatusCode < 400
	return success, responseTime, resp.StatusCode, nil
}

// ProbeTCP TCP 探测
func (c *Collector) ProbeTCP(addr string, port int, timeout int) (bool, int64, error) {
	target := net.JoinHostPort(addr, strconv.Itoa(port))
	
	start := time.Now()
	conn, err := net.DialTimeout("tcp", target, time.Duration(timeout)*time.Second)
	responseTime := time.Since(start).Milliseconds()
	
	if err != nil {
		return false, responseTime, err
	}
	
	conn.Close()
	return true, responseTime, nil
}

// ProbeUDP UDP 探测
func (c *Collector) ProbeUDP(addr string, port int, timeout int) (bool, int64, error) {
	target := net.JoinHostPort(addr, strconv.Itoa(port))
	
	start := time.Now()
	conn, err := net.DialTimeout("udp", target, time.Duration(timeout)*time.Second)
	responseTime := time.Since(start).Milliseconds()
	
	if err != nil {
		return false, responseTime, err
	}
	
	// UDP 是无连接的，这里只是测试能否创建连接
	conn.Close()
	return true, responseTime, nil
}

// ProbeICMP ICMP 探测（Ping），返回是否可达与往返延迟（毫秒）
func (c *Collector) ProbeICMP(addr string, timeout int) (bool, int64, error) {
	// 解析目标地址（支持域名与 IP）
	ip, err := net.ResolveIPAddr("ip", addr)
	if err != nil {
		return false, 0, fmt.Errorf("解析目标地址失败: %w", err)
	}
	
	// ICMP 原始套接字需要特权（Linux root / macOS 亦需权限）
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return false, 0, fmt.Errorf("创建 ICMP 套接字失败（可能需要管理员/root 权限）: %w", err)
	}
	defer conn.Close()
	
	// 构造 Echo 请求
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("netpanel-monitor"),
		},
	}
	
	msgBytes, err := msg.Marshal(nil)
	if err != nil {
		return false, 0, err
	}
	
	// 设置读取超时
	if err := conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Second)); err != nil {
		return false, 0, err
	}
	
	start := time.Now()
	if _, err := conn.WriteTo(msgBytes, &net.IPAddr{IP: ip.IP}); err != nil {
		return false, time.Since(start).Milliseconds(), err
	}
	
	// 读取 Echo Reply
	reply := make([]byte, 1500)
	n, _, err := conn.ReadFrom(reply)
	responseTime := time.Since(start).Milliseconds()
	if err != nil {
		return false, responseTime, err
	}
	
	replyMsg, err := icmp.ParseMessage(1, reply[:n])
	if err != nil {
		return false, responseTime, err
	}
	
	if replyMsg.Type == ipv4.ICMPTypeEchoReply {
		return true, responseTime, nil
	}
	return false, responseTime, fmt.Errorf("收到非预期 ICMP 回复类型: %v", replyMsg.Type)
}

// CloseSSHConnections 关闭所有 SSH 连接
func (c *Collector) CloseSSHConnections() {
	for id, client := range c.sshClients {
		if err := client.Close(); err != nil {
			log.Printf("[Collector] 关闭 SSH 连接失败 (server_id=%d): %v\n", id, err)
		}
	}
	c.sshClients = make(map[uint]*ssh.Client)
}
