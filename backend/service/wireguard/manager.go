package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
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
	"golang.org/x/crypto/curve25519"
	"gorm.io/gorm"
)

// Manager WireGuard 管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	dataDir string
	running sync.Map // map[uint]bool — 正在运行的接口 ID
	mu      sync.Mutex
}

// NewManager 创建 WireGuard 管理器
func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	if absDir, err := filepath.Abs(dataDir); err == nil {
		dataDir = absDir
	}
	return &Manager{db: db, log: log, dataDir: dataDir}
}

// GenerateKeyPair 生成 WireGuard 密钥对（私钥 + 公钥）
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	var privKey [32]byte
	if _, err = rand.Read(privKey[:]); err != nil {
		return "", "", fmt.Errorf("生成随机私钥失败: %w", err)
	}
	// Curve25519 clamping
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	var pubKey [32]byte
	curve25519.ScalarBaseMult(&pubKey, &privKey)

	privateKey = base64.StdEncoding.EncodeToString(privKey[:])
	publicKey = base64.StdEncoding.EncodeToString(pubKey[:])
	return
}

// getConfDir 获取 WireGuard 配置文件目录
func (m *Manager) getConfDir() string {
	dir := filepath.Join(m.dataDir, "wireguard")
	_ = os.MkdirAll(dir, 0700)
	return dir
}

// getConfPath 获取指定接口的配置文件路径
func (m *Manager) getConfPath(id uint) string {
	return filepath.Join(m.getConfDir(), fmt.Sprintf("wg%d.conf", id))
}

// getInterfaceName 获取接口名称
func (m *Manager) getInterfaceName(id uint) string {
	return fmt.Sprintf("wg%d", id)
}

// StartAll 启动所有已启用的 WireGuard 接口
func (m *Manager) StartAll() {
	go func() {
		var configs []model.WireguardConfig
		m.db.Where("enable = ?", true).Find(&configs)
		for _, cfg := range configs {
			cfg := cfg
			go func() {
				if err := m.Start(cfg.ID); err != nil {
					m.log.Errorf("[WireGuard][%s] 启动失败: %v", cfg.Name, err)
				}
			}()
		}
	}()
}

// StopAll 停止所有运行中的 WireGuard 接口
func (m *Manager) StopAll() {
	var wg sync.WaitGroup
	m.running.Range(func(key, value interface{}) bool {
		id := key.(uint)
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Stop(id)
		}()
		return true
	})
	wg.Wait()
}

// Start 启动指定的 WireGuard 接口
func (m *Manager) Start(id uint) error {
	m.Stop(id)

	var cfg model.WireguardConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("WireGuard 配置不存在: %w", err)
	}

	// 生成配置文件
	if err := m.generateConfig(id); err != nil {
		m.db.Model(&model.WireguardConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": err.Error(),
		})
		return fmt.Errorf("生成配置文件失败: %w", err)
	}

	confPath := m.getConfPath(id)
	ifName := m.getInterfaceName(id)

	// 使用 wg-quick 启动接口
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows 上使用 wireguard.exe 命令
		cmd = exec.Command("wireguard.exe", "/installtunnelservice", confPath)
	} else {
		cmd = exec.Command("wg-quick", "up", confPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("启动失败: %v, 输出: %s", err, string(output))
		m.log.Errorf("[WireGuard][%s] %s", ifName, errMsg)
		m.db.Model(&model.WireguardConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": errMsg,
		})
		return errors.New(errMsg)
	}

	m.running.Store(id, true)
	m.db.Model(&model.WireguardConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[WireGuard][%s] 已启动", ifName)
	return nil
}

// Stop 停止指定的 WireGuard 接口
func (m *Manager) Stop(id uint) {
	if _, ok := m.running.Load(id); !ok {
		m.db.Model(&model.WireguardConfig{}).Where("id = ?", id).Update("status", "stopped")
		return
	}

	confPath := m.getConfPath(id)
	ifName := m.getInterfaceName(id)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("wireguard.exe", "/uninstalltunnelservice", ifName)
	} else {
		cmd = exec.Command("wg-quick", "down", confPath)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		m.log.Warnf("[WireGuard][%s] 停止时出错: %v, 输出: %s", ifName, err, string(output))
	}

	m.running.Delete(id)
	m.db.Model(&model.WireguardConfig{}).Where("id = ?", id).Update("status", "stopped")
	m.log.Infof("[WireGuard][%s] 已停止", ifName)
}

// GetStatus 获取接口运行状态
func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.running.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// generateConfig 生成 WireGuard 配置文件
func (m *Manager) generateConfig(id uint) error {
	var cfg model.WireguardConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return err
	}

	var peers []model.WireguardPeer
	m.db.Where("wireguard_id = ? AND enable = ?", id, true).Find(&peers)

	var sb strings.Builder

	// [Interface] 段
	sb.WriteString("[Interface]\n")
	if cfg.PrivateKey != "" {
		sb.WriteString(fmt.Sprintf("PrivateKey = %s\n", cfg.PrivateKey))
	}
	if cfg.Address != "" {
		sb.WriteString(fmt.Sprintf("Address = %s\n", cfg.Address))
	}
	if cfg.ListenPort > 0 {
		sb.WriteString(fmt.Sprintf("ListenPort = %d\n", cfg.ListenPort))
	}
	if cfg.DNS != "" {
		sb.WriteString(fmt.Sprintf("DNS = %s\n", cfg.DNS))
	}
	if cfg.MTU > 0 {
		sb.WriteString(fmt.Sprintf("MTU = %d\n", cfg.MTU))
	}
	if cfg.Table != "" {
		sb.WriteString(fmt.Sprintf("Table = %s\n", cfg.Table))
	}
	if cfg.PreUp != "" {
		sb.WriteString(fmt.Sprintf("PreUp = %s\n", cfg.PreUp))
	}
	if cfg.PostUp != "" {
		sb.WriteString(fmt.Sprintf("PostUp = %s\n", cfg.PostUp))
	}
	if cfg.PreDown != "" {
		sb.WriteString(fmt.Sprintf("PreDown = %s\n", cfg.PreDown))
	}
	if cfg.PostDown != "" {
		sb.WriteString(fmt.Sprintf("PostDown = %s\n", cfg.PostDown))
	}

	// [Peer] 段
	for _, peer := range peers {
		sb.WriteString("\n[Peer]\n")
		if peer.PublicKey != "" {
			sb.WriteString(fmt.Sprintf("PublicKey = %s\n", peer.PublicKey))
		}
		if peer.PresharedKey != "" {
			sb.WriteString(fmt.Sprintf("PresharedKey = %s\n", peer.PresharedKey))
		}
		if peer.Endpoint != "" {
			sb.WriteString(fmt.Sprintf("Endpoint = %s\n", peer.Endpoint))
		}
		if peer.AllowedIPs != "" {
			sb.WriteString(fmt.Sprintf("AllowedIPs = %s\n", peer.AllowedIPs))
		}
		if peer.PersistentKeepalive > 0 {
			sb.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
		}
	}

	confPath := m.getConfPath(id)
	return os.WriteFile(confPath, []byte(sb.String()), 0600)
}

// ReloadConfig 重新加载配置（停止后重新启动）
func (m *Manager) ReloadConfig(id uint) error {
	if _, ok := m.running.Load(id); ok {
		m.Stop(id)
		// 等待一小段时间确保接口完全关闭
		time.Sleep(500 * time.Millisecond)
		return m.Start(id)
	}
	return nil
}
