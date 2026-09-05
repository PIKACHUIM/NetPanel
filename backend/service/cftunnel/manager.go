// Package cftunnel Cloudflare Tunnel (cloudflared) 进程管理：
// 支持 quick（临时隧道）/ named（命名隧道）/ token（远程配置）三种模式，
// 通过命令行方式管理 cloudflared 进程（与 easytier 类似的进程管理方式）。
package cftunnel

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/cert/ca"
)

const (
	// maxLogLines 单个隧道保留的日志行数
	maxLogLines = 500
	// binaryName cloudflared 可执行文件名
	binaryName = "cloudflared"
	// configDirName named 模式临时配置目录
	configDirName = "cftunnel"
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
	db      *gorm.DB
	log     *logrus.Logger
	dataDir  string
	caSigner *ca.Signer
	tunnels sync.Map // map[uint]*processEntry
	stopping bool
	mu       sync.Mutex
}

// NewManager 创建管理器。dataDir 用于存放 named 模式的临时配置文件。
func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{db: db, log: log, dataDir: dataDir}
}

// getBinaryPath 返回 cloudflared 二进制路径（data/bin/cloudflared[.exe]）。
// Windows 下带 .exe 后缀，与 downloader.DownloadBinary 落盘的文件名保持一致。
func (m *Manager) getBinaryPath() string {
	name := binaryName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(m.dataDir, "bin", name)
}

// isBinaryAvailable 检查 cloudflared 二进制是否存在
func (m *Manager) isBinaryAvailable() bool {
	info, err := os.Stat(m.getBinaryPath())
	return err == nil && !info.IsDir()
}

// IsBinaryAvailable 供 API 层判断二进制是否已就绪
func (m *Manager) IsBinaryAvailable() bool {
	return m.isBinaryAvailable()
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
		_ = m.Stop(key.(uint))
		return true
	})
}

// Start 启动指定隧道
func (m *Manager) Start(id uint) error {
	_ = m.Stop(id)

	if !m.isBinaryAvailable() {
		return fmt.Errorf("cloudflared 二进制不存在，请先下载: %s", m.getBinaryPath())
	}

	var cfg model.CftunnelConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("CF 隧道配置不存在: %w", err)
	}

	args, extraEnv, err := m.buildArgs(&cfg)
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
	if len(extraEnv) > 0 {
		// token 等敏感凭据经环境变量传入，不出现在进程命令行中
		cmd.Env = append(os.Environ(), extraEnv...)
	}

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
		defer m.log.Debugf("[CF隧道][%d] stdout 监听已退出", id)
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			logBuf.write(line)
			// quick 模式：隧道地址每次启动随机生成，从日志中提取并落库，
			// 前端可直接展示可访问入口，无需翻日志
			if cfg.Mode == "quick" {
				if url := quickURLRe.FindString(line); url != "" {
					m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Update("quick_url", url)
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
			m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": errMsg,
				"quick_url":  "",
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
			m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":    "stopped",
				"quick_url": "",
			})
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

// Stop 停止指定隧道。进程未运行时仅同步数据库状态，不视为错误。
func (m *Manager) Stop(id uint) error {
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
	return m.db.Model(&model.CftunnelConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":    "stopped",
		"quick_url": "",
	}).Error
}

// Restart 重启指定隧道
func (m *Manager) Restart(id uint) error {
	_ = m.Stop(id)
	return m.Start(id)
}

// GetStatus 返回隧道运行状态详情（running/status/pid/quick_url/last_error）。
// 返回的 map 始终反映进程的真实内存状态；error 仅表示数据库配置记录缺失，
// 供 API 层回 404 使用，监控等只关心运行状态的调用方可忽略该 error。
func (m *Manager) GetStatus(id uint) (map[string]any, error) {
	st := map[string]any{
		"id":      id,
		"running": false,
		"status":  "stopped",
	}
	if val, ok := m.tunnels.Load(id); ok {
		st["running"] = true
		st["status"] = "running"
		if entry, ok := val.(*processEntry); ok && entry.cmd != nil && entry.cmd.Process != nil {
			st["pid"] = entry.cmd.Process.Pid
		}
	}

	var cfg model.CftunnelConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return st, fmt.Errorf("CF 隧道配置不存在: %w", err)
	}
	st["quick_url"] = cfg.QuickURL
	st["last_error"] = cfg.LastError
	return st, nil
}

// GetLogs 返回隧道日志
func (m *Manager) GetLogs(id uint) []string {
	if val, ok := m.tunnels.Load(id); ok {
		return val.(*processEntry).logs.lines()
	}
	return nil
}

// buildArgs 根据模式构建 cloudflared 命令行参数与额外环境变量。
//
//	quick:  cloudflared tunnel --url <local_url> --no-autoupdate
//	named:  cloudflared tunnel --config <config.yml> run <name|uuid> --no-autoupdate
//	token:  cloudflared tunnel run --no-autoupdate  （token 经 TUNNEL_TOKEN 环境变量传入）
//
// 安全约束：
//   - token 不作为命令行参数传递，避免在 ps / /proc/<pid>/cmdline 中泄露凭据；
//   - TunnelName 需通过白名单校验，防止被当作 cloudflared flag 解析（flag 注入）。
func (m *Manager) buildArgs(cfg *model.CftunnelConfig) (args []string, env []string, err error) {
	switch cfg.Mode {
	case "quick":
		if cfg.LocalURL == "" {
			return nil, nil, fmt.Errorf("quick 模式需要填写本地服务地址（LocalURL）")
		}
		if err := validateLocalURL(cfg.LocalURL); err != nil {
			return nil, nil, err
		}
		return []string{"tunnel", "--url", cfg.LocalURL, "--no-autoupdate"}, nil, nil

	case "named":
		if cfg.TunnelName == "" {
			return nil, nil, fmt.Errorf("named 模式需要填写隧道名称或 UUID")
		}
		if err := ValidateTunnelName(cfg.TunnelName); err != nil {
			return nil, nil, err
		}
		out := []string{"tunnel"}
		configPath := cfg.ConfigFile
		if configPath == "" {
			// 自动生成临时 config.yml（若凭据文件已提供）
			if cfg.CredentialsFile != "" {
				generated, genErr := m.writeTempConfig(cfg)
				if genErr != nil {
					return nil, nil, genErr
				}
				configPath = generated
			}
		}
		if configPath != "" {
			out = append(out, "--config", configPath)
		}
		out = append(out, "run", cfg.TunnelName, "--no-autoupdate")
		return out, nil, nil

	case "token":
		if cfg.Token == "" {
			return nil, nil, fmt.Errorf("token 模式需要填写 Token")
		}
		// Token 经环境变量传入，不进入 argv
		return []string{"tunnel", "run", "--no-autoupdate"},
			[]string{"TUNNEL_TOKEN=" + cfg.Token}, nil

	default:
		return nil, nil, fmt.Errorf("未知模式: %q（可选 quick/named/token）", cfg.Mode)
	}
}

// tunnelNameRe 隧道名称/UUID 白名单：字母数字开头，允许 . _ - ，长度 1-63。
// 作用是阻断以 - 开头被 cloudflared 误解析为 flag，以及含换行导致的 YAML 注入。
var tunnelNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,62}$`)

// ValidateTunnelName 校验隧道名称是否合法（供 API 层前置校验复用）。
func ValidateTunnelName(name string) error {
	if !tunnelNameRe.MatchString(name) {
		return fmt.Errorf("隧道名称非法：仅允许字母、数字、下划线、点、连字符，且需以字母或数字开头（长度 1-63）")
	}
	return nil
}

// validateLocalURL 校验 quick 模式的本地服务地址，阻断以 - 开头的 flag 注入。
func validateLocalURL(raw string) error {
	if strings.HasPrefix(raw, "-") {
		return fmt.Errorf("本地服务地址非法：不能以 - 开头")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("本地服务地址解析失败: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "tcp" {
		return fmt.Errorf("本地服务地址仅支持 http/https/tcp 协议")
	}
	if u.Host == "" {
		return fmt.Errorf("本地服务地址缺少主机名")
	}
	return nil
}

// tunnelConfigFile named 模式生成的 config.yml 结构，交由 yaml 库序列化，
// 避免手工拼接字符串导致换行注入任意配置键。
type tunnelConfigFile struct {
	Tunnel          string `yaml:"tunnel"`
	CredentialsFile string `yaml:"credentials-file"`
}

// writeTempConfig 为 named 模式生成临时 config.yml（写入 dataDir/cftunnel/<id>.yml）
func (m *Manager) writeTempConfig(cfg *model.CftunnelConfig) (string, error) {
	credPath, err := m.resolveCredentialsFile(cfg.CredentialsFile)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(m.dataDir, configDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}
	path := filepath.Join(dir, fmt.Sprintf("tunnel-%d.yml", cfg.ID))

	content, err := yaml.Marshal(tunnelConfigFile{
		Tunnel:          cfg.TunnelName,
		CredentialsFile: credPath,
	})
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %w", err)
	}
	// 配置内含凭据文件路径，限制为仅属主可读写
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return "", fmt.Errorf("写入配置文件失败: %w", err)
	}
	return path, nil
}

// resolveCredentialsFile 校验凭据文件路径：必须位于数据目录内，防止路径穿越
// 导致 cloudflared 读取宿主机任意文件（如 /root/.ssh/id_rsa）。
func (m *Manager) resolveCredentialsFile(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("凭据文件路径为空")
	}
	abs, err := filepath.Abs(raw)
	if err != nil {
		return "", fmt.Errorf("解析凭据文件路径失败: %w", err)
	}
	baseAbs, err := filepath.Abs(m.dataDir)
	if err != nil {
		return "", fmt.Errorf("解析数据目录失败: %w", err)
	}
	rel, err := filepath.Rel(baseAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("凭据文件必须位于数据目录内: %s", baseAbs)
	}
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		return "", fmt.Errorf("凭据文件不存在或不是文件: %s", abs)
	}
	return abs, nil
}

// CASigner 返回 CA 签发器（用于 enrollment）
func (m *Manager) CASigner() *ca.Signer {
	return m.caSigner
}
