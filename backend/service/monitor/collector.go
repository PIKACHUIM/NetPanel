package monitor

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// Collector 数据采集器（SSH/HTTP 被动采集）
type Collector struct {
	db      *gorm.DB
	manager *Manager
	
	// SSH 连接池
	sshClients map[uint]*ssh.Client
}

// NewCollector 创建数据采集器
func NewCollector(db *gorm.DB, manager *Manager) *Collector {
	return &Collector{
		db:         db,
		manager:    manager,
		sshClients: make(map[uint]*ssh.Client),
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

// getSSHClient 获取或创建 SSH 客户端
func (c *Collector) getSSHClient(server model.MonitorServer) (*ssh.Client, error) {
	// 检查现有连接
	if client, ok := c.sshClients[server.ID]; ok {
		// 测试连接是否有效
		session, err := client.NewSession()
		if err == nil {
			session.Close()
			return client, nil
		}
		// 连接失效，删除
		delete(c.sshClients, server.ID)
	}
	
	// 创建新连接
	config := &ssh.ClientConfig{
		User:            server.SSHUser,
		Timeout:         10 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	
	// 认证方式
	if server.SSHPassword != "" {
		config.Auth = []ssh.AuthMethod{
			ssh.Password(server.SSHPassword),
		}
	} else if server.SSHKeyFile != "" {
		// TODO: 读取私钥文件
		return nil, fmt.Errorf("私钥认证暂未实现")
	} else {
		return nil, fmt.Errorf("未配置 SSH 认证信息")
	}
	
	client, err := ssh.Dial("tcp", server.SSHAddr, config)
	if err != nil {
		return nil, err
	}
	
	c.sshClients[server.ID] = client
	return client, nil
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
	target := fmt.Sprintf("%s:%d", addr, port)
	
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
	target := fmt.Sprintf("%s:%d", addr, port)
	
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

// CloseSSHConnections 关闭所有 SSH 连接
func (c *Collector) CloseSSHConnections() {
	for id, client := range c.sshClients {
		if err := client.Close(); err != nil {
			log.Printf("[Collector] 关闭 SSH 连接失败 (server_id=%d): %v\n", id, err)
		}
	}
	c.sshClients = make(map[uint]*ssh.Client)
}
