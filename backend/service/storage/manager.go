package storage

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/netpanel/netpanel/model"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/webdav"
	"gorm.io/gorm"
)

type storageEntry struct {
	listener net.Listener
	server   *http.Server
	// SFTP 专用
	sshListener net.Listener
}

// Manager 网络存储管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	entries sync.Map // map[uint]*storageEntry
	dataDir string
}

func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{db: db, log: log, dataDir: dataDir}
}

func (m *Manager) StartAll() {
	var configs []model.StorageConfig
	m.db.Where("enable = ?", true).Find(&configs)
	for _, c := range configs {
		if err := m.Start(c.ID); err != nil {
			m.log.Errorf("网络存储 [%s] 启动失败: %v", c.Name, err)
		}
	}
}

func (m *Manager) StopAll() {
	m.entries.Range(func(key, value interface{}) bool {
		m.Stop(key.(uint))
		return true
	})
}

func (m *Manager) Start(id uint) error {
	m.Stop(id)

	var cfg model.StorageConfig
	if err := m.db.First(&cfg, id).Error; err != nil {
		return fmt.Errorf("存储配置不存在: %w", err)
	}

	switch cfg.Protocol {
	case "webdav":
		return m.startWebDAV(id, &cfg)
	case "sftp":
		return m.startSFTP(id, &cfg)
	case "smb":
		return m.startSMB(id, &cfg)
	default:
		return fmt.Errorf("不支持的协议: %s", cfg.Protocol)
	}
}

func (m *Manager) startWebDAV(id uint, cfg *model.StorageConfig) error {
	handler := &webdav.Handler{
		FileSystem: webdav.Dir(cfg.RootPath),
		LockSystem: webdav.NewMemLS(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 基础认证
		if cfg.Username != "" {
			user, pass, ok := r.BasicAuth()
			if !ok || user != cfg.Username || pass != cfg.Password {
				w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		handler.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.ListenPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("WebDAV 监听 %s 失败: %w", addr, err)
	}

	srv := &http.Server{Handler: mux}
	entry := &storageEntry{listener: ln, server: srv}
	m.entries.Store(id, entry)

	go func() {
		srv.Serve(ln)
		m.entries.Delete(id)
		m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("status", "stopped")
	}()

	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[WebDAV][%d] 已启动，监听 %s，根目录: %s", id, addr, cfg.RootPath)
	return nil
}

// startSFTP 启动 SFTP 服务（基于 SSH）
func (m *Manager) startSFTP(id uint, cfg *model.StorageConfig) error {
	// 获取或生成 SSH 主机密钥
	hostKey, err := m.getOrCreateHostKey(id)
	if err != nil {
		return fmt.Errorf("获取 SSH 主机密钥失败: %w", err)
	}

	// 配置 SSH 服务器
	sshConfig := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if cfg.Username == "" {
				// 未设置用户名，允许任意登录
				return nil, nil
			}
			if c.User() == cfg.Username && string(pass) == cfg.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("用户名或密码错误")
		},
	}
	sshConfig.AddHostKey(hostKey)

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.ListenPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("SFTP 监听 %s 失败: %w", addr, err)
	}

	entry := &storageEntry{sshListener: ln}
	m.entries.Store(id, entry)

	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[SFTP][%d] 已启动，监听 %s，根目录: %s", id, addr, cfg.RootPath)

	go func() {
		defer func() {
			m.entries.Delete(id)
			m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("status", "stopped")
		}()

		for {
			conn, err := ln.Accept()
			if err != nil {
				// 监听器被关闭，正常退出
				return
			}
			go m.handleSFTPConn(conn, sshConfig, cfg.RootPath)
		}
	}()

	return nil
}

// handleSFTPConn 处理单个 SFTP 连接
func (m *Manager) handleSFTPConn(conn net.Conn, config *ssh.ServerConfig, rootPath string) {
	defer conn.Close()

	// SSH 握手
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		m.log.Debugf("[SFTP] SSH 握手失败: %v", err)
		return
	}
	defer sshConn.Close()

	// 丢弃全局请求
	go ssh.DiscardRequests(reqs)

	// 处理 channel
	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		ch, requests, err := newChan.Accept()
		if err != nil {
			m.log.Debugf("[SFTP] 接受 channel 失败: %v", err)
			return
		}

		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := req.Type == "subsystem" && len(req.Payload) >= 4
				if ok {
					subsystem := string(req.Payload[4:])
					ok = subsystem == "sftp"
				}
				req.Reply(ok, nil)
			}
		}(requests)

		// 启动 SFTP 服务
		server, err := sftp.NewServer(ch,
			sftp.WithServerWorkingDirectory(rootPath),
		)
		if err != nil {
			m.log.Errorf("[SFTP] 创建 SFTP 服务失败: %v", err)
			ch.Close()
			return
		}

		if err := server.Serve(); err != nil && err != io.EOF {
			m.log.Debugf("[SFTP] 服务结束: %v", err)
		}
		server.Close()
	}
}

// getOrCreateHostKey 获取或生成 SSH 主机密钥
func (m *Manager) getOrCreateHostKey(id uint) (ssh.Signer, error) {
	keyPath := fmt.Sprintf("%s/sftp_%d_host.key", m.dataDir, id)

	// 尝试读取已有密钥
	if keyData, err := os.ReadFile(keyPath); err == nil {
		block, _ := pem.Decode(keyData)
		if block != nil {
			key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err == nil {
				return ssh.NewSignerFromKey(key)
			}
		}
	}

	// 生成新密钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成 RSA 密钥失败: %w", err)
	}

	// 保存密钥到文件
	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		m.log.Warnf("[SFTP] 保存主机密钥失败: %v", err)
	}

	return ssh.NewSignerFromKey(privateKey)
}

// startSMB 启动 SMB 服务（暂不支持，需要系统级 samba）
func (m *Manager) startSMB(id uint, cfg *model.StorageConfig) error {
	errMsg := "SMB 协议需要系统安装 Samba，请手动配置 /etc/samba/smb.conf"
	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": errMsg,
	})
	return fmt.Errorf("%s", errMsg)
}

func (m *Manager) Stop(id uint) {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*storageEntry)
		if entry.server != nil {
			entry.server.Close()
		}
		if entry.listener != nil {
			entry.listener.Close()
		}
		if entry.sshListener != nil {
			entry.sshListener.Close()
		}
		m.entries.Delete(id)
	}
	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.entries.Load(id); ok {
		return "running"
	}
	return "stopped"
}
