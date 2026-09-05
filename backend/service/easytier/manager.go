package easytier

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const maxLogLines = 500 // 每个实例最多保留的日志行数

// ringBuffer 环形日志缓冲区，线程安全
type ringBuffer struct {
	mu   sync.RWMutex
	buf  []string
	size int
	pos  int
	full bool
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{buf: make([]string, size), size: size}
}

// write 写入一行日志
func (r *ringBuffer) write(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.pos] = line
	r.pos = (r.pos + 1) % r.size
	if r.pos == 0 {
		r.full = true
	}
}

// lines 返回所有日志行（按时间顺序）
func (r *ringBuffer) lines() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.full {
		result := make([]string, r.pos)
		copy(result, r.buf[:r.pos])
		return result
	}
	result := make([]string, r.size)
	copy(result, r.buf[r.pos:])
	copy(result[r.size-r.pos:], r.buf[:r.pos])
	return result
}

// PeerInfo 节点信息（来自 easytier-cli peer）
type PeerInfo struct {
	IPv4        string `json:"ipv4"`
	Hostname    string `json:"hostname"`
	Cost        string `json:"cost"`
	Latency     string `json:"latency"`
	TxBytes     string `json:"tx_bytes"`
	RxBytes     string `json:"rx_bytes"`
	TunnelProto string `json:"tunnel_proto"`
	NatType     string `json:"nat_type"`
	ID          string `json:"id"`
}

// RouteInfo 路由信息（来自 easytier-cli route）
type RouteInfo struct {
	IPv4        string `json:"ipv4"`
	Hostname    string `json:"hostname"`
	Proxy       string `json:"proxy_cidrs"`
	NextHopIPv4 string `json:"next_hop_ipv4"`
	Cost        string `json:"cost"`
}

// NodeInfo 节点综合信息
type NodeInfo struct {
	Peers  []PeerInfo  `json:"peers"`
	Routes []RouteInfo `json:"routes"`
}

type processEntry struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{} // 进程退出后关闭，用于等待进程完全退出
	logs   *ringBuffer  // 实时日志缓冲区
}

// Manager EasyTier 管理器（命令行进程管理）
type Manager struct {
	db       *gorm.DB
	log      *logrus.Logger
	dataDir  string
	clients  sync.Map // map[uint]*processEntry
	servers  sync.Map // map[uint]*processEntry
	stopping bool     // 标记是否正在关闭，关闭期间禁止自动重启
	mu       sync.Mutex
}

// isWinPcapPanic 检测 stderr 输出中是否包含 WinPcap/Npcap 接口枚举失败的 panic 信息
// EasyTier 进程 panic 时，详细信息输出到 stderr，cmd.Wait() 返回的 error 仅为退出码，
// 因此必须通过捕获 stderr 内容来判断崩溃原因。
func isWinPcapPanic(stderr string) bool {
	msg := strings.ToLower(stderr)
	return strings.Contains(msg, "unable to get interface list") ||
		strings.Contains(msg, "winpcap") ||
		strings.Contains(msg, "npcap") ||
		strings.Contains(msg, "pnet_datalink")
}

func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	// 将 dataDir 转为绝对路径，避免相对路径在工作目录变化时找不到二进制文件
	if absDir, err := filepath.Abs(dataDir); err == nil {
		dataDir = absDir
	}
	return &Manager{db: db, log: log, dataDir: dataDir}
}

// getBinaryPath 获取 easytier-core 二进制路径
func (m *Manager) getBinaryPath() string {
	binName := "easytier-core"
	if runtime.GOOS == "windows" {
		binName = "easytier-core.exe"
	}
	return filepath.Join(m.dataDir, "bin", binName)
}

// isBinaryAvailable 检查二进制是否存在
func (m *Manager) isBinaryAvailable() bool {
	_, err := os.Stat(m.getBinaryPath())
	return err == nil
}

func (m *Manager) StartAll() {
	go func() {
		var clients []model.EasytierClient
		m.db.Where("enable = ?", true).Find(&clients)
		for _, c := range clients {
			c := c
			go func() {
				if err := m.StartClient(c.ID); err != nil {
					m.log.Errorf("EasyTier 客户端 [%s] 启动失败: %v", c.Name, err)
				}
			}()
		}

		var servers []model.EasytierServer
		m.db.Where("enable = ?", true).Find(&servers)
		for _, s := range servers {
			s := s
			go func() {
				if err := m.StartServer(s.ID); err != nil {
					m.log.Errorf("EasyTier 服务端 [%s] 启动失败: %v", s.Name, err)
				}
			}()
		}
	}()
}

func (m *Manager) StopAll() {
	// 设置关闭标志，阻止自动重启
	m.mu.Lock()
	m.stopping = true
	m.mu.Unlock()

	var wg sync.WaitGroup

	m.clients.Range(func(key, value interface{}) bool {
		entry := value.(*processEntry)
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry.cancel()
			if entry.cmd.Process != nil {
				_ = entry.cmd.Process.Kill()
			}
			_ = entry.cmd.Wait()
		}()
		return true
	})
	m.servers.Range(func(key, value interface{}) bool {
		entry := value.(*processEntry)
		wg.Add(1)
		go func() {
			defer wg.Done()
			entry.cancel()
			if entry.cmd.Process != nil {
				_ = entry.cmd.Process.Kill()
			}
			_ = entry.cmd.Wait()
		}()
		return true
	})

	wg.Wait()
}

// ===== 客户端 =====

func (m *Manager) StartClient(id uint) error {
	m.StopClient(id)

	if !m.isBinaryAvailable() {
		return fmt.Errorf("easytier-core 二进制不存在，请先下载: %s", m.getBinaryPath())
	}

	var cfg model.EasytierClient
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("EasyTier 客户端配置不存在: %w", err)
	}

	args := m.buildClientArgs(&cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.getBinaryPath(), args...)
	// 设置工作目录为二进制文件所在目录，确保能找到 wintun.dll 等依赖文件
	cmd.Dir = filepath.Dir(m.getBinaryPath())

	// 创建日志缓冲区
	logBuf := newRingBuffer(maxLogLines)
	// 用于检测 WinPcap panic 的 stderr 缓冲
	var stderrBuf bytes.Buffer

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		cancel()
		m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return fmt.Errorf("启动 EasyTier 客户端失败: %w", err)
	}

	entry := &processEntry{cmd: cmd, cancel: cancel, done: make(chan struct{}), logs: logBuf}
	m.clients.Store(id, entry)

	// 异步读取 stdout
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logBuf.write(line)
			_, _ = fmt.Fprintln(os.Stdout, line)
		}
	}()
	// 异步读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logBuf.write("[stderr] " + line)
			stderrBuf.WriteString(line + "\n")
			_, _ = fmt.Fprintln(os.Stderr, line)
		}
	}()

	go func() {
		err := cmd.Wait()
		// 进程已退出，立即关闭 done channel，通知 StopClient 端口等资源已释放
		close(entry.done)
		stderrOutput := stderrBuf.String()
		m.clients.Delete(id)
		if err != nil {
			errMsg := fmt.Sprintf("进程异常退出: %v", err)
			m.log.Warnf("[EasyTier客户端][%d] %s", id, errMsg)
			m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": errMsg,
			})
			// 自动重启（延迟5秒，避免快速循环崩溃）
			time.Sleep(5 * time.Second)
			// 关闭期间不自动重启
			m.mu.Lock()
			isStopping := m.stopping
			m.mu.Unlock()
			if isStopping {
				return
			}
			var cur model.EasytierClient
			if m.db.First(&cur, id).Error == nil && cur.Enable {
				// 检测 WinPcap/Npcap 崩溃（通过 stderr 输出判断），自动开启 no_tun 选项
				if isWinPcapPanic(stderrOutput) && !cur.NoTun {
					m.log.Warnf("[EasyTier客户端][%d] 检测到 WinPcap/Npcap 崩溃，自动开启 --no-tun 模式", id)
					m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Update("no_tun", true)
				}
				m.log.Infof("[EasyTier客户端][%d] 尝试自动重启...", id)
				if restartErr := m.StartClient(id); restartErr != nil {
					m.log.Errorf("[EasyTier客户端][%d] 自动重启失败: %v", id, restartErr)
				}
			}
		} else {
			m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Update("status", "stopped")
			m.log.Infof("[EasyTier客户端][%d] 进程已退出", id)
		}
	}()

	m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[EasyTier客户端][%d] 已启动，PID: %d", id, cmd.Process.Pid)
	return nil
}

func (m *Manager) StopClient(id uint) {
	if val, ok := m.clients.Load(id); ok {
		entry := val.(*processEntry)
		entry.cancel()
		if entry.cmd.Process != nil {
			_ = entry.cmd.Process.Kill()
		}
		// 等待进程完全退出，确保端口等资源已释放
		<-entry.done
		m.clients.Delete(id)
	}
	m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetClientStatus(id uint) string {
	if _, ok := m.clients.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// buildClientArgs 构建 easytier-core 客户端命令行参数
func (m *Manager) buildClientArgs(cfg *model.EasytierClient) []string {
	var args []string

	// ===== 运行时选项 =====
	if cfg.MultiThread {
		args = append(args, "--multi-thread")
		if cfg.MultiThreadCount > 2 {
			args = append(args, "--multi-thread-count", fmt.Sprintf("%d", cfg.MultiThreadCount))
		}
	}

	// ===== 基本设置 =====
	if cfg.Hostname != "" {
		args = append(args, "--hostname", cfg.Hostname)
	}
	if cfg.InstanceName != "" {
		args = append(args, "--instance-name", cfg.InstanceName)
	}

	// ===== 网络设置 =====
	if cfg.NetworkName != "" {
		args = append(args, "--network-name", cfg.NetworkName)
	}
	if cfg.NetworkPassword != "" {
		args = append(args, "--network-secret", cfg.NetworkPassword.String())
	}

	// 虚拟 IP（DHCP 模式与手动指定互斥）
	if cfg.EnableDhcp {
		args = append(args, "--dhcp")
	} else if cfg.VirtualIP != "" {
		args = append(args, "--ipv4", cfg.VirtualIP)
	}
	if cfg.IPv6 != "" {
		args = append(args, "--ipv6", cfg.IPv6)
	}

	// 服务器地址（支持多个）
	if cfg.ServerAddr != "" {
		for _, s := range strings.Split(cfg.ServerAddr, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				args = append(args, "-p", s)
			}
		}
	}
	// 外部节点（公共共享节点）
	if cfg.ExternalNodes != "" {
		for _, s := range strings.Split(cfg.ExternalNodes, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				args = append(args, "-e", s)
			}
		}
	}

	// ===== 监听器设置 =====
	if cfg.NoListener || cfg.ListenPorts == "" {
		// 未指定监听端口时默认不监听，避免占用随机端口
		args = append(args, "--no-listener")
	} else {
		for _, p := range strings.Split(cfg.ListenPorts, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				args = append(args, "-l", p)
			}
		}
	}
	if cfg.MappedListeners != "" {
		for _, ml := range strings.Split(cfg.MappedListeners, ",") {
			ml = strings.TrimSpace(ml)
			if ml != "" {
				args = append(args, "--mapped-listeners", ml)
			}
		}
	}

	// ===== RPC 设置 =====
	if cfg.RpcPortal != "" {
		args = append(args, "--rpc-portal", cfg.RpcPortal)
	}
	if cfg.RpcPortalWhitelist != "" {
		args = append(args, "--rpc-portal-whitelist", cfg.RpcPortalWhitelist)
	}

	// ===== 子网代理 =====
	if cfg.ProxyCidrs != "" {
		for _, cidr := range strings.Split(cfg.ProxyCidrs, ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr != "" {
				args = append(args, "--proxy-networks", cidr)
			}
		}
	}

	// ===== 出口节点 =====
	if cfg.ExitNodes != "" {
		for _, node := range strings.Split(cfg.ExitNodes, ",") {
			node = strings.TrimSpace(node)
			if node != "" {
				args = append(args, "--exit-nodes", node)
			}
		}
	}

	// ===== 网络行为选项 =====
	if cfg.LatencyFirst {
		args = append(args, "--latency-first")
	}
	if cfg.DisableP2P {
		args = append(args, "--disable-p2p")
	}
	if cfg.P2POnly {
		args = append(args, "--p2p-only")
	}
	if cfg.EnableExitNode {
		args = append(args, "--enable-exit-node")
	}
	if cfg.RelayAllPeerRpc {
		args = append(args, "--relay-all-peer-rpc")
	}
	if cfg.ProxyForwardBySystem {
		args = append(args, "--proxy-forward-by-system")
	}
	if cfg.DefaultProtocol != "" {
		args = append(args, "--default-protocol", cfg.DefaultProtocol)
	}

	// ===== 打洞选项 =====
	if cfg.DisableUdpHolePunching {
		args = append(args, "--disable-udp-hole-punching")
	}
	if cfg.DisableTcpHolePunching {
		args = append(args, "--disable-tcp-hole-punching")
	}
	if cfg.DisableSymHolePunching {
		args = append(args, "--disable-sym-hole-punching")
	}

	// ===== 协议加速选项 =====
	if cfg.EnableKcpProxy {
		args = append(args, "--enable-kcp-proxy")
	}
	if cfg.DisableKcpInput {
		args = append(args, "--disable-kcp-input")
	}
	if cfg.EnableQuicProxy {
		args = append(args, "--enable-quic-proxy")
	}
	if cfg.DisableQuicInput {
		args = append(args, "--disable-quic-input")
	}
	if cfg.QuicListenPort > 0 {
		args = append(args, "--quic-listen-port", fmt.Sprintf("%d", cfg.QuicListenPort))
	}

	// ===== TUN/网卡选项 =====
	if cfg.NoTun {
		args = append(args, "--no-tun")
	}
	if cfg.DevName != "" {
		args = append(args, "--dev-name", cfg.DevName)
	}
	if cfg.UseSmoltcp {
		args = append(args, "--use-smoltcp")
	}
	if cfg.DisableIpv6 {
		args = append(args, "--disable-ipv6")
	}
	if cfg.Mtu > 0 {
		args = append(args, "--mtu", fmt.Sprintf("%d", cfg.Mtu))
	}
	if cfg.AcceptDns {
		args = append(args, "--accept-dns")
		if cfg.TldDnsZone != "" {
			args = append(args, "--tld-dns-zone", cfg.TldDnsZone)
		}
	}
	if cfg.BindDevice != "" {
		args = append(args, "--bind-device", cfg.BindDevice)
	}

	// ===== 安全选项 =====
	if cfg.DisableEncryption {
		args = append(args, "--disable-encryption")
	}
	if cfg.EncryptionAlgorithm != "" {
		args = append(args, "--encryption-algorithm", cfg.EncryptionAlgorithm)
	}
	if cfg.PrivateMode {
		args = append(args, "--private-mode")
	}
	if cfg.PrivateKey != "" {
		args = append(args, "--private-key", cfg.PrivateKey.String())
	}
	if cfg.PreSharedKey != "" {
		args = append(args, "--pre-shared-key", cfg.PreSharedKey)
	}

	// ===== 中继选项 =====
	if cfg.RelayNetworkWhitelist != "" {
		args = append(args, "--relay-network-whitelist", cfg.RelayNetworkWhitelist)
	}
	if cfg.ForeignRelayBpsLimit > 0 {
		args = append(args, "--foreign-relay-bps-limit", fmt.Sprintf("%d", cfg.ForeignRelayBpsLimit))
	}
	if cfg.DisableRelayKcp {
		args = append(args, "--disable-relay-kcp")
	}
	if cfg.EnableRelayForeignNetworkKcp {
		args = append(args, "--enable-relay-foreign-network-kcp")
	}

	// ===== 流量控制 =====
	if cfg.TcpWhitelist != "" {
		args = append(args, "--tcp-whitelist", cfg.TcpWhitelist)
	}
	if cfg.UdpWhitelist != "" {
		args = append(args, "--udp-whitelist", cfg.UdpWhitelist)
	}
	if cfg.Compression != "" && cfg.Compression != "none" {
		args = append(args, "--compression", cfg.Compression)
	}

	// ===== STUN 服务器 =====
	if cfg.StunServers != "" {
		for _, s := range strings.Split(cfg.StunServers, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				args = append(args, "--stun-servers", s)
			}
		}
	}
	if cfg.StunServersV6 != "" {
		for _, s := range strings.Split(cfg.StunServersV6, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				args = append(args, "--stun-servers-v6", s)
			}
		}
	}

	// ===== VPN 门户 =====
	if cfg.EnableVpnPortal && cfg.VpnPortalListenPort > 0 && cfg.VpnPortalClientNetwork != "" {
		args = append(args, "--vpn-portal",
			fmt.Sprintf("wg://0.0.0.0:%d/%s", cfg.VpnPortalListenPort, cfg.VpnPortalClientNetwork))
	}

	// ===== SOCKS5 代理 =====
	if cfg.EnableSocks5 && cfg.Socks5Port > 0 {
		args = append(args, "--socks5", fmt.Sprintf("%d", cfg.Socks5Port))
	}

	// ===== 手动路由 =====
	if cfg.EnableManualRoutes && cfg.ManualRoutes != "" {
		for _, route := range strings.Split(cfg.ManualRoutes, ",") {
			route = strings.TrimSpace(route)
			if route != "" {
				args = append(args, "--manual-routes", route)
			}
		}
	}

	// ===== 端口转发 =====
	if cfg.PortForwards != "" {
		for _, pf := range strings.Split(cfg.PortForwards, "\n") {
			pf = strings.TrimSpace(pf)
			if pf != "" {
				args = append(args, "--port-forward", pf)
			}
		}
	}

	// ===== 日志选项 =====
	if cfg.ConsoleLogLevel != "" {
		args = append(args, "--console-log-level", cfg.ConsoleLogLevel)
	}
	if cfg.FileLogLevel != "" {
		args = append(args, "--file-log-level", cfg.FileLogLevel)
	}
	if cfg.FileLogDir != "" {
		args = append(args, "--file-log-dir", cfg.FileLogDir)
	}
	if cfg.FileLogSize > 0 {
		args = append(args, "--file-log-size", fmt.Sprintf("%d", cfg.FileLogSize))
	}
	if cfg.FileLogCount > 0 {
		args = append(args, "--file-log-count", fmt.Sprintf("%d", cfg.FileLogCount))
	}

	// 额外参数（兜底，用于不常用的高级参数）
	if cfg.ExtraArgs != "" {
		extraParts := strings.Fields(cfg.ExtraArgs)
		args = append(args, extraParts...)
	}

	return args
}

// ===== 服务端 =====

func (m *Manager) StartServer(id uint) error {
	m.StopServer(id)

	if !m.isBinaryAvailable() {
		return fmt.Errorf("easytier-core 二进制不存在，请先下载: %s", m.getBinaryPath())
	}

	var cfg model.EasytierServer
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("EasyTier 服务端配置不存在: %w", err)
	}

	args := m.buildServerArgs(&cfg)
	m.log.Infof("[EasyTier服务端][%d] 启动命令: %s %s", id, m.getBinaryPath(), strings.Join(args, " "))
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.getBinaryPath(), args...)
	// 设置工作目录为二进制文件所在目录，确保能找到 wintun.dll 等依赖文件
	cmd.Dir = filepath.Dir(m.getBinaryPath())

	// 创建日志缓冲区
	logBuf := newRingBuffer(maxLogLines)
	// 用于检测 WinPcap panic 的 stderr 缓冲
	var stderrBuf bytes.Buffer

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		cancel()
		m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return fmt.Errorf("启动 EasyTier 服务端失败: %w", err)
	}

	entry := &processEntry{cmd: cmd, cancel: cancel, done: make(chan struct{}), logs: logBuf}
	m.servers.Store(id, entry)

	// 异步读取 stdout
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logBuf.write(line)
			_, _ = fmt.Fprintln(os.Stdout, line)
		}
	}()
	// 异步读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logBuf.write("[stderr] " + line)
			stderrBuf.WriteString(line + "\n")
			_, _ = fmt.Fprintln(os.Stderr, line)
		}
	}()

	go func() {
		err := cmd.Wait()
		// 进程已退出，立即关闭 done channel，通知 StopServer 端口等资源已释放
		close(entry.done)
		stderrOutput := stderrBuf.String()
		m.servers.Delete(id)
		if err != nil {
			errMsg := fmt.Sprintf("进程异常退出: %v", err)
			m.log.Warnf("[EasyTier服务端][%d] %s", id, errMsg)
			if stderrOutput != "" {
				m.log.Warnf("[EasyTier服务端][%d] stderr输出:\n%s", id, stderrOutput)
			}
			m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": errMsg,
			})
			// 自动重启（延迟5秒，避免快速循环崩溃）
			time.Sleep(5 * time.Second)
			// 关闭期间不自动重启
			m.mu.Lock()
			isStopping := m.stopping
			m.mu.Unlock()
			if isStopping {
				return
			}
			var cur model.EasytierServer
			if m.db.First(&cur, id).Error == nil && cur.Enable {
				// 检测 WinPcap/Npcap 崩溃（通过 stderr 输出判断），自动开启 no_tun 选项
				if isWinPcapPanic(stderrOutput) && !cur.NoTun {
					m.log.Warnf("[EasyTier服务端][%d] 检测到 WinPcap/Npcap 崩溃，自动开启 --no-tun 模式", id)
					m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Update("no_tun", true)
				}
				m.log.Infof("[EasyTier服务端][%d] 尝试自动重启...", id)
				if restartErr := m.StartServer(id); restartErr != nil {
					m.log.Errorf("[EasyTier服务端][%d] 自动重启失败: %v", id, restartErr)
				}
			}
		} else {
			m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Update("status", "stopped")
			m.log.Infof("[EasyTier服务端][%d] 进程已退出", id)
		}
	}()

	m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[EasyTier服务端][%d] 已启动，PID: %d", id, cmd.Process.Pid)
	return nil
}

func (m *Manager) StopServer(id uint) {
	if val, ok := m.servers.Load(id); ok {
		entry := val.(*processEntry)
		entry.cancel()
		if entry.cmd.Process != nil {
			_ = entry.cmd.Process.Kill()
		}
		// 等待进程完全退出，确保端口等资源已释放
		<-entry.done
		m.servers.Delete(id)
	}
	m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetServerStatus(id uint) string {
	if _, ok := m.servers.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// buildServerArgs 构建 easytier-core 服务端命令行参数
func (m *Manager) buildServerArgs(cfg *model.EasytierServer) []string {
	var args []string

	// ===== 运行时选项 =====
	if cfg.MultiThread {
		args = append(args, "--multi-thread")
		if cfg.MultiThreadCount > 2 {
			args = append(args, "--multi-thread-count", fmt.Sprintf("%d", cfg.MultiThreadCount))
		}
	}

	// ===== config-server 节点模式 =====
	// 节点模式下只需传入 --config-server 地址，其余参数由 config-server 下发，不再手动配置
	// URL 格式：tcp://host:port/<token>，token 不能为空
	if cfg.ServerMode == "config-server" && cfg.ConfigServerAddr != "" {
		for _, addr := range strings.Split(cfg.ConfigServerAddr, ",") {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			// 将 token 拼接到 URL 末尾（如果 URL 末尾没有 /token 路径）
			if cfg.ConfigServerToken != "" {
				// 去掉末尾的 /，再拼接 /<token>
				addr = strings.TrimRight(addr, "/") + "/" + cfg.ConfigServerToken
			}
			args = append(args, "--config-server", addr)
		}
		if cfg.MachineID != "" {
			args = append(args, "--machine-id", cfg.MachineID)
		}
		// config-server 模式下，配置由服务端下发，但 TUN 网卡创建需要管理员权限。
		// 若用户未勾选 no_tun，仍尊重用户配置（不强制加 --no-tun）。
		if cfg.NoTun {
			args = append(args, "--no-tun")
		}
		// 额外参数（兜底）
		if cfg.ExtraArgs != "" {
			extraParts := strings.Fields(cfg.ExtraArgs)
			args = append(args, extraParts...)
		}
		return args
	}

	// ===== 以下为 standalone 独立模式参数 =====

	if cfg.Hostname != "" {
		args = append(args, "--hostname", cfg.Hostname)
	}
	if cfg.InstanceName != "" {
		args = append(args, "--instance-name", cfg.InstanceName)
	}

	listenAddr := cfg.ListenAddr
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}

	// 监听端口（支持多个）
	if cfg.ListenPorts != "" {
		for _, p := range strings.Split(cfg.ListenPorts, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if strings.Contains(p, ":") {
				parts := strings.SplitN(p, ":", 2)
				args = append(args, "-l", fmt.Sprintf("%s://%s:%s", parts[0], listenAddr, parts[1]))
			} else {
				args = append(args, "-l", p)
			}
		}
	}

	// ===== RPC 设置 =====
	if cfg.RpcPortal != "" {
		args = append(args, "--rpc-portal", cfg.RpcPortal)
	}
	if cfg.RpcPortalWhitelist != "" {
		args = append(args, "--rpc-portal-whitelist", cfg.RpcPortalWhitelist)
	}

	// ===== 网络行为选项 =====
	if cfg.NoTun {
		args = append(args, "--no-tun")
	}
	if cfg.DisableP2P {
		args = append(args, "--disable-p2p")
	}
	if cfg.EnableExitNode {
		args = append(args, "--enable-exit-node")
	}
	if cfg.RelayAllPeerRpc {
		args = append(args, "--relay-all-peer-rpc")
	}
	if cfg.DefaultProtocol != "" {
		args = append(args, "--default-protocol", cfg.DefaultProtocol)
	}
	if cfg.ProxyForwardBySystem {
		args = append(args, "--proxy-forward-by-system")
	}

	// ===== 协议加速选项 =====
	if cfg.EnableKcpProxy {
		args = append(args, "--enable-kcp-proxy")
	}
	if cfg.DisableKcpInput {
		args = append(args, "--disable-kcp-input")
	}
	if cfg.EnableQuicProxy {
		args = append(args, "--enable-quic-proxy")
	}
	if cfg.DisableQuicInput {
		args = append(args, "--disable-quic-input")
	}
	if cfg.QuicListenPort > 0 {
		args = append(args, "--quic-listen-port", fmt.Sprintf("%d", cfg.QuicListenPort))
	}

	// ===== 安全选项 =====
	if cfg.DisableEncryption {
		args = append(args, "--disable-encryption")
	}
	if cfg.EncryptionAlgorithm != "" {
		args = append(args, "--encryption-algorithm", cfg.EncryptionAlgorithm)
	}
	if cfg.PrivateMode {
		args = append(args, "--private-mode")
	}
	if cfg.PrivateKey != "" {
		args = append(args, "--private-key", cfg.PrivateKey.String())
	}
	if cfg.PreSharedKey != "" {
		args = append(args, "--pre-shared-key", cfg.PreSharedKey)
	}

	// ===== 中继选项 =====
	// 服务端必须传入 --relay-network-whitelist，否则 easytier-core 会以普通节点模式启动并立即退出。
	// 默认值 "*" 表示中继所有网络；用户可自定义为 "net1,net2" 等。
	relayWhitelist := cfg.RelayNetworkWhitelist
	if relayWhitelist == "" {
		relayWhitelist = "*"
	}
	args = append(args, "--relay-network-whitelist", relayWhitelist)
	if cfg.ForeignRelayBpsLimit > 0 {
		args = append(args, "--foreign-relay-bps-limit", fmt.Sprintf("%d", cfg.ForeignRelayBpsLimit))
	}
	if cfg.DisableRelayKcp {
		args = append(args, "--disable-relay-kcp")
	}
	if cfg.EnableRelayForeignNetworkKcp {
		args = append(args, "--enable-relay-foreign-network-kcp")
	}

	// ===== 流量控制 =====
	if cfg.TcpWhitelist != "" {
		args = append(args, "--tcp-whitelist", cfg.TcpWhitelist)
	}
	if cfg.UdpWhitelist != "" {
		args = append(args, "--udp-whitelist", cfg.UdpWhitelist)
	}
	if cfg.Compression != "" && cfg.Compression != "none" {
		args = append(args, "--compression", cfg.Compression)
	}

	// ===== STUN 服务器 =====
	if cfg.StunServers != "" {
		for _, s := range strings.Split(cfg.StunServers, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				args = append(args, "--stun-servers", s)
			}
		}
	}
	if cfg.StunServersV6 != "" {
		for _, s := range strings.Split(cfg.StunServersV6, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				args = append(args, "--stun-servers-v6", s)
			}
		}
	}

	// ===== 手动路由 =====
	if cfg.EnableManualRoutes && cfg.ManualRoutes != "" {
		for _, route := range strings.Split(cfg.ManualRoutes, ",") {
			route = strings.TrimSpace(route)
			if route != "" {
				args = append(args, "--manual-routes", route)
			}
		}
	}

	// ===== 端口转发 =====
	if cfg.PortForwards != "" {
		for _, pf := range strings.Split(cfg.PortForwards, "\n") {
			pf = strings.TrimSpace(pf)
			if pf != "" {
				args = append(args, "--port-forward", pf)
			}
		}
	}

	// ===== 日志选项 =====
	if cfg.ConsoleLogLevel != "" {
		args = append(args, "--console-log-level", cfg.ConsoleLogLevel)
	}
	if cfg.FileLogLevel != "" {
		args = append(args, "--file-log-level", cfg.FileLogLevel)
	}
	if cfg.FileLogDir != "" {
		args = append(args, "--file-log-dir", cfg.FileLogDir)
	}
	if cfg.FileLogSize > 0 {
		args = append(args, "--file-log-size", fmt.Sprintf("%d", cfg.FileLogSize))
	}
	if cfg.FileLogCount > 0 {
		args = append(args, "--file-log-count", fmt.Sprintf("%d", cfg.FileLogCount))
	}

	// 额外参数（兜底）
	if cfg.ExtraArgs != "" {
		extraParts := strings.Fields(cfg.ExtraArgs)
		args = append(args, extraParts...)
	}

	return args
}

// ===== 日志与节点信息 =====

// GetClientLogs 获取客户端实例的实时日志（最近 maxLogLines 行）
func (m *Manager) GetClientLogs(id uint) []string {
	if val, ok := m.clients.Load(id); ok {
		return val.(*processEntry).logs.lines()
	}
	return []string{}
}

// GetServerLogs 获取服务端实例的实时日志（最近 maxLogLines 行）
func (m *Manager) GetServerLogs(id uint) []string {
	if val, ok := m.servers.Load(id); ok {
		return val.(*processEntry).logs.lines()
	}
	return []string{}
}

// getRpcPortal 从配置中提取 RPC 地址，用于调用 easytier-cli
func getRpcPortal(rpcPortal string) string {
	if rpcPortal == "" || rpcPortal == "0" {
		return ""
	}
	// 如果只是端口号，补全为 127.0.0.1:port
	if !strings.Contains(rpcPortal, ":") {
		return "127.0.0.1:" + rpcPortal
	}
	// 如果是 0.0.0.0:port 形式，替换为 127.0.0.1:port
	parts := strings.SplitN(rpcPortal, ":", 2)
	if parts[0] == "0.0.0.0" || parts[0] == "" {
		return "127.0.0.1:" + parts[1]
	}
	return rpcPortal
}

// getCliPath 获取 easytier-cli 二进制路径
func (m *Manager) getCliPath() string {
	cliName := "easytier-cli"
	if runtime.GOOS == "windows" {
		cliName = "easytier-cli.exe"
	}
	return filepath.Join(m.dataDir, "bin", cliName)
}

// runCli 执行 easytier-cli 命令并返回输出
func (m *Manager) runCli(rpcAddr string, args ...string) (string, error) {
	cliPath := m.getCliPath()
	if _, err := os.Stat(cliPath); err != nil {
		return "", fmt.Errorf("easytier-cli 不存在: %s", cliPath)
	}
	cmdArgs := []string{"--rpc-portal", rpcAddr}
	cmdArgs = append(cmdArgs, args...)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cliPath, cmdArgs...)
	cmd.Dir = filepath.Dir(cliPath)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("easytier-cli 执行失败: %w", err)
	}
	return string(out), nil
}

// parseCliPeerTable 解析 easytier-cli peer 的文本表格输出为结构化数据
// 输出格式（表头行 + 数据行，以 | 分隔）：
// ipv4 | hostname | cost | latency | tx_bytes | rx_bytes | tunnel_proto | nat_type | id
func parseCliPeerTable(output string) []PeerInfo {
	var peers []PeerInfo
	lines := strings.Split(output, "\n")
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 跳过分隔线（全是 - 和 +）
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			continue
		}
		// 检测表头行
		if strings.Contains(line, "ipv4") && strings.Contains(line, "hostname") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		// 解析数据行
		cols := strings.Split(line, "|")
		if len(cols) < 9 {
			continue
		}
		trim := func(s string) string { return strings.TrimSpace(s) }
		peers = append(peers, PeerInfo{
			IPv4:        trim(cols[0]),
			Hostname:    trim(cols[1]),
			Cost:        trim(cols[2]),
			Latency:     trim(cols[3]),
			TxBytes:     trim(cols[4]),
			RxBytes:     trim(cols[5]),
			TunnelProto: trim(cols[6]),
			NatType:     trim(cols[7]),
			ID:          trim(cols[8]),
		})
	}
	return peers
}

// parseCliRouteTable 解析 easytier-cli route 的文本表格输出
func parseCliRouteTable(output string) []RouteInfo {
	var routes []RouteInfo
	lines := strings.Split(output, "\n")
	inTable := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			continue
		}
		if strings.Contains(line, "ipv4") && strings.Contains(line, "hostname") {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 5 {
			continue
		}
		trim := func(s string) string { return strings.TrimSpace(s) }
		routes = append(routes, RouteInfo{
			IPv4:        trim(cols[0]),
			Hostname:    trim(cols[1]),
			Proxy:       trim(cols[2]),
			NextHopIPv4: trim(cols[3]),
			Cost:        trim(cols[4]),
		})
	}
	return routes
}

// GetClientPeers 通过 easytier-cli 获取客户端节点信息
func (m *Manager) GetClientPeers(id uint) (*NodeInfo, error) {
	var cfg model.EasytierClient
	if err := m.db.First(&cfg, id).Error; err != nil {
		return nil, fmt.Errorf("配置不存在: %w", err)
	}
	rpcAddr := getRpcPortal(cfg.RpcPortal)
	if rpcAddr == "" {
		return nil, fmt.Errorf("未配置 RPC 门户地址，无法获取节点信息")
	}
	return m.fetchNodeInfo(rpcAddr)
}

// GetServerPeers 通过 easytier-cli 获取服务端节点信息
func (m *Manager) GetServerPeers(id uint) (*NodeInfo, error) {
	var cfg model.EasytierServer
	if err := m.db.First(&cfg, id).Error; err != nil {
		return nil, fmt.Errorf("配置不存在: %w", err)
	}
	rpcAddr := getRpcPortal(cfg.RpcPortal)
	if rpcAddr == "" {
		return nil, fmt.Errorf("未配置 RPC 门户地址，无法获取节点信息")
	}
	return m.fetchNodeInfo(rpcAddr)
}

// fetchNodeInfo 通过 RPC 地址获取节点信息（peer + route）
func (m *Manager) fetchNodeInfo(rpcAddr string) (*NodeInfo, error) {
	// 尝试 JSON 输出（新版 easytier-cli 支持 --output-format json）
	peerOut, peerErr := m.runCli(rpcAddr, "peer", "--output-format", "json")
	routeOut, routeErr := m.runCli(rpcAddr, "route", "--output-format", "json")

	info := &NodeInfo{}

	if peerErr == nil && strings.TrimSpace(peerOut) != "" {
		// 尝试解析 JSON
		var peers []PeerInfo
		if err := json.Unmarshal([]byte(strings.TrimSpace(peerOut)), &peers); err == nil {
			info.Peers = peers
		} else {
			// 回退到文本解析
			info.Peers = parseCliPeerTable(peerOut)
		}
	} else if peerErr != nil {
		// 回退：不带 --output-format 参数
		peerOut2, err2 := m.runCli(rpcAddr, "peer")
		if err2 == nil {
			info.Peers = parseCliPeerTable(peerOut2)
		}
	}

	if routeErr == nil && strings.TrimSpace(routeOut) != "" {
		var routes []RouteInfo
		if err := json.Unmarshal([]byte(strings.TrimSpace(routeOut)), &routes); err == nil {
			info.Routes = routes
		} else {
			info.Routes = parseCliRouteTable(routeOut)
		}
	} else if routeErr != nil {
		routeOut2, err2 := m.runCli(rpcAddr, "route")
		if err2 == nil {
			info.Routes = parseCliRouteTable(routeOut2)
		}
	}

	return info, nil
}

