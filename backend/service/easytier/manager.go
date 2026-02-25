package easytier

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

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
	cmd.Stderr = os.Stderr

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
		cmd.Wait()
		m.clients.Delete(id)
		m.db.Model(&model.EasytierClient{}).Where("id = ?", id).Update("status", "stopped")
		m.log.Infof("[EasyTier客户端][%d] 进程已退出", id)
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

	// 虚拟 IP
	if cfg.VirtualIP != "" {
		args = append(args, "--ipv4", cfg.VirtualIP)
	}

	// 本地监听端口（支持多个）
	// 格式：12345（基准端口）或 tcp:11010,udp:11011（多协议多端口）
	if cfg.ListenPorts != "" {
		ports := strings.Split(cfg.ListenPorts, ",")
		for _, p := range ports {
			p = strings.TrimSpace(p)
			if p != "" {
				args = append(args, "-l", p)
			}
		}
	}

	// 额外参数
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
	cmd.Stderr = os.Stderr

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
		cmd.Wait()
		m.servers.Delete(id)
		m.db.Model(&model.EasytierServer{}).Where("id = ?", id).Update("status", "stopped")
		m.log.Infof("[EasyTier服务端][%d] 进程已退出", id)
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
	args := []string{"--multi-thread"}

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
			// 如果是纯数字（基准端口），直接传入
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

	if cfg.ExtraArgs != "" {
		extraParts := strings.Fields(cfg.ExtraArgs)
		args = append(args, extraParts...)
	}

	return args
}
