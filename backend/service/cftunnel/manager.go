// Package cftunnel Cloudflare Tunnel (cloudflared) 进程管理：
// 支持 quick（临时隧道）/ named（命名隧道）/ token（远程配置）三种模式，
// 通过命令行方式管理 cloudflared 进程（与 easytier 类似的进程管理方式）。
package cftunnel

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

const (
	// maxLogLines 单个隧道保留的日志行数
	maxLogLines = 500
	// binaryName cloudflared 可执行文件名
	binaryName = "cloudflared"
	// configDirName named 模式临时配置目录
	configDirName = "cftunnel"
)

// processEntry 单个 cloudflared 进程
type processEntry struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{} // 进程退出后关闭
	logs   *ringBuffer
}

// ringBuffer 环形日志缓冲区（与 easytier 相同实现）
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

func (r *ringBuffer) write(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buf[r.pos] = line
	r.pos = (r.pos + 1) % r.size
	if r.pos == 0 {
		r.full = true
	}
}

func (r *ringBuffer) lines() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, r.size)
	if r.full {
		out = append(out, r.buf[r.pos:]...)
		out = append(out, r.buf[:r.pos]...)
	} else {
		out = append(out, r.buf[:r.pos]...)
	}
	return out
}

// Manager Cloudflare Tunnel 管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	dataDir string
	tunnels sync.Map // map[uint]*processEntry
	stopping bool
	mu       sync.Mutex
}

// NewManager 创建管理器。dataDir 用于存放 named 模式的临时配置文件。
func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{db: db, log: log, dataDir: dataDir}
}

// getBinaryPath 返回 cloudflared 二进制路径（data/bin/cloudflared）
func (m *Manager) getBinaryPath() string {
	return filepath.Join(m.dataDir, "bin", binaryName)
}

// isBinaryAvailable 检查 cloudflared 二进制是否存在
func (m *Manager) isBinaryAvailable() bool {
	info, err := os.Stat(m.getBinaryPath())
	return err == nil && !info.IsDir()
}

// GetBinaryPath 供前端/API 展示二进制路径
func (m *Manager) GetBinaryPath() string {
	return m.getBinaryPath()
}

// GetBinDir 返回 bin 目录路径（用于下载）
func (m *Manager) GetBinDir() string {
	return filepath.Join(m.dataDir, "bin")
}

// StartAll 启动所有启用状态的隧道
func (m *Manager) StartAll() {
	var tunnels []model.CftunnelConfig
	if err := m.db.Where("enable = ?", true).Find(&tunnels).Error; err != nil {
		m.log.Warnf("[CF隧道] 读取配置失败: %v", err)
		return
	}
	for _, t := range tunnels {
		if err := m.Start(t.ID); err != nil {
			m.log.Warnf("[CF隧道][%d] 启动失败: %v", t.ID, err)
		}
	}
}

// StopAll 停止所有隧道
func (m *Manager) StopAll() {
	m.mu.Lock()
	m.stopping = true
	m.mu.Unlock()
	m.tunnels.Range(func(key, _ interface{}) bool {
		m.Stop(key.(uint))
		return true
	})
}

// Start 启动指定隧道
func (m *Manager) Start(id uint) error {
	m.Stop(id)

	if !m.isBinaryAvailable() {
		return fmt.Errorf("cloudflared 二进制不存在，请先下载: %s", m.getBinaryPath())
	}

	var cfg model.CftunnelConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("CF 隧道配置不存在: %w", err)
	}

	args, err := m.buildArgs(&cfg)
	if err != nil {
		m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, m.getBinaryPath(), args...)
	cmd.Dir = filepath.Dir(m.getBinaryPath())

	logBuf := newRingBuffer(maxLogLines)
	var stderrBuf bytes.Buffer

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		cancel()
		m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return fmt.Errorf("启动 CF 隧道失败: %w", err)
	}

	entry := &processEntry{cmd: cmd, cancel: cancel, done: make(chan struct{}), logs: logBuf}
	m.tunnels.Store(id, entry)

	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			logBuf.write(scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logBuf.write("[stderr] " + line)
			stderrBuf.WriteString(line + "\n")
		}
	}()

	go func() {
		err := cmd.Wait()
		close(entry.done)
		m.tunnels.Delete(id)
		_ = stderrBuf.String()
		if err != nil {
			errMsg := fmt.Sprintf("进程异常退出: %v", err)
			m.log.Warnf("[CF隧道][%d] %s", id, errMsg)
			m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": errMsg,
			})
			// 自动重启（延迟 5 秒，关闭期间不重启）
			time.Sleep(5 * time.Second)
			m.mu.Lock()
			isStopping := m.stopping
			m.mu.Unlock()
			if isStopping {
				return
			}
			var cur model.CftunnelConfig
			if m.db.First(&cur, id).Error == nil && cur.Enable {
				m.log.Infof("[CF隧道][%d] 尝试自动重启...", id)
				if restartErr := m.Start(id); restartErr != nil {
					m.log.Errorf("[CF隧道][%d] 自动重启失败: %v", id, restartErr)
				}
			}
		} else {
			m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Update("status", "stopped")
			m.log.Infof("[CF隧道][%d] 进程已退出", id)
		}
	}()

	m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[CF隧道][%d] 已启动，PID: %d", id, cmd.Process.Pid)
	return nil
}

// Stop 停止指定隧道
func (m *Manager) Stop(id uint) {
	if val, ok := m.tunnels.Load(id); ok {
		entry := val.(*processEntry)
		entry.cancel()
		if entry.cmd.Process != nil {
			_ = entry.cmd.Process.Kill()
		}
		<-entry.done
		m.tunnels.Delete(id)
	}
	m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Update("status", "stopped")
}

// Restart 重启指定隧道
func (m *Manager) Restart(id uint) error {
	m.Stop(id)
	return m.Start(id)
}

// GetStatus 返回隧道运行状态
func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.tunnels.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// GetLogs 返回隧道日志
func (m *Manager) GetLogs(id uint) []string {
	if val, ok := m.tunnels.Load(id); ok {
		return val.(*processEntry).logs.lines()
	}
	return nil
}

// buildArgs 根据模式构建 cloudflared 命令行参数。
//
//	quick:  cloudflared tunnel --url <local_url> --no-autoupdate
//	named:  cloudflared tunnel --config <config.yml> run <name|uuid> --no-autoupdate
//	token:  cloudflared tunnel run --token <token> --no-autoupdate
func (m *Manager) buildArgs(cfg *model.CftunnelConfig) ([]string, error) {
	switch cfg.Mode {
	case "quick":
		if cfg.LocalURL == "" {
			return nil, fmt.Errorf("quick 模式需要填写本地服务地址（LocalURL）")
		}
		return []string{"tunnel", "--url", cfg.LocalURL, "--no-autoupdate"}, nil

	case "named":
		if cfg.TunnelName == "" {
			return nil, fmt.Errorf("named 模式需要填写隧道名称或 UUID")
		}
		args := []string{"tunnel"}
		configPath := cfg.ConfigFile
		if configPath == "" {
			// 自动生成临时 config.yml（若凭据文件已提供）
			if cfg.CredentialsFile != "" {
				generated, err := m.writeTempConfig(cfg)
				if err != nil {
					return nil, err
				}
				configPath = generated
			}
		}
		if configPath != "" {
			args = append(args, "--config", configPath)
		}
		args = append(args, "run", cfg.TunnelName, "--no-autoupdate")
		return args, nil

	case "token":
		if cfg.Token == "" {
			return nil, fmt.Errorf("token 模式需要填写 Token")
		}
		return []string{"tunnel", "run", "--token", cfg.Token, "--no-autoupdate"}, nil

	default:
		return nil, fmt.Errorf("未知模式: %q（可选 quick/named/token）", cfg.Mode)
	}
}

// writeTempConfig 为 named 模式生成临时 config.yml（写入 dataDir/cftunnel/<name>.yml）
func (m *Manager) writeTempConfig(cfg *model.CftunnelConfig) (string, error) {
	dir := filepath.Join(m.dataDir, configDirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("tunnel-%d.yml", cfg.ID))
	content := fmt.Sprintf("tunnel: %s\ncredentials-file: %s\n", cfg.TunnelName, cfg.CredentialsFile)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入配置文件失败: %w", err)
	}
	return path, nil
}
