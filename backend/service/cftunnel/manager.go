// Package cftunnel Cloudflare Tunnel (cloudflared) 进程管理：
// 支持 quick（临时隧道）/ named（命名隧道，Token 认证）两种模式，
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
	"regexp"
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
)

// quickURLRe 匹配 cloudflared quick 模式日志中的临时隧道地址，
// 形如 https://<random>.trycloudflare.com
var quickURLRe = regexp.MustCompile(`(?i)https://[a-z0-9-]+\.trycloudflare\.com(?:\s|$)`)

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
	db       *gorm.DB
	log      *logrus.Logger
	dataDir  string
	tunnels  sync.Map // map[uint]*processEntry
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
	var tunnels []model.CloudflareTunnel
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

	var cfg model.CloudflareTunnel
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("CF 隧道配置不存在: %w", err)
	}

	args, err := m.buildArgs(&cfg)
	if err != nil {
		m.db.Model(&model.CloudflareTunnel{}).Where("id = ?", id).Updates(map[string]interface{}{
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
		m.db.Model(&model.CloudflareTunnel{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return fmt.Errorf("启动 CF 隧道失败: %w", err)
	}

	entry := &processEntry{cmd: cmd, cancel: cancel, done: make(chan struct{}), logs: logBuf}
	m.tunnels.Store(id, entry)

	go func() {
		defer m.log.Debugf("[CF隧道][%d] stdout 监听已退出", id)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logBuf.write(line)
			// quick 模式：隧道地址每次启动随机生成，从日志中提取并落库，
			// 前端可直接展示可访问入口，无需翻日志
			if cfg.Type == "quick" {
				if url := quickURLRe.FindString(line); url != "" {
					m.db.Model(&model.CloudflareTunnel{}).Where("id = ?", id).Update("public_url", url)
					m.log.Infof("[CF隧道][%d] quick 入口已更新: %s", id, url)
				}
			}
		}
	}()
	go func() {
		defer m.log.Debugf("[CF隧道][%d] stderr 监听已退出", id)
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
			m.db.Model(&model.CloudflareTunnel{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": errMsg,
				"public_url": "",
			})
			// 自动重启（延迟 5 秒，关闭期间不重启）
			time.Sleep(5 * time.Second)
			m.mu.Lock()
			isStopping := m.stopping
			m.mu.Unlock()
			if isStopping {
				return
			}
			var cur model.CloudflareTunnel
			if m.db.First(&cur, id).Error == nil && cur.Enable {
				m.log.Infof("[CF隧道][%d] 尝试自动重启...", id)
				if restartErr := m.Start(id); restartErr != nil {
					m.log.Errorf("[CF隧道][%d] 自动重启失败: %v", id, restartErr)
				}
			}
		} else {
			m.db.Model(&model.CloudflareTunnel{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "stopped",
				"public_url": "",
			})
			m.log.Infof("[CF隧道][%d] 进程已退出", id)
		}
	}()

	m.db.Model(&model.CloudflareTunnel{}).Where("id = ?", id).Updates(map[string]interface{}{
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
	// 进程停止后 quick 隧道地址随即失效，一并清理
	m.db.Model(&model.CloudflareTunnel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "stopped",
		"public_url": "",
	})
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
//	named:  cloudflared tunnel run --token <token> --no-autoupdate
func (m *Manager) buildArgs(cfg *model.CloudflareTunnel) ([]string, error) {
	switch cfg.Type {
	case "quick":
		if cfg.LocalURL == "" {
			return nil, fmt.Errorf("quick 模式需要填写本地服务地址（LocalURL）")
		}
		return []string{"tunnel", "--url", cfg.LocalURL, "--no-autoupdate"}, nil

	case "named":
		// 命名隧道统一走 Token 认证（与 API 层 Create/Update 校验一致）
		if cfg.Token == "" {
			return nil, fmt.Errorf("named 模式需要填写 Cloudflare Tunnel Token")
		}
		return []string{"tunnel", "run", "--token", cfg.Token, "--no-autoupdate"}, nil

	default:
		return nil, fmt.Errorf("未知模式: %q（可选 quick/named）", cfg.Type)
	}
}
