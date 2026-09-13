package storage

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/netpanel/netpanel/model"
	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/webdav"
	"gorm.io/gorm"
)

// constantTimeEqual 常量时间字符串比较，避免通过响应时间侧信道推断凭据。
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

type storageEntry struct {
	listener net.Listener
	server   *http.Server
	// SFTP 专用
	sshListener net.Listener
	// SMB 专用
	smbConfPath string // Samba 配置文件路径
	smbPidFile  string // smbd PID 文件路径
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

	// 前置校验：网络存储以面板进程权限（通常 root/Administrator）读写目标目录，
	// 因此必须确保根目录真实存在且为目录，并且配置了访问凭据。
	// 校验放在 Start 这一唯一入口，保证 API 与开机恢复路径都受约束。
	// ValidateConfig 为纵深防御：即使 handler 侧漏校验，启动前也会被拦下。
	if err := ValidateConfig(&cfg); err != nil {
		return m.failStart(id, err)
	}
	rootPath, err := ValidateRootPath(cfg.RootPath)
	if err != nil {
		return m.failStart(id, err)
	}
	if err := ValidateCredentials(cfg.Username, cfg.Password.String(), cfg.Protocol); err != nil {
		return m.failStart(id, err)
	}
	if err := ValidateListenAddr(cfg.ListenAddr); err != nil {
		return m.failStart(id, err)
	}
	if err := ValidateListenPort(cfg.ListenPort); err != nil {
		return m.failStart(id, err)
	}
	// 使用规范化（符号链接已解析）后的绝对路径
	cfg.RootPath = rootPath

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

// failStart 记录启动失败原因并同步到数据库状态，返回原始错误。
func (m *Manager) failStart(id uint, err error) error {
	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": err.Error(),
	})
	m.log.Warnf("[网络存储][%d] 启动被拒绝: %v", id, err)
	return err
}

func (m *Manager) startWebDAV(id uint, cfg *model.StorageConfig) error {
	handler := &webdav.Handler{
		FileSystem: webdav.Dir(cfg.RootPath),
		LockSystem: webdav.NewMemLS(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// 基础认证。凭据已由 ValidateCredentials 保证非空；
		// 比较使用常量时间函数，避免通过响应时间侧信道逐字节推断口令。
		user, pass, ok := r.BasicAuth()
		if !ok || !constantTimeEqual(user, cfg.Username) || !constantTimeEqual(pass, cfg.Password.String()) {
			w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
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

	// 配置 SSH 服务器。
	// 凭据已由 ValidateCredentials 保证非空；原实现在用户名为空时直接放行任意
	// 登录（连口令都不校验），等于把共享目录以匿名 SFTP 暴露出去。
	sshConfig := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// 匿名放行已移除：SFTP 无文件系统沙箱，匿名等于向局域网开放
			// 面板权限的整个磁盘；ValidateConfig 强制要求用户名密码。
			// 比较使用常量时间函数，避免时间侧信道逐字节推断口令。
			if constantTimeEqual(c.User(), cfg.Username) && constantTimeEqual(string(pass), cfg.Password.String()) {
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

		// 等待 subsystem 请求，确认是 sftp 后再启动服务
		go func() {
			defer ch.Close()

			// 等待 sftp subsystem 请求
			ok := false
			for req := range requests {
				if req.Type == "subsystem" && len(req.Payload) >= 4 {
					subsystem := string(req.Payload[4:])
					if subsystem == "sftp" {
						req.Reply(true, nil)
						ok = true
						break
					}
				}
				// 非 sftp subsystem 请求，拒绝
				if req.WantReply {
					req.Reply(false, nil)
				}
			}
			if !ok {
				m.log.Debugf("[SFTP] 未收到 sftp subsystem 请求")
				return
			}

			// 丢弃后续请求
			go func() {
				for req := range requests {
					if req.WantReply {
						req.Reply(false, nil)
					}
				}
			}()

			// 启动 SFTP 服务
			server, err := sftp.NewServer(ch,
				sftp.WithServerWorkingDirectory(rootPath),
			)
			if err != nil {
				m.log.Errorf("[SFTP] 创建 SFTP 服务失败: %v", err)
				return
			}

			if err := server.Serve(); err != nil && err != io.EOF {
				m.log.Debugf("[SFTP] 服务结束: %v", err)
			}
			server.Close()
		}()
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

// startSMB 启动 SMB 服务（通过管理系统 Samba 配置实现）
func (m *Manager) startSMB(id uint, cfg *model.StorageConfig) error {
	// 检查 smbd 是否可用
	if _, err := exec.LookPath("smbd"); err != nil {
		errMsg := "SMB 协议需要系统安装 Samba（未找到 smbd 命令），请先安装: apt install samba"
		m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": errMsg,
		})
		return fmt.Errorf("%s", errMsg)
	}

	// 用户名会同时进入 smb.conf 和 useradd/smbpasswd 的参数列表：
	// 含换行可注入任意 smb.conf 指令，以 '-' 开头会被 useradd 当作选项解析
	if cfg.Username != "" {
		if err := validateSystemUsername(cfg.Username); err != nil {
			m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": err.Error(),
			})
			return err
		}
	}

	// 生成 Samba 配置文件
	confPath := fmt.Sprintf("%s/smb_%d.conf", m.dataDir, id)
	shareName := fmt.Sprintf("netpanel_%d", id)
	if cfg.Name != "" {
		if err := validateShareName(cfg.Name); err != nil {
			m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
				"status":     "error",
				"last_error": err.Error(),
			})
			return err
		}
		shareName = cfg.Name
	}

	readOnly := "no"
	if cfg.ReadOnly {
		readOnly = "yes"
	}

	// 构建 smb.conf
	var confBuf strings.Builder
	confBuf.WriteString("[global]\n")
	confBuf.WriteString(fmt.Sprintf("smb ports = %d\n", cfg.ListenPort))
	confBuf.WriteString(fmt.Sprintf("bind interfaces only = yes\n"))
	if cfg.ListenAddr != "" && cfg.ListenAddr != "0.0.0.0" {
		confBuf.WriteString(fmt.Sprintf("interfaces = %s\n", cfg.ListenAddr))
	}
	confBuf.WriteString("server role = standalone\n")
	confBuf.WriteString("map to guest = Bad User\n")
	confBuf.WriteString("log level = 1\n")
	confBuf.WriteString(fmt.Sprintf("pid directory = %s\n", m.dataDir))
	confBuf.WriteString(fmt.Sprintf("lock directory = %s\n", m.dataDir))
	confBuf.WriteString(fmt.Sprintf("private dir = %s\n", m.dataDir))
	confBuf.WriteString(fmt.Sprintf("state directory = %s\n", m.dataDir))
	confBuf.WriteString(fmt.Sprintf("cache directory = %s\n", m.dataDir))
	confBuf.WriteString("security = user\n")

	if cfg.Username == "" {
		// 无认证，允许匿名访问
		confBuf.WriteString("guest ok = yes\n")
		confBuf.WriteString("guest account = nobody\n")
	}

	confBuf.WriteString(fmt.Sprintf("\n[%s]\n", shareName))
	confBuf.WriteString(fmt.Sprintf("path = %s\n", cfg.RootPath))
	confBuf.WriteString(fmt.Sprintf("read only = %s\n", readOnly))
	confBuf.WriteString("browseable = yes\n")
	confBuf.WriteString("create mask = 0644\n")
	confBuf.WriteString("directory mask = 0755\n")

	if cfg.Username == "" {
		confBuf.WriteString("guest ok = yes\n")
		confBuf.WriteString("force user = nobody\n")
	} else {
		confBuf.WriteString("guest ok = no\n")
		confBuf.WriteString(fmt.Sprintf("valid users = %s\n", cfg.Username))
	}

	if err := os.MkdirAll(m.dataDir, 0700); err != nil {
		return fmt.Errorf("创建数据目录失败: %w", err)
	}
	if err := os.WriteFile(confPath, []byte(confBuf.String()), 0600); err != nil {
		return fmt.Errorf("写入 SMB 配置文件失败: %w", err)
	}

	// 如果设置了用户名密码，需要创建 Samba 用户
	if cfg.Username != "" && cfg.Password != "" {
		// 确保系统用户存在（忽略已存在的错误）
		// "--" 终止选项解析，确保用户名不会被当作命令行选项
		exec.Command("useradd", "-M", "-s", "/sbin/nologin", "--", cfg.Username).Run()
		// 设置 Samba 密码
		cmd := exec.Command("smbpasswd", "-a", "-s", "--", cfg.Username)
		cmd.Stdin = strings.NewReader(cfg.Password.String() + "\n" + cfg.Password.String() + "\n")
		if out, err := cmd.CombinedOutput(); err != nil {
			m.log.Warnf("[SMB] 设置 Samba 用户密码失败: %v, output: %s", err, string(out))
		}
	}

	// 启动独立的 smbd 进程
	pidFile := fmt.Sprintf("%s/smbd_%d.pid", m.dataDir, id)
	smbdCmd := exec.Command("smbd",
		"--configfile", confPath,
		"--daemon",
		"--no-process-group",
		fmt.Sprintf("--pidfile=%s", pidFile),
	)
	if out, err := smbdCmd.CombinedOutput(); err != nil {
		errMsg := fmt.Sprintf("启动 smbd 失败: %v, output: %s", err, string(out))
		m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     "error",
			"last_error": errMsg,
		})
		os.Remove(confPath)
		return fmt.Errorf("%s", errMsg)
	}

	// 记录配置文件路径和 PID 文件路径，用于后续停止
	entry := &storageEntry{smbConfPath: confPath, smbPidFile: pidFile}
	m.entries.Store(id, entry)

	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[SMB][%d] 已启动，监听 %s:%d，共享: %s，根目录: %s", id, cfg.ListenAddr, cfg.ListenPort, shareName, cfg.RootPath)
	return nil
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
		// 停止 SMB 进程
		if entry.smbPidFile != "" {
			m.stopSMBProcess(entry.smbPidFile)
		}
		// 清理 SMB 配置文件
		if entry.smbConfPath != "" {
			os.Remove(entry.smbConfPath)
		}
		m.entries.Delete(id)
	}
	m.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("status", "stopped")
}

// stopSMBProcess 通过 PID 文件停止 smbd 进程
func (m *Manager) stopSMBProcess(pidFile string) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		m.log.Debugf("[SMB] 读取 PID 文件失败: %v", err)
		return
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		m.log.Debugf("[SMB] 解析 PID 失败: %v", err)
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		m.log.Debugf("[SMB] 查找进程 %d 失败: %v", pid, err)
		return
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		m.log.Debugf("[SMB] 发送 SIGTERM 到进程 %d 失败: %v", pid, err)
	}
	os.Remove(pidFile)
}

func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.entries.Load(id); ok {
		return "running"
	}
	return "stopped"
}
