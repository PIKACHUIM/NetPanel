package stun

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// NATInfo STUN 检测结果
type NATInfo struct {
	IP      string
	Port    int
	NATType string
}

// stunEntry 单个 STUN 任务
type stunEntry struct {
	cancel context.CancelFunc
	info   *NATInfo
	mu     sync.RWMutex
}

// Manager STUN 管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	entries sync.Map // map[uint]*stunEntry
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

func (m *Manager) StartAll() {
	var rules []model.StunRule
	m.db.Where("enable = ?", true).Find(&rules)
	for _, rule := range rules {
		if err := m.Start(rule.ID); err != nil {
			m.log.Errorf("STUN [%s] 启动失败: %v", rule.Name, err)
		}
	}
}

func (m *Manager) StopAll() {
	m.entries.Range(func(key, value interface{}) bool {
		entry := value.(*stunEntry)
		entry.cancel()
		return true
	})
}

func (m *Manager) Start(id uint) error {
	m.Stop(id)

	var rule model.StunRule
	if err := m.db.First(&rule, id).Error; err != nil {
		return fmt.Errorf("规则不存在: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &stunEntry{cancel: cancel}
	m.entries.Store(id, entry)

	go m.runSTUN(ctx, id, &rule, entry)

	m.db.Model(&model.StunRule{}).Where("id = ?", id).Update("status", "running")
	return nil
}

func (m *Manager) Stop(id uint) {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*stunEntry)
		entry.cancel()
		m.entries.Delete(id)
	}
	m.db.Model(&model.StunRule{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.entries.Load(id); ok {
		return "running"
	}
	return "stopped"
}

func (m *Manager) GetCurrentInfo(id uint) *NATInfo {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*stunEntry)
		entry.mu.RLock()
		defer entry.mu.RUnlock()
		return entry.info
	}
	return nil
}

// runSTUN 持续运行 STUN 检测
func (m *Manager) runSTUN(ctx context.Context, id uint, rule *model.StunRule, entry *stunEntry) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 立即执行一次
	m.doSTUNCheck(ctx, id, rule, entry)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.doSTUNCheck(ctx, id, rule, entry)
		}
	}
}

// doSTUNCheck 执行一次 STUN 检测
func (m *Manager) doSTUNCheck(ctx context.Context, id uint, rule *model.StunRule, entry *stunEntry) {
	stunServer := rule.StunServer
	if stunServer == "" {
		stunServer = "stun.l.google.com:19302"
	}

	ip, port, natType, err := detectNAT(stunServer)
	if err != nil {
		m.log.Warnf("[STUN][%d] 检测失败: %v", id, err)
		m.db.Model(&model.StunRule{}).Where("id = ?", id).Update("last_error", err.Error())
		return
	}

	info := &NATInfo{IP: ip, Port: port, NATType: natType}

	entry.mu.Lock()
	oldInfo := entry.info
	entry.info = info
	entry.mu.Unlock()

	// 更新数据库
	m.db.Model(&model.StunRule{}).Where("id = ?", id).Updates(map[string]interface{}{
		"current_ip":   ip,
		"current_port": port,
		"nat_type":     natType,
		"last_error":   "",
	})

	// IP 变化时触发回调
	if oldInfo == nil || oldInfo.IP != ip || oldInfo.Port != port {
		m.log.Infof("[STUN][%d] IP/端口变化: %s:%d (NAT类型: %s)", id, ip, port, natType)
		// TODO: 触发回调任务
	}
}

// detectNAT 使用 STUN 协议检测公网 IP 和端口
// 简化实现：通过 UDP 连接 STUN 服务器获取映射地址
func detectNAT(stunServer string) (ip string, port int, natType string, err error) {
	// 解析 STUN 服务器地址
	serverAddr, err := net.ResolveUDPAddr("udp", stunServer)
	if err != nil {
		return "", 0, "", fmt.Errorf("解析 STUN 服务器地址失败: %w", err)
	}

	// 创建本地 UDP 连接
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		return "", 0, "", fmt.Errorf("连接 STUN 服务器失败: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	// 构建 STUN Binding Request
	// RFC 5389: 0x0001 = Binding Request, Magic Cookie = 0x2112A442
	request := []byte{
		0x00, 0x01, // Message Type: Binding Request
		0x00, 0x00, // Message Length: 0
		0x21, 0x12, 0xA4, 0x42, // Magic Cookie
		// Transaction ID (12 bytes)
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07, 0x08,
		0x09, 0x0A, 0x0B, 0x0C,
	}

	if _, err = conn.Write(request); err != nil {
		return "", 0, "", fmt.Errorf("发送 STUN 请求失败: %w", err)
	}

	// 读取响应
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return "", 0, "", fmt.Errorf("读取 STUN 响应失败: %w", err)
	}

	// 解析 STUN 响应
	ip, port, err = parseSTUNResponse(buf[:n])
	if err != nil {
		return "", 0, "", err
	}

	natType = "Unknown"
	return ip, port, natType, nil
}

// parseSTUNResponse 解析 STUN 响应，提取 XOR-MAPPED-ADDRESS
func parseSTUNResponse(data []byte) (string, int, error) {
	if len(data) < 20 {
		return "", 0, fmt.Errorf("STUN 响应太短")
	}

	// 检查 Message Type: 0x0101 = Binding Success Response
	msgType := uint16(data[0])<<8 | uint16(data[1])
	if msgType != 0x0101 {
		return "", 0, fmt.Errorf("非 Binding Success Response: 0x%04X", msgType)
	}

	// 解析属性
	offset := 20
	for offset+4 <= len(data) {
		attrType := uint16(data[offset])<<8 | uint16(data[offset+1])
		attrLen := int(uint16(data[offset+2])<<8 | uint16(data[offset+3]))
		offset += 4

		if offset+attrLen > len(data) {
			break
		}

		switch attrType {
		case 0x0020: // XOR-MAPPED-ADDRESS
			if attrLen >= 8 {
				// Family: data[offset+1] (0x01=IPv4, 0x02=IPv6)
				xorPort := (uint16(data[offset+2])<<8 | uint16(data[offset+3])) ^ 0x2112
				port := int(xorPort)

				// XOR with magic cookie
				ip := net.IP{
					data[offset+4] ^ 0x21,
					data[offset+5] ^ 0x12,
					data[offset+6] ^ 0xA4,
					data[offset+7] ^ 0x42,
				}
				return ip.String(), port, nil
			}
		case 0x0001: // MAPPED-ADDRESS (fallback)
			if attrLen >= 8 {
				port := int(uint16(data[offset+2])<<8 | uint16(data[offset+3]))
				ip := net.IP{data[offset+4], data[offset+5], data[offset+6], data[offset+7]}
				return ip.String(), port, nil
			}
		}

		// 对齐到 4 字节
		offset += attrLen
		if attrLen%4 != 0 {
			offset += 4 - attrLen%4
		}
	}

	return "", 0, fmt.Errorf("未找到映射地址")
}
