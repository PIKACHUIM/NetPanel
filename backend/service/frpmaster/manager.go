package frpmaster

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/selector"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// 节点派生状态。
const (
	StatusOnline  = "online"
	StatusOffline = "offline"
	// DefaultOfflineAfter 心跳超过该时长未刷新即视为离线。
	DefaultOfflineAfter = 90 * time.Second
	// maxAgentLogBatch 单次日志回传的最大行数（防单条上报打爆日志表）。
	maxAgentLogBatch = 100
	// maxAgentLogMessageLen 单条日志消息最大字节数（防超长 message 撑满 text 列）。
	maxAgentLogMessageLen = 4096
)

// 包级错误，供 API 层区分 404 / 401。
var (
	ErrNodeNotFound = errors.New("frpmaster: 节点不存在")
	ErrBadToken     = errors.New("frpmaster: 节点 token 校验失败")
)

// CreateRequest 注册节点入参（admin API）。
type CreateRequest struct {
	Name       string
	Region     string
	ServerAddr string
	ServerPort int
	FrpsToken  string
	Remark     string
}

// Manager frpc 多节点 Master 控制面管理器。
type Manager struct {
	db           *gorm.DB
	log          *logrus.Logger
	offlineAfter time.Duration
}

// NewManager 创建节点管理器。
func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	if log == nil {
		log = logrus.New()
	}
	return &Manager{
		db:           db,
		log:          log,
		offlineAfter: DefaultOfflineAfter,
	}
}

// SetOfflineAfter 覆盖离线判定窗口（测试用）；d<=0 忽略。
func (m *Manager) SetOfflineAfter(d time.Duration) {
	if d > 0 {
		m.offlineAfter = d
	}
}

// nodeStatus 按 LastSeen 派生在线状态。
func (m *Manager) nodeStatus(n model.FrpMasterNode) string {
	if n.LastSeen.IsZero() || time.Since(n.LastSeen) > m.offlineAfter {
		return StatusOffline
	}
	return StatusOnline
}

// List 返回全部节点（含派生状态，token 相关字段不回显）。
func (m *Manager) List() ([]model.FrpMasterNode, error) {
	var nodes []model.FrpMasterNode
	if err := m.db.Order("id desc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	out := make([]model.FrpMasterNode, 0, len(nodes))
	for _, n := range nodes {
		n.Status = m.nodeStatus(n)
		out = append(out, n)
	}
	return out, nil
}

// Lines 把在线节点注册为自动选线的候选线路（每节点一条：其 frps 服务端入口）。
//
// 规划 #84 M4：远程节点的 frpc 隧道都经由该节点的 frps 服务端暴露，客户端
// 访问「同一穿透服务」时可选任意在线节点的入口 —— 由此把 selector 从单机
// 候选扩展到多节点候选池。节点离线（心跳超时）时不再提供线路，linereg 的
// SetLines 全量刷新会自动将其从选线状态中清理。
// 注：Line.Region / Weight 字段依赖 M1（#85 selector 加权）合入后可在此补全。
func (m *Manager) Lines() ([]selector.Line, error) {
	var nodes []model.FrpMasterNode
	if err := m.db.Find(&nodes).Error; err != nil {
		m.log.Warnf("[frpmaster] 读取节点列表失败: %v", err)
		return nil, err
	}
	lines := make([]selector.Line, 0, len(nodes))
	for _, n := range nodes {
		if m.nodeStatus(n) != StatusOnline {
			continue
		}
		if n.ServerAddr == "" || n.ServerPort <= 0 || n.ServerPort > 65535 {
			continue
		}
		lines = append(lines, selector.Line{
			ID:      fmt.Sprintf("fnode:%d", n.ID),
			Name:    n.Name,
			Tool:    "frpc-remote",
			Address: fmt.Sprintf("%s:%d", n.ServerAddr, n.ServerPort),
		})
	}
	return lines, nil
}

// Create 注册节点：生成节点 token（明文仅本函数返回一次），落库存 SHA-256。
func (m *Manager) Create(req CreateRequest) (*model.FrpMasterNode, string, error) {
	if req.Name == "" || req.ServerAddr == "" {
		return nil, "", errors.New("frpmaster: name 与 server_addr 为必填项")
	}
	if req.ServerPort <= 0 || req.ServerPort > 65535 {
		return nil, "", errors.New("frpmaster: server_port 需在 1-65535 之间")
	}
	token, err := newToken(32)
	if err != nil {
		return nil, "", fmt.Errorf("frpmaster: 生成 token 失败: %w", err)
	}
	node := &model.FrpMasterNode{
		Name:          req.Name,
		Region:        req.Region,
		NodeTokenHash: hashToken(token),
		ServerAddr:    req.ServerAddr,
		ServerPort:    req.ServerPort,
		FrpsToken:     req.FrpsToken,
		Remark:        req.Remark,
	}
	if err := m.db.Create(node).Error; err != nil {
		return nil, "", err
	}
	return node, token, nil
}

// Delete 删除节点。
func (m *Manager) Delete(id uint) error {
	return m.db.Delete(&model.FrpMasterNode{}, id).Error
}

// Authenticate 校验节点控制通道 token（sha256 恒定时间比较，防时序侧信道）。
func (m *Manager) Authenticate(id uint, token string) bool {
	if token == "" {
		return false
	}
	var n model.FrpMasterNode
	if err := m.db.First(&n, id).Error; err != nil {
		return false
	}
	got := hashToken(token)
	return subtle.ConstantTimeCompare([]byte(n.NodeTokenHash), []byte(got)) == 1
}

// Heartbeat 节点心跳：刷新 LastSeen。
func (m *Manager) Heartbeat(id uint) error {
	return m.db.Model(&model.FrpMasterNode{}).
		Where("id = ?", id).
		Update("last_seen", time.Now()).Error
}

// SaveStatus 保存节点上报的隧道状态 JSON（M3 原样存储，M4 解析展示）。
func (m *Manager) SaveStatus(id uint, tunnelsJSON string) error {
	return m.db.Model(&model.FrpMasterNode{}).
		Where("id = ?", id).
		Update("last_tunnels", tunnelsJSON).Error
}

// SaveLogs 保存节点回传的日志行：聚合写入 SystemLog（Service=frpmaster，
// 消息带节点 id 前缀便于过滤）。空行跳过；单批最多 maxAgentLogBatch 行，
// 超出部分截断。
func (m *Manager) SaveLogs(nodeID uint, lines []string) (int, error) {
	if len(lines) > maxAgentLogBatch {
		lines = lines[:maxAgentLogBatch]
	}
	now := time.Now()
	entries := make([]model.SystemLog, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		entries = append(entries, model.SystemLog{
			Level:   "info",
			Service: "frpmaster",
			Message: truncateToUTF8(fmt.Sprintf("[节点 %d] %s", nodeID, line), maxAgentLogMessageLen),
			LogTime: now,
		})
	}
	if len(entries) == 0 {
		return 0, nil
	}
	if err := m.db.Create(&entries).Error; err != nil {
		return 0, err
	}
	return len(entries), nil
}

// Config 生成节点的 frpc.toml（基于节点保存的 frps 连接信息）。
// 当前生成 [common] 最小集；代理配置随 M4（节点隧道管理）扩展。
func (m *Manager) Config(id uint) (string, error) {
	var n model.FrpMasterNode
	if err := m.db.First(&n, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNodeNotFound
		}
		return "", err
	}
	return GenerateFrpcToml(ClientConfig{
		ServerAddr:    n.ServerAddr,
		ServerPort:    n.ServerPort,
		Token:         n.FrpsToken,
		TLSEnable:     true,
		LoginFailExit: true,
		LogLevel:      "info",
	}), nil
}

// newToken 生成 n 字节随机数的 hex 字符串。
func newToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// hashToken 返回 token 的 SHA-256 hex（落库/比对用，不回存明文）。
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// truncateToUTF8 将字符串截断至最多 maxBytes 字节（按 UTF-8 rune 边界对齐）。
func truncateToUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	b := []byte(s)
	// 从 maxBytes 向前找到第一个非续接字节（非 0b10xxxxxx），
	// 在该 rune 的起始位置截断。原实现从 maxBytes-1 起找，会把跨越
	// 边界的那个 rune 的续接字节留在末尾，产生非法 UTF-8。
	for i := maxBytes - 1; i >= 0; i-- {
		if b[i]&0xc0 != 0x80 {
			// b[i] 是某个 rune 的首字节，判断该 rune 是否完整落在边界内
			size := 1
			switch {
			case b[i]&0x80 == 0x00:
				size = 1
			case b[i]&0xe0 == 0xc0:
				size = 2
			case b[i]&0xf0 == 0xe0:
				size = 3
			case b[i]&0xf8 == 0xf0:
				size = 4
			}
			if i+size <= maxBytes {
				return string(b[:i+size])
			}
			return string(b[:i])
		}
	}
	return ""
}
