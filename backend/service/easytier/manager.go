package easytier

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

type processEntry struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// Manager EasyTier 管理器（命令行进程管理）
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	dataDir string
	clients sync.Map // map[uint]*processEntry
	servers sync.Map // map[uint]*processEntry
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
	m.clients.Range(func(key, value interface{}) bool {
		entry := value.(*processEntry)
		entry.cancel()
		return true
	})
	m.servers.Range(func(key, value interface{}) bool {
		entry := value.(*processEntry)
		entry.cancel()
		return true
	})
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
	cmd.Stdout = os.Stdout
	// 使用 bytes.Buffer 捕获 stderr，同时保留输出到控制台
	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Start(); err != nil {
		cancel()
		m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return fmt.Errorf("启动 EasyTier 客户端失败: %w", err)
	}

	entry := &processEntry{cmd: cmd, cancel: cancel}
	m.clients.Store(id, entry)

	go func() {
		err := cmd.Wait()
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
			entry.cmd.Process.Kill()
		}
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

	// 多线程运行时
	if cfg.MultiThread {
		args = append(args, "--multi-thread")
	}

	// 自定义主机名
	if cfg.Hostname != "" {
		args = append(args, "--hostname", cfg.Hostname)
	}

	// 服务器地址（支持多个）
	if cfg.ServerAddr != "" {
		servers := strings.Split(cfg.ServerAddr, ",")
		for _, s := range servers {
			s = strings.TrimSpace(s)
			if s != "" {
				args = append(args, "-p", s)
			}
		}
	}

	// 网络名称和密码
	if cfg.NetworkName != "" {
		args = append(args, "--network-name", cfg.NetworkName)
	}
	if cfg.NetworkPassword != "" {
		args = append(args, "--network-secret", cfg.NetworkPassword)
	}

	// 虚拟 IP（DHCP 模式与手动指定互斥）
	if cfg.EnableDhcp {
		args = append(args, "--dhcp")
	} else if cfg.VirtualIP != "" {
		args = append(args, "--ipv4", cfg.VirtualIP)
	}

	// 不创建 TUN 虚拟网卡（无需 WinPcap/Npcap，适用于仅中继场景）
	if cfg.NoTun {
		args = append(args, "--no-tun")
	}

	// 自定义 TUN 设备名
	if cfg.DevName != "" {
		args = append(args, "--dev-name", cfg.DevName)
	}

	// 本地监听端口（支持多个）
	if cfg.ListenPorts != "" {
		ports := strings.Split(cfg.ListenPorts, ",")
		for _, p := range ports {
			p = strings.TrimSpace(p)
			if p != "" {
				args = append(args, "-l", p)
			}
		}
	}

	// 映射监听器（公告外部地址，用于 NAT 后）
	if cfg.MappedListeners != "" {
		for _, ml := range strings.Split(cfg.MappedListeners, ",") {
			ml = strings.TrimSpace(ml)
			if ml != "" {
				args = append(args, "--mapped-listeners", ml)
			}
		}
	}

	// 子网代理（将本机子网共享给虚拟网络）
	if cfg.ProxyCidrs != "" {
		for _, cidr := range strings.Split(cfg.ProxyCidrs, ",") {
			cidr = strings.TrimSpace(cidr)
			if cidr != "" {
				args = append(args, "--proxy-networks", cidr)
			}
		}
	}

	// 出口节点
	if cfg.ExitNodes != "" {
		for _, node := range strings.Split(cfg.ExitNodes, ",") {
			node = strings.TrimSpace(node)
			if node != "" {
				args = append(args, "--exit-nodes", node)
			}
		}
	}

	// ===== 网络行为选项 =====

	// 延迟优先路由
	if cfg.LatencyFirst {
		args = append(args, "--latency-first")
	}

	// 禁用 P2P 直连，强制走中继
	if cfg.DisableP2P {
		args = append(args, "--disable-p2p")
	}

	// 仅 P2P，禁用中继
	if cfg.P2POnly {
		args = append(args, "--p2p-only")
	}

	// 允许本节点作为出口节点
	if cfg.EnableExitNode {
		args = append(args, "--enable-exit-node")
	}

	// 中继所有对等 RPC
	if cfg.RelayAllPeerRpc {
		args = append(args, "--relay-all-peer-rpc")
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
	if cfg.EnableQuicProxy {
		args = append(args, "--enable-quic-proxy")
	}

	// ===== TUN/网卡选项 =====
	if cfg.UseSmoltcp {
		args = append(args, "--use-smoltcp")
	}
	if cfg.DisableIpv6 {
		args = append(args, "--disable-ipv6")
	}
	if cfg.Mtu > 0 {
		args = append(args, "--mtu", fmt.Sprintf("%d", cfg.Mtu))
	}
	if cfg.EnableMagicDns {
		args = append(args, "--enable-magic-dns")
	}

	// ===== 安全选项 =====
	if cfg.DisableEncryption {
		args = append(args, "--disable-encryption")
	}
	if cfg.EnablePrivateMode {
		args = append(args, "--enable-private-mode")
	}

	// ===== 中继选项 =====
	if cfg.RelayNetworkWhitelist != "" {
		args = append(args, "--relay-network-whitelist", cfg.RelayNetworkWhitelist)
	}

	// ===== VPN 门户 =====
	if cfg.EnableVpnPortal && cfg.VpnPortalListenPort > 0 && cfg.VpnPortalClientNetwork != "" {
		args = append(args, "--vpn-portal",
			fmt.Sprintf("wg://0.0.0.0:%d/%s", cfg.VpnPortalListenPort, cfg.VpnPortalClientNetwork))
	}

	// ===== SOCKS5 代理 =====
	if cfg.EnableSocks5 && cfg.Socks5Port > 0 {
		args = append(args, "--socks5", fmt.Sprintf("socks5://0.0.0.0:%d", cfg.Socks5Port))
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
	// 格式：proto:bind_ip:bind_port:dst_ip:dst_port，多条用换行分隔
	if cfg.PortForwards != "" {
		for _, pf := range strings.Split(cfg.PortForwards, "\n") {
			pf = strings.TrimSpace(pf)
			if pf != "" {
				args = append(args, "--port-forward", pf)
			}
		}
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
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.getBinaryPath(), args...)
	cmd.Stdout = os.Stdout
	// 使用 bytes.Buffer 捕获 stderr，同时保留输出到控制台
	var stderrBuf bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)

	if err := cmd.Start(); err != nil {
		cancel()
		m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return fmt.Errorf("启动 EasyTier 服务端失败: %w", err)
	}

	entry := &processEntry{cmd: cmd, cancel: cancel}
	m.servers.Store(id, entry)

	go func() {
		err := cmd.Wait()
		stderrOutput := stderrBuf.String()
		m.servers.Delete(id)
		if err != nil {
			errMsg := fmt.Sprintf("进程异常退出: %v", err)
			m.log.Warnf("[EasyTier服务端][%d] %s", id, errMsg)
			m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": errMsg,
			})
			// 自动重启（延迟5秒，避免快速循环崩溃）
			time.Sleep(5 * time.Second)
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
			entry.cmd.Process.Kill()
		}
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

	// 多线程运行时（服务端默认开启）
	if cfg.MultiThread {
		args = append(args, "--multi-thread")
	}

	// ===== config-server 节点模式 =====
	// 节点模式下只需传入 --config-server 地址，其余参数由 config-server 下发，不再手动配置
	if cfg.ServerMode == "config-server" && cfg.ConfigServerAddr != "" {
		for _, addr := range strings.Split(cfg.ConfigServerAddr, ",") {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				args = append(args, "--config-server", addr)
			}
		}
		// 额外参数（兜底）
		if cfg.ExtraArgs != "" {
			extraParts := strings.Fields(cfg.ExtraArgs)
			args = append(args, extraParts...)
		}
		return args
	}

	// ===== 以下为 standalone 独立模式参数 =====

	// 自定义主机名
	if cfg.Hostname != "" {
		args = append(args, "--hostname", cfg.Hostname)
	}

	listenAddr := cfg.ListenAddr
	if listenAddr == "" {
		listenAddr = "0.0.0.0"
	}

	// 监听端口（支持多个）
	// 格式：12345（基准端口）或 tcp:11010,udp:11011（多协议多端口）
	if cfg.ListenPorts != "" {
		ports := strings.Split(cfg.ListenPorts, ",")
		for _, p := range ports {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// 如果包含协议前缀（如 tcp:11010），拼接地址
			if strings.Contains(p, ":") {
				// 格式如 tcp:11010 → tcp://0.0.0.0:11010
				parts := strings.SplitN(p, ":", 2)
				args = append(args, "-l", fmt.Sprintf("%s://%s:%s", parts[0], listenAddr, parts[1]))
			} else {
				// 纯数字基准端口，直接传入
				args = append(args, "-l", p)
			}
		}
	}

	if cfg.NetworkName != "" {
		args = append(args, "--network-name", cfg.NetworkName)
	}
	if cfg.NetworkPassword != "" {
		args = append(args, "--network-secret", cfg.NetworkPassword)
	}

	// 不创建 TUN 虚拟网卡（服务端通常不需要 TUN，但保留选项）
	if cfg.NoTun {
		args = append(args, "--no-tun")
	}

	// ===== 网络行为选项 =====
	if cfg.DisableP2P {
		args = append(args, "--disable-p2p")
	}
	if cfg.EnableExitNode {
		args = append(args, "--enable-exit-node")
	}
	if cfg.RelayAllPeerRpc {
		args = append(args, "--relay-all-peer-rpc")
	}

	// ===== 协议加速选项 =====
	if cfg.EnableKcpProxy {
		args = append(args, "--enable-kcp-proxy")
	}
	if cfg.EnableQuicProxy {
		args = append(args, "--enable-quic-proxy")
	}

	// ===== 安全选项 =====
	if cfg.DisableEncryption {
		args = append(args, "--disable-encryption")
	}
	if cfg.EnablePrivateMode {
		args = append(args, "--enable-private-mode")
	}

	// ===== 中继选项 =====
	if cfg.RelayNetworkWhitelist != "" {
		args = append(args, "--relay-network-whitelist", cfg.RelayNetworkWhitelist)
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

	// 额外参数（兜底）
	if cfg.ExtraArgs != "" {
		extraParts := strings.Fields(cfg.ExtraArgs)
		args = append(args, extraParts...)
	}

	return args
}
