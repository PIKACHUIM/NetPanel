package stun

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// NATType NAT 类型枚举
type NATType string

const (
	NATTypeUnknown           NATType = "Unknown"
	NATTypeOpenInternet      NATType = "Open Internet"
	NATTypeFullCone          NATType = "Full Cone NAT"
	NATTypeRestrictedCone    NATType = "Restricted Cone NAT"
	NATTypePortRestricted    NATType = "Port Restricted Cone NAT"
	NATTypeSymmetric         NATType = "Symmetric NAT"
	NATTypeSymmetricFirewall NATType = "Symmetric UDP Firewall"
	NATTypeBlocked           NATType = "UDP Blocked"
)

// NATInfo STUN 检测结果
type NATInfo struct {
	IP      string
	Port    int
	NATType NATType
}

// CallbackNotifier 回调通知接口（由外部注入，避免循环依赖）
type CallbackNotifier interface {
	TriggerBySTUN(ruleID uint, ip string, port int) error
}

// stunEntry 单个 STUN 任务运行实例
type stunEntry struct {
	cancel     context.CancelFunc
	info       *NATInfo
	stunStatus string // penetrating / timeout / failed
	mu         sync.RWMutex
}

// Manager STUN 管理器
type Manager struct {
	db       *gorm.DB
	log      *logrus.Logger
	entries  sync.Map // map[uint]*stunEntry
	callback CallbackNotifier
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// SetCallbackNotifier 注入回调通知器
func (m *Manager) SetCallbackNotifier(n CallbackNotifier) {
	m.callback = n
}

func (m *Manager) StartAll() {
	var rules []model.StunRule
	m.db.Where("enable = ?", true).Find(&rules)
	for _, rule := range rules {
		if err := m.Start(rule.ID); err != nil {
			m.log.Errorf("[STUN服务][%s] 启动失败: %v", rule.Name, err)
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
	if !rule.Enable {
		return fmt.Errorf("规则 [%s] 未启用", rule.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &stunEntry{cancel: cancel}
	m.entries.Store(id, entry)

	m.db.Model(&model.StunRule{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})

	go m.runLoop(ctx, id, entry)
	m.log.Infof("[STUN服务][%s] 已启动", rule.Name)
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

func (m *Manager) Restart(id uint) error {
	m.Stop(id)
	time.Sleep(300 * time.Millisecond)
	return m.Start(id)
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
		if entry.info != nil {
			cp := *entry.info
			return &cp
		}
	}
	return nil
}

// GetStunStatus 获取 STUN 穿透细化状态
func (m *Manager) GetStunStatus(id uint) string {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*stunEntry)
		entry.mu.RLock()
		defer entry.mu.RUnlock()
		return entry.stunStatus
	}
	return ""
}

// defaultSTUNServer 默认 STUN 服务器
const defaultSTUNServer = "stun.l.google.com:19302"

// runLoop 主循环：定时检测 + 指数退避重试。
//   - proxy 模式：持久转发器（UDP/TCP）保活 + 转发，映射地址随保活刷新；
//   - direct 模式：周期检测外部地址，由路由器（UPnP/NATMAP）负责映射转发。
//
// 关键设计：保活与重试退避解耦。
//   - 保活固定按 keepaliveInterval（<=25s）运行，维持 NAT 映射（UDP NAT 映射空闲
//     回收通常 30-120s）。失败只影响状态与显示，不推迟下一次保活，避免退避期内
//     映射失效、恢复后拿到新端口触发多余"地址变化"回调。
//   - 错误/建立失败才进入指数退避（5s->...->5min），仅用于重试建立转发器或
//     处理检测失败，不影响保活定时器。
func (m *Manager) runLoop(ctx context.Context, id uint, entry *stunEntry) {
	defer func() {
		m.entries.Delete(id)
		m.db.Model(&model.StunRule{}).Where("id = ?", id).Update("status", "stopped")
	}()

	backoff := 5 * time.Second
	maxBackoff := 5 * time.Minute
	// 检测/重试间隔：失败/重试时使用（指数退避），成功后重置为 baseCheckInterval
	checkInterval := 30 * time.Second
	baseCheckInterval := 30 * time.Second
	// 保活间隔：独立于退避，固定运行以维持 NAT 映射
	keepaliveInterval := 25 * time.Second
	keepaliveTicker := time.NewTicker(keepaliveInterval)
	defer keepaliveTicker.Stop()

	var fwd forwarder
	defer func() {
		if fwd != nil {
			fwd.Close()
		}
	}()

	// runKeepalive 执行一轮保活（仅在已有转发器时生效）。
	// 保活失败只影响状态与显示，不推迟下一次保活定时器。
	runKeepalive := func() {
		if fwd == nil {
			return
		}
		info, err := fwd.Keepalive()
		if err != nil {
			m.log.Debugf("[STUN服务][%d] 保活失败: %v", id, err)
			return
		}
		if info != nil {
			entry.mu.Lock()
			entry.info = info
			entry.stunStatus = "penetrating"
			entry.mu.Unlock()
			m.db.Model(&model.StunRule{}).Where("id = ?", id).Updates(map[string]interface{}{
				"current_ip":   info.IP,
				"current_port": info.Port,
				"last_error":   "",
				"stun_status":  "penetrating",
			})
			if info.NATType != "" {
				m.db.Model(&model.StunRule{}).Where("id = ?", id).Update("nat_type", string(info.NATType))
			}
		}
	}

	for {
		// 重新读取最新配置（配置修改会经 Stop/Start 重建循环，此处兜底）
		var rule model.StunRule
		if err := m.db.First(&rule, id).Error; err != nil {
			m.log.Errorf("[STUN服务][%d] 读取配置失败: %v", id, err)
			return
		}

		var changed bool
		var err error
		if rule.ForwardMode == "direct" {
			changed, err = m.doDirectCheck(ctx, id, &rule, entry)
		} else {
			// proxy 模式：转发器仅创建一次（配置修改经 Restart 重建）
			if fwd == nil {
				fwd, err = m.startForwarder(ctx, id, &rule, entry)
				if err != nil {
					m.log.Warnf("[STUN服务][%s] 建立转发失败 (退避 %v): %v", rule.Name, backoff, err)
					entry.mu.Lock()
					entry.stunStatus = "failed"
					entry.mu.Unlock()
					m.db.Model(&model.StunRule{}).Where("id = ?", id).Updates(map[string]interface{}{
						"last_error":  err.Error(),
						"stun_status": "failed",
					})
					select {
					case <-ctx.Done():
						return
					case <-time.After(backoff):
						backoff = min(backoff*2, maxBackoff)
						continue
					}
				}
			}
			changed, err = m.doProxyCheck(id, &rule, entry, fwd)
		}

		if err != nil {
			m.log.Warnf("[STUN服务][%s] 检测失败 (退避 %v): %v", rule.Name, backoff, err)
			// 区分超时和失败
			stunStatus := "failed"
			if strings.Contains(err.Error(), "超时") || strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline") {
				stunStatus = "timeout"
			}
			entry.mu.Lock()
			entry.stunStatus = stunStatus
			entry.mu.Unlock()
			m.db.Model(&model.StunRule{}).Where("id = ?", id).Updates(map[string]interface{}{
				"last_error":  err.Error(),
				"stun_status": stunStatus,
			})

			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
				backoff = min(backoff*2, maxBackoff)
				continue
			}
		}

		// 成功后重置检测间隔（保活间隔不变）
		checkInterval = baseCheckInterval
		backoff = 5 * time.Second

		// IP/端口变化时触发回调
		if changed && rule.CallbackTaskID > 0 && m.callback != nil {
			info := m.GetCurrentInfo(id)
			if info != nil {
				if err := m.callback.TriggerBySTUN(rule.CallbackTaskID, info.IP, info.Port); err != nil {
					m.log.Warnf("[STUN服务][%s] 触发回调失败: %v", rule.Name, err)
				}
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-keepaliveTicker.C:
			runKeepalive()
		case <-time.After(checkInterval):
		}
	}
}

// startForwarder 建立 proxy 模式的持久转发器（按目标协议选 UDP/TCP）。
// 未配置转发目标时仍建立 UDP 转发器做保活探测（不转发流量）。
func (m *Manager) startForwarder(ctx context.Context, id uint, rule *model.StunRule, entry *stunEntry) (forwarder, error) {
	stunServer := rule.StunServer
	if stunServer == "" {
		stunServer = defaultSTUNServer
	}

	// 一次性 NAT 类型检测（独立 socket，不影响转发映射的持久性）
	natType := NATTypeUnknown
	if !rule.DisableValidation {
		if info, err := detectNATType(stunServer); err == nil {
			natType = info.NATType
			m.db.Model(&model.StunRule{}).Where("id = ?", id).Update("nat_type", string(natType))
		} else {
			m.log.Warnf("[STUN服务][%s] NAT 类型检测失败（不影响穿透）: %v", rule.Name, err)
		}
	}

	stunAddr, err := net.ResolveUDPAddr("udp4", stunServer)
	if err != nil {
		return nil, fmt.Errorf("解析 STUN 服务器地址失败: %w", err)
	}

	protocol := strings.ToLower(strings.TrimSpace(rule.TargetProtocol))
	if protocol == "" {
		protocol = "udp"
	}

	if protocol == "tcp" {
		if rule.TargetAddress == "" || rule.TargetPort <= 0 {
			return nil, fmt.Errorf("proxy+TCP 模式需要配置转发目标地址与端口")
		}
		target := fmt.Sprintf("%s:%d", rule.TargetAddress, rule.TargetPort)
		return newTCPForward(ctx, rule.ListenPort, target, stunServer, natType, m.log)
	}

	var target *net.UDPAddr
	if rule.TargetAddress != "" && rule.TargetPort > 0 {
		target, err = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", rule.TargetAddress, rule.TargetPort))
		if err != nil {
			return nil, fmt.Errorf("解析转发目标失败: %w", err)
		}
	} else {
		m.log.Warnf("[STUN服务][%s] 未配置转发目标，仅做地址探测与保活（不转发流量）", rule.Name)
	}
	f, err := newUDPForward(rule.ListenPort, stunAddr, target, natType, m.log)
	if err != nil {
		return nil, err
	}
	f.startSweeper(ctx)
	if target != nil {
		m.log.Infof("[STUN服务][%s] UDP 转发已建立: 外部映射 -> 本地 :%d -> %s", rule.Name, f.LocalPort(), target)
	}
	return f, nil
}

// doProxyCheck 执行一轮保活并更新映射地址状态，返回地址是否变化。
// TCP 转发器的保活可能拿不到映射地址（返回 nil info）：此时仅保持状态，
// 不触发地址变化判定。
func (m *Manager) doProxyCheck(id uint, rule *model.StunRule, entry *stunEntry, fwd forwarder) (bool, error) {
	info, err := fwd.Keepalive()
	if err != nil {
		return false, err
	}

	entry.mu.Lock()
	oldInfo := entry.info
	if info != nil {
		entry.info = info
	}
	entry.stunStatus = "penetrating"
	entry.mu.Unlock()

	cur := info
	if cur == nil {
		cur = oldInfo
	}
	if cur == nil {
		return false, nil
	}

	updates := map[string]interface{}{
		"current_ip":   cur.IP,
		"current_port": cur.Port,
		"last_error":   "",
		"stun_status":  "penetrating",
	}
	if cur.NATType != "" {
		updates["nat_type"] = string(cur.NATType)
	}
	m.db.Model(&model.StunRule{}).Where("id = ?", id).Updates(updates)

	if info == nil {
		return false, nil
	}
	changed := oldInfo == nil || oldInfo.IP != info.IP || oldInfo.Port != info.Port
	if changed {
		m.log.Infof("[STUN服务][%s] 地址变化: %s:%d (NAT: %s)", rule.Name, info.IP, info.Port, info.NATType)
	}
	return changed, nil
}

// doDirectCheck direct 模式检测：路由器负责转发（UPnP/NATMAP 映射到目标），
// 面板周期探测外部地址、刷新映射并在变化时触发回调。
func (m *Manager) doDirectCheck(ctx context.Context, id uint, rule *model.StunRule, entry *stunEntry) (bool, error) {
	stunServer := rule.StunServer
	if stunServer == "" {
		stunServer = defaultSTUNServer
	}

	var info *NATInfo
	var err error

	if rule.DisableValidation {
		// 禁用有效性检测：只做基础 Binding Request，不做 NAT 类型判断
		info, err = detectBasicSTUN(stunServer)
	} else {
		// 执行完整 NAT 类型检测
		info, err = detectNATType(stunServer)
	}
	if err != nil {
		return false, err
	}

	if rule.UseNATMAP {
		m.log.Warnf("[STUN服务][%s] NATMAP 暂未实现，已忽略（可改用 UPnP）", rule.Name)
	}

	// UPnP 映射到目标（direct 模式语义）：外部端口 -> 目标地址:目标端口
	if rule.UseUPnP && rule.TargetPort > 0 {
		if upnpIP, upnpPort, err := m.upnpMapTarget(rule); err == nil {
			m.log.Infof("[STUN服务][%s] UPnP 映射成功: %s:%d -> %s:%d",
				rule.Name, upnpIP, upnpPort, rule.TargetAddress, rule.TargetPort)
			info.IP = upnpIP
			info.Port = upnpPort
		} else {
			m.log.Warnf("[STUN服务][%s] UPnP 映射失败，使用 STUN 结果: %v", rule.Name, err)
		}
	} else if rule.UseUPnP {
		m.log.Warnf("[STUN服务][%s] 已启用 UPnP 但未配置转发目标端口，跳过映射", rule.Name)
	}

	entry.mu.Lock()
	oldInfo := entry.info
	entry.info = info
	entry.stunStatus = "penetrating"
	entry.mu.Unlock()

	// 判断是否变化
	changed := oldInfo == nil || oldInfo.IP != info.IP || oldInfo.Port != info.Port

	// 更新数据库
	updates := map[string]interface{}{
		"current_ip":   info.IP,
		"current_port": info.Port,
		"nat_type":     string(info.NATType),
		"last_error":   "",
		"stun_status":  "penetrating",
	}
	m.db.Model(&model.StunRule{}).Where("id = ?", id).Updates(updates)

	if changed {
		m.log.Infof("[STUN服务][%s] 地址变化: %s:%d (NAT: %s)", rule.Name, info.IP, info.Port, info.NATType)
	}

	return changed, nil
}

// detectBasicSTUN 仅做基础 Binding Request，不做 NAT 类型判断（禁用有效性检测时使用）
func detectBasicSTUN(stunServer string) (*NATInfo, error) {
	serverAddr, err := net.ResolveUDPAddr("udp4", stunServer)
	if err != nil {
		return nil, fmt.Errorf("解析 STUN 服务器地址失败: %w", err)
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, fmt.Errorf("创建 UDP socket 失败: %w", err)
	}
	defer conn.Close()

	resp, err := sendSTUN(conn, serverAddr, false, false)
	if err != nil {
		return nil, fmt.Errorf("STUN 请求失败: %w", err)
	}
	if resp.msgType != msgTypeBindingResponse {
		return nil, fmt.Errorf("STUN 返回非成功响应")
	}

	ip, port, err := getMappedAddress(resp)
	if err != nil {
		return nil, fmt.Errorf("解析映射地址失败: %w", err)
	}

	return &NATInfo{IP: ip, Port: port, NATType: NATTypeUnknown}, nil
}

// ===== STUN 协议实现 =====

const (
	stunMagicCookie = 0x2112A442
	// 消息类型
	msgTypeBindingRequest  = 0x0001
	msgTypeBindingResponse = 0x0101
	msgTypeBindingError    = 0x0111
	// 属性类型
	attrMappedAddress    = 0x0001
	attrChangedAddress   = 0x0005
	attrXORMappedAddress = 0x0020
	attrChangeRequest    = 0x0003
)

// stunMessage STUN 消息
type stunMessage struct {
	msgType       uint16
	transactionID [12]byte
	attributes    map[uint16][]byte
}

// buildBindingRequest 构建 Binding Request
func buildBindingRequest(changeIP, changePort bool) []byte {
	var tid [12]byte
	rand.Read(tid[:])

	msg := make([]byte, 20)
	binary.BigEndian.PutUint16(msg[0:2], msgTypeBindingRequest)
	binary.BigEndian.PutUint32(msg[4:8], stunMagicCookie)
	copy(msg[8:20], tid[:])

	if changeIP || changePort {
		// 添加 CHANGE-REQUEST 属性
		attr := make([]byte, 8)
		binary.BigEndian.PutUint16(attr[0:2], attrChangeRequest)
		binary.BigEndian.PutUint16(attr[2:4], 4)
		var flags uint32
		if changeIP {
			flags |= 0x04
		}
		if changePort {
			flags |= 0x02
		}
		binary.BigEndian.PutUint32(attr[4:8], flags)
		msg = append(msg, attr...)
		binary.BigEndian.PutUint16(msg[2:4], uint16(len(msg)-20))
	}

	return msg
}

// parseSTUNResponse 解析 STUN 响应
func parseSTUNResponse(data []byte) (*stunMessage, error) {
	if len(data) < 20 {
		return nil, fmt.Errorf("响应太短: %d 字节", len(data))
	}

	msg := &stunMessage{
		msgType:    binary.BigEndian.Uint16(data[0:2]),
		attributes: make(map[uint16][]byte),
	}
	copy(msg.transactionID[:], data[8:20])

	// 解析属性
	offset := 20
	for offset+4 <= len(data) {
		attrType := binary.BigEndian.Uint16(data[offset : offset+2])
		attrLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
		offset += 4
		if offset+attrLen > len(data) {
			break
		}
		msg.attributes[attrType] = data[offset : offset+attrLen]
		offset += attrLen
		// 4 字节对齐
		if attrLen%4 != 0 {
			offset += 4 - attrLen%4
		}
	}
	return msg, nil
}

// extractAddress 从属性中提取 IP:Port
func extractAddress(data []byte, xor bool) (string, int, error) {
	if len(data) < 8 {
		return "", 0, fmt.Errorf("地址属性太短")
	}
	family := data[1]
	rawPort := binary.BigEndian.Uint16(data[2:4])
	var port int
	var ip net.IP

	if xor {
		port = int(rawPort ^ uint16(stunMagicCookie>>16))
		if family == 0x01 { // IPv4
			ip = net.IP{
				data[4] ^ 0x21,
				data[5] ^ 0x12,
				data[6] ^ 0xA4,
				data[7] ^ 0x42,
			}
		} else {
			return "", 0, fmt.Errorf("暂不支持 IPv6 XOR 地址")
		}
	} else {
		port = int(rawPort)
		if family == 0x01 {
			ip = net.IP{data[4], data[5], data[6], data[7]}
		} else {
			return "", 0, fmt.Errorf("暂不支持 IPv6 地址")
		}
	}
	return ip.String(), port, nil
}

// sendSTUN 向 STUN 服务器发送请求并接收响应
func sendSTUN(conn *net.UDPConn, serverAddr *net.UDPAddr, changeIP, changePort bool) (*stunMessage, error) {
	req := buildBindingRequest(changeIP, changePort)
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.WriteToUDP(req, serverAddr); err != nil {
		return nil, fmt.Errorf("发送失败: %w", err)
	}

	buf := make([]byte, 1500)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, fmt.Errorf("接收超时: %w", err)
	}
	return parseSTUNResponse(buf[:n])
}

// getMappedAddress 从响应中获取映射地址
func getMappedAddress(msg *stunMessage) (string, int, error) {
	if data, ok := msg.attributes[attrXORMappedAddress]; ok {
		return extractAddress(data, true)
	}
	if data, ok := msg.attributes[attrMappedAddress]; ok {
		return extractAddress(data, false)
	}
	return "", 0, fmt.Errorf("响应中无映射地址属性")
}

// getChangedAddress 从响应中获取备用服务器地址
func getChangedAddress(msg *stunMessage) (string, int, error) {
	data, ok := msg.attributes[attrChangedAddress]
	if !ok {
		return "", 0, fmt.Errorf("无 CHANGED-ADDRESS 属性")
	}
	return extractAddress(data, false)
}

// detectNATType 执行完整的 RFC 3489 NAT 类型检测
func detectNATType(stunServer string) (*NATInfo, error) {
	serverAddr, err := net.ResolveUDPAddr("udp4", stunServer)
	if err != nil {
		return nil, fmt.Errorf("解析 STUN 服务器地址失败: %w", err)
	}

	// 创建本地 UDP socket
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, fmt.Errorf("创建 UDP socket 失败: %w", err)
	}
	defer conn.Close()

	// === Test I: 基础绑定请求 ===
	resp1, err := sendSTUN(conn, serverAddr, false, false)
	if err != nil {
		return &NATInfo{NATType: NATTypeBlocked}, fmt.Errorf("Test I 失败（UDP 可能被封锁）: %w", err)
	}
	if resp1.msgType != msgTypeBindingResponse {
		return &NATInfo{NATType: NATTypeBlocked}, fmt.Errorf("Test I 收到非成功响应")
	}

	ip1, port1, err := getMappedAddress(resp1)
	if err != nil {
		return nil, fmt.Errorf("Test I 解析映射地址失败: %w", err)
	}

	// 获取本地 IP，判断是否直连互联网
	localIP := getLocalIP()

	info := &NATInfo{IP: ip1, Port: port1}

	if localIP == ip1 {
		// 本地 IP == 映射 IP，可能是直连或对称防火墙
		// === Test II: 请求服务器换 IP+Port 响应 ===
		_, err2 := sendSTUN(conn, serverAddr, true, true)
		if err2 == nil {
			info.NATType = NATTypeOpenInternet
		} else {
			info.NATType = NATTypeSymmetricFirewall
		}
		return info, nil
	}

	// 存在 NAT，获取备用服务器地址
	changedIP, changedPort, err := getChangedAddress(resp1)
	if err != nil {
		// 无法获取备用地址，无法进一步判断
		info.NATType = NATTypeUnknown
		return info, nil
	}

	// === Test II: 请求服务器换 IP+Port 响应 ===
	_, err2 := sendSTUN(conn, serverAddr, true, true)
	if err2 == nil {
		// 收到来自不同 IP+Port 的响应 => Full Cone NAT
		info.NATType = NATTypeFullCone
		return info, nil
	}

	// === Test I(b): 向备用服务器发送请求 ===
	altAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", changedIP, changedPort))
	if err != nil {
		info.NATType = NATTypeUnknown
		return info, nil
	}

	resp1b, err := sendSTUN(conn, altAddr, false, false)
	if err != nil {
		// 无法到达备用服务器
		info.NATType = NATTypeUnknown
		return info, nil
	}

	ip1b, port1b, err := getMappedAddress(resp1b)
	if err != nil {
		info.NATType = NATTypeUnknown
		return info, nil
	}

	if ip1b != ip1 || port1b != port1 {
		// 不同服务器看到不同映射 => Symmetric NAT
		info.NATType = NATTypeSymmetric
		return info, nil
	}

	// === Test III: 请求服务器只换 Port 响应 ===
	_, err3 := sendSTUN(conn, serverAddr, false, true)
	if err3 == nil {
		// 收到来自不同 Port 的响应 => Restricted Cone NAT
		info.NATType = NATTypeRestrictedCone
	} else {
		// 未收到 => Port Restricted Cone NAT
		info.NATType = NATTypePortRestricted
	}

	return info, nil
}

// getLocalIP 获取本机出口 IP
func getLocalIP() string {
	conn, err := net.Dial("udp4", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// ===== UPnP 实现 =====

// upnpMapTarget 通过 UPnP 在路由器上把外部端口映射到目标地址:目标端口
// （direct 模式语义：路由器直接转发，面板不经手流量）。
// 目标为本机/回环时映射到面板所在主机；目标为其它内网设备时需要路由器
// 支持跨主机内部地址映射。返回外部 IP 和外部端口。
func (m *Manager) upnpMapTarget(rule *model.StunRule) (string, int, error) {
	gateway, err := discoverUPnPGateway()
	if err != nil {
		return "", 0, fmt.Errorf("UPnP 网关发现失败: %w", err)
	}

	externalIP, err := gateway.getExternalIP()
	if err != nil {
		return "", 0, fmt.Errorf("获取外部 IP 失败: %w", err)
	}

	protocol := strings.ToLower(strings.TrimSpace(rule.TargetProtocol))
	if protocol != "tcp" {
		protocol = "udp"
	}
	internalIP := strings.TrimSpace(rule.TargetAddress)
	if internalIP == "" || internalIP == "127.0.0.1" || internalIP == "localhost" {
		internalIP = getLocalIP()
	}
	if err := gateway.addPortMapping(rule.TargetPort, rule.TargetPort, internalIP, protocol, "NetPanel STUN"); err != nil {
		return "", 0, fmt.Errorf("添加 UPnP 端口映射失败: %w", err)
	}

	return externalIP, rule.TargetPort, nil
}

// upnpGateway UPnP 网关
type upnpGateway struct {
	controlURL  string
	serviceType string
}

const (
	upnpSSDPAddr      = "239.255.255.250:1900"
	upnpSearchMsg     = "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 2\r\nST: urn:schemas-upnp-org:device:InternetGatewayDevice:1\r\n\r\n"
	upnpWANIPService  = "urn:schemas-upnp-org:service:WANIPConnection:1"
	upnpWANPPPService = "urn:schemas-upnp-org:service:WANPPPConnection:1"
)

// discoverUPnPGateway 通过 SSDP 发现 UPnP 网关
func discoverUPnPGateway() (*upnpGateway, error) {
	addr, err := net.ResolveUDPAddr("udp4", upnpSSDPAddr)
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	if _, err := conn.WriteToUDP([]byte(upnpSearchMsg), addr); err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, fmt.Errorf("未发现 UPnP 网关")
	}

	// 解析 LOCATION 头
	location := parseHTTPHeader(string(buf[:n]), "LOCATION")
	if location == "" {
		return nil, fmt.Errorf("UPnP 响应中无 LOCATION")
	}

	// 获取设备描述 XML
	controlURL, serviceType, err := fetchUPnPControlURL(location)
	if err != nil {
		return nil, err
	}

	return &upnpGateway{controlURL: controlURL, serviceType: serviceType}, nil
}

// parseHTTPHeader 从 HTTP 响应中解析指定头部
func parseHTTPHeader(response, header string) string {
	header = header + ":"
	for _, line := range splitLines(response) {
		if len(line) > len(header) {
			prefix := line[:len(header)]
			if equalFold(prefix, header) {
				return trimSpace(line[len(header):])
			}
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

// fetchUPnPControlURL 从设备描述 XML 中获取控制 URL
func fetchUPnPControlURL(location string) (string, string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(location)
	if err != nil {
		return "", "", fmt.Errorf("获取 UPnP 设备描述失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	xml := string(body)

	// 简单 XML 解析，查找 WANIPConnection 或 WANPPPConnection 的 controlURL
	for _, svcType := range []string{upnpWANIPService, upnpWANPPPService} {
		idx := strings.Index(xml, svcType)
		if idx < 0 {
			continue
		}
		// 在服务类型后面找 controlURL
		sub := xml[idx:]
		ctrlIdx := strings.Index(sub, "<controlURL>")
		if ctrlIdx < 0 {
			continue
		}
		ctrlEnd := strings.Index(sub[ctrlIdx:], "</controlURL>")
		if ctrlEnd < 0 {
			continue
		}
		ctrlURL := sub[ctrlIdx+len("<controlURL>") : ctrlIdx+ctrlEnd]

		// 如果是相对路径，拼接 base URL
		if !strings.HasPrefix(ctrlURL, "http") {
			base := extractBaseURL(location)
			ctrlURL = base + ctrlURL
		}
		return ctrlURL, svcType, nil
	}
	return "", "", fmt.Errorf("未找到 WANIPConnection 或 WANPPPConnection 服务")
}

func extractBaseURL(location string) string {
	// 提取 http://host:port 部分
	for i := len("http://"); i < len(location); i++ {
		if location[i] == '/' {
			return location[:i]
		}
	}
	return location
}

// soapAction 发送 UPnP SOAP 请求
func (g *upnpGateway) soapAction(action, body string) (string, error) {
	soapBody := `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
<s:Body>` + body + `</s:Body>
</s:Envelope>`

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("POST", g.controlURL, strings.NewReader(soapBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, g.serviceType, action))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("SOAP 错误 HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	return string(respBody), nil
}

// getExternalIP 获取外部 IP
func (g *upnpGateway) getExternalIP() (string, error) {
	body := `<u:GetExternalIPAddress xmlns:u="` + g.serviceType + `"></u:GetExternalIPAddress>`
	resp, err := g.soapAction("GetExternalIPAddress", body)
	if err != nil {
		return "", err
	}

	// 解析 <NewExternalIPAddress>
	start := strings.Index(resp, "<NewExternalIPAddress>")
	end := strings.Index(resp, "</NewExternalIPAddress>")
	if start < 0 || end < 0 {
		return "", fmt.Errorf("响应中无外部 IP")
	}
	return resp[start+len("<NewExternalIPAddress>") : end], nil
}

// addPortMapping 添加端口映射
func (g *upnpGateway) addPortMapping(externalPort, internalPort int, internalIP, protocol, description string) error {
	body := fmt.Sprintf(`<u:AddPortMapping xmlns:u="%s">
<NewRemoteHost></NewRemoteHost>
<NewExternalPort>%d</NewExternalPort>
<NewProtocol>%s</NewProtocol>
<NewInternalPort>%d</NewInternalPort>
<NewInternalClient>%s</NewInternalClient>
<NewEnabled>1</NewEnabled>
<NewPortMappingDescription>%s</NewPortMappingDescription>
<NewLeaseDuration>0</NewLeaseDuration>
</u:AddPortMapping>`, g.serviceType, externalPort, protocol, internalPort, internalIP, description)

	_, err := g.soapAction("AddPortMapping", body)
	return err
}

// min 辅助函数
func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
