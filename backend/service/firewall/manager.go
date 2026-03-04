package firewall

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager 防火墙管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	syncing bool       // 是否正在同步
	syncMu  sync.Mutex // 防止并发同步
	// 最后一次同步时间和结果
	lastSyncAt  time.Time
	lastSyncErr error
}

// NewManager 创建防火墙管理器
func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// SyncStatus 同步状态
type SyncStatus struct {
	Syncing     bool      `json:"syncing"`
	LastSyncAt  time.Time `json:"last_sync_at"`
	LastSyncErr string    `json:"last_sync_err"`
	Total       int       `json:"total"` // 本次同步到的规则数
}

// GetSyncStatus 获取当前同步状态
func (m *Manager) GetSyncStatus() SyncStatus {
	m.syncMu.Lock()
	defer m.syncMu.Unlock()
	errStr := ""
	if m.lastSyncErr != nil {
		errStr = m.lastSyncErr.Error()
	}
	var total int64
	m.db.Model(&model.FirewallRule{}).Where("is_system = ?", true).Count(&total)
	return SyncStatus{
		Syncing:     m.syncing,
		LastSyncAt:  m.lastSyncAt,
		LastSyncErr: errStr,
		Total:       int(total),
	}
}

// SyncSystemRulesAsync 异步从系统防火墙同步规则到数据库（非阻塞）
// 若已在同步中则直接返回
func (m *Manager) SyncSystemRulesAsync() {
	m.syncMu.Lock()
	if m.syncing {
		m.syncMu.Unlock()
		m.log.Info("[Firewall] 同步已在进行中，跳过本次触发")
		return
	}
	m.syncing = true
	m.syncMu.Unlock()

	go func() {
		defer func() {
			m.syncMu.Lock()
			m.syncing = false
			m.syncMu.Unlock()
		}()
		m.log.Info("[Firewall] 开始异步同步系统防火墙规则...")
		err := m.doSyncSystemRules()
		m.syncMu.Lock()
		m.lastSyncAt = time.Now()
		m.lastSyncErr = err
		m.syncMu.Unlock()
		if err != nil {
			m.log.Errorf("[Firewall] 同步系统防火墙规则失败: %v", err)
		} else {
			m.log.Info("[Firewall] 同步系统防火墙规则完成")
		}
	}()
}

// doSyncSystemRules 实际执行同步逻辑（在 goroutine 中运行）
func (m *Manager) doSyncSystemRules() error {
	rules, err := m.ListSystemRules()
	if err != nil {
		return fmt.Errorf("读取系统规则失败: %w", err)
	}

	// 先将所有 is_system=true 的旧记录标记为待清理（通过 raw 字段去重）
	// 策略：以 raw 字段为唯一键，存在则更新，不存在则新增，多余的删除
	existingRaws := make(map[string]uint) // raw -> id
	var existing []model.FirewallRule
	m.db.Where("is_system = ?", true).Find(&existing)
	for _, r := range existing {
		existingRaws[r.Raw] = r.ID
	}

	incomingRaws := make(map[string]bool)
	for _, sr := range rules {
		incomingRaws[sr.Raw] = true
		if id, ok := existingRaws[sr.Raw]; ok {
			// 已存在，更新字段（名称、方向、动作等可能变化）
			m.db.Model(&model.FirewallRule{}).Where("id = ?", id).Updates(map[string]any{
				"name":      sr.Name,
				"direction": sr.Direction,
				"action":    sr.Action,
				"protocol":  sr.Protocol,
				"src_ip":    sr.SrcIP,
				"dst_ip":    sr.DstIP,
				"port":      sr.Port,
				"interface": sr.Interface,
			})
		} else {
			// 新增
			newRule := model.FirewallRule{
				Name:        sr.Name,
				Enable:      true,
				Direction:   sr.Direction,
				Action:      sr.Action,
				Protocol:    sr.Protocol,
				SrcIP:       sr.SrcIP,
				DstIP:       sr.DstIP,
				Port:        sr.Port,
				Interface:   sr.Interface,
				Priority:    100,
				ApplyStatus: "applied", // 系统已有规则，标记为已应用
				IsSystem:    true,
				Raw:         sr.Raw,
				Remark:      "从系统防火墙自动同步",
			}
			m.db.Create(&newRule)
		}
	}

	// 删除系统中已不存在的旧记录（is_system=true 且 raw 不在本次结果中）
	for raw, id := range existingRaws {
		if !incomingRaws[raw] {
			m.db.Delete(&model.FirewallRule{}, id)
		}
	}
	return nil
}

// StartAutoSync 启动定时自动同步（每 30 分钟同步一次）
// 应在程序启动时调用，在后台 goroutine 中运行
func (m *Manager) StartAutoSync() {
	// 启动时立即同步一次
	m.SyncSystemRulesAsync()

	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			m.log.Info("[Firewall] 定时触发系统防火墙规则同步")
			m.SyncSystemRulesAsync()
		}
	}()
}

// DetectBackend 检测当前系统可用的防火墙后端
// 返回值：iptables / nftables / ufw / firewalld / openwrt / windows / unknown
func (m *Manager) DetectBackend() string {
	if runtime.GOOS == "windows" {
		return "windows"
	}
	// OpenWrt 特征：/etc/openwrt_release 存在
	if fileExists("/etc/openwrt_release") {
		return "openwrt"
	}
	// 优先检测 ufw
	if commandExists("ufw") {
		return "ufw"
	}
	// 检测 firewalld
	if commandExists("firewall-cmd") {
		return "firewalld"
	}
	// 检测 nftables
	if commandExists("nft") {
		return "nftables"
	}
	// 检测 iptables
	if commandExists("iptables") {
		return "iptables"
	}
	return "unknown"
}

// ApplyRule 将规则应用到系统防火墙
func (m *Manager) ApplyRule(rule *model.FirewallRule) error {
	backend := m.DetectBackend()
	m.log.Infof("[Firewall] 应用规则: id=%d name=%s backend=%s", rule.ID, rule.Name, backend)

	var err error
	switch backend {
	case "windows":
		err = m.applyWindows(rule)
	case "ufw":
		err = m.applyUfw(rule)
	case "firewalld":
		err = m.applyFirewalld(rule)
	case "nftables":
		err = m.applyNftables(rule)
	case "iptables":
		err = m.applyIptables(rule)
	case "openwrt":
		err = m.applyOpenwrt(rule)
	default:
		err = fmt.Errorf("未检测到受支持的防火墙后端（iptables/nftables/ufw/firewalld/Windows）")
	}

	// 更新应用状态
	status := "applied"
	lastError := ""
	if err != nil {
		status = "error"
		lastError = err.Error()
		m.log.Errorf("[Firewall] 应用规则失败: id=%d err=%v", rule.ID, err)
	}
	m.db.Model(&model.FirewallRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"apply_status": status,
		"last_error":   lastError,
	})
	return err
}

// RemoveRule 从系统防火墙删除规则
func (m *Manager) RemoveRule(rule *model.FirewallRule) error {
	backend := m.DetectBackend()
	m.log.Infof("[Firewall] 删除规则: id=%d name=%s backend=%s", rule.ID, rule.Name, backend)

	var err error
	switch backend {
	case "windows":
		err = m.removeWindows(rule)
	case "ufw":
		err = m.removeUfw(rule)
	case "firewalld":
		err = m.removeFirewalld(rule)
	case "nftables":
		err = m.removeNftables(rule)
	case "iptables":
		err = m.removeIptables(rule)
	case "openwrt":
		err = m.removeOpenwrt(rule)
	default:
		err = fmt.Errorf("未检测到受支持的防火墙后端")
	}
	if err != nil {
		m.log.Warnf("[Firewall] 删除规则失败（可能规则不存在）: id=%d err=%v", rule.ID, err)
	}
	return err
}

// ─── iptables ────────────────────────────────────────────────────────────────

func (m *Manager) applyIptables(rule *model.FirewallRule) error {
	args := m.buildIptablesArgs(rule, "-A")
	return runCmd("iptables", args...)
}

func (m *Manager) removeIptables(rule *model.FirewallRule) error {
	args := m.buildIptablesArgs(rule, "-D")
	return runCmd("iptables", args...)
}

func (m *Manager) buildIptablesArgs(rule *model.FirewallRule, op string) []string {
	chain := "INPUT"
	if rule.Direction == "out" {
		chain = "OUTPUT"
	}
	args := []string{op, chain}

	if rule.Interface != "" {
		if rule.Direction == "out" {
			args = append(args, "-o", rule.Interface)
		} else {
			args = append(args, "-i", rule.Interface)
		}
	}
	if rule.Protocol != "" && rule.Protocol != "all" {
		proto := rule.Protocol
		if proto == "tcp+udp" {
			// iptables 不支持 tcp+udp，需要分两条，这里先用 tcp
			proto = "tcp"
		}
		args = append(args, "-p", proto)
	}
	if rule.SrcIP != "" {
		args = append(args, "-s", rule.SrcIP)
	}
	if rule.DstIP != "" {
		args = append(args, "-d", rule.DstIP)
	}
	if rule.Port != "" && rule.Protocol != "icmp" && rule.Protocol != "all" {
		if strings.Contains(rule.Port, "-") {
			args = append(args, "--dport", strings.ReplaceAll(rule.Port, "-", ":"))
		} else {
			args = append(args, "--dport", rule.Port)
		}
	}
	target := "ACCEPT"
	if rule.Action == "deny" {
		target = "DROP"
	}
	args = append(args, "-j", target)
	return args
}

// ─── nftables ────────────────────────────────────────────────────────────────

func (m *Manager) applyNftables(rule *model.FirewallRule) error {
	expr := m.buildNftExpr(rule)
	return runCmd("nft", "add", "rule", "inet", "filter",
		m.nftChain(rule.Direction), expr)
}

func (m *Manager) removeNftables(rule *model.FirewallRule) error {
	// nftables 删除规则需要 handle，这里通过注释名称匹配删除
	// 简化实现：flush 整个链后重新应用所有规则（生产环境建议存储 handle）
	m.log.Warnf("[Firewall] nftables 删除规则：简化实现，仅记录，不实际删除（需手动管理 handle）")
	return nil
}

func (m *Manager) nftChain(direction string) string {
	if direction == "out" {
		return "output"
	}
	return "input"
}

func (m *Manager) buildNftExpr(rule *model.FirewallRule) string {
	var parts []string
	if rule.Protocol != "" && rule.Protocol != "all" {
		proto := rule.Protocol
		if proto == "tcp+udp" {
			proto = "tcp"
		}
		parts = append(parts, fmt.Sprintf("ip protocol %s", proto))
	}
	if rule.SrcIP != "" {
		parts = append(parts, fmt.Sprintf("ip saddr %s", rule.SrcIP))
	}
	if rule.DstIP != "" {
		parts = append(parts, fmt.Sprintf("ip daddr %s", rule.DstIP))
	}
	if rule.Port != "" {
		proto := rule.Protocol
		if proto == "tcp+udp" || proto == "tcp" {
			parts = append(parts, fmt.Sprintf("tcp dport %s", rule.Port))
		} else if proto == "udp" {
			parts = append(parts, fmt.Sprintf("udp dport %s", rule.Port))
		}
	}
	verdict := "accept"
	if rule.Action == "deny" {
		verdict = "drop"
	}
	parts = append(parts, verdict)
	return strings.Join(parts, " ")
}

// ─── ufw ─────────────────────────────────────────────────────────────────────

func (m *Manager) applyUfw(rule *model.FirewallRule) error {
	args := m.buildUfwArgs(rule, false)
	return runCmd("ufw", args...)
}

func (m *Manager) removeUfw(rule *model.FirewallRule) error {
	args := m.buildUfwArgs(rule, true)
	return runCmd("ufw", args...)
}

func (m *Manager) buildUfwArgs(rule *model.FirewallRule, delete bool) []string {
	var args []string
	if delete {
		args = append(args, "delete")
	}
	action := "allow"
	if rule.Action == "deny" {
		action = "deny"
	}
	args = append(args, action)

	dir := "in"
	if rule.Direction == "out" {
		dir = "out"
	}
	args = append(args, dir)

	if rule.Interface != "" {
		args = append(args, "on", rule.Interface)
	}
	if rule.Protocol != "" && rule.Protocol != "all" {
		proto := rule.Protocol
		if proto == "tcp+udp" {
			proto = "any"
		}
		args = append(args, "proto", proto)
	}
	if rule.SrcIP != "" {
		args = append(args, "from", rule.SrcIP)
	} else {
		args = append(args, "from", "any")
	}
	if rule.Port != "" {
		args = append(args, "to", "any", "port", rule.Port)
	} else if rule.DstIP != "" {
		args = append(args, "to", rule.DstIP)
	}
	return args
}

// ─── firewalld ───────────────────────────────────────────────────────────────

func (m *Manager) applyFirewalld(rule *model.FirewallRule) error {
	if rule.Port != "" {
		proto := rule.Protocol
		if proto == "tcp+udp" || proto == "all" {
			proto = "tcp"
		}
		portProto := fmt.Sprintf("%s/%s", rule.Port, proto)
		op := "--add-port"
		if rule.Action == "deny" {
			// firewalld 默认拒绝，允许才需要 add-port
			return nil
		}
		return runCmd("firewall-cmd", "--permanent", op+"="+portProto)
	}
	return nil
}

func (m *Manager) removeFirewalld(rule *model.FirewallRule) error {
	if rule.Port != "" {
		proto := rule.Protocol
		if proto == "tcp+udp" || proto == "all" {
			proto = "tcp"
		}
		portProto := fmt.Sprintf("%s/%s", rule.Port, proto)
		return runCmd("firewall-cmd", "--permanent", "--remove-port="+portProto)
	}
	return nil
}

// ─── OpenWrt (iptables-based) ─────────────────────────────────────────────────

func (m *Manager) applyOpenwrt(rule *model.FirewallRule) error {
	// OpenWrt 使用 iptables，但链名可能不同（INPUT/FORWARD/OUTPUT）
	return m.applyIptables(rule)
}

func (m *Manager) removeOpenwrt(rule *model.FirewallRule) error {
	return m.removeIptables(rule)
}

// ─── Windows ─────────────────────────────────────────────────────────────────

func (m *Manager) applyWindows(rule *model.FirewallRule) error {
	args := m.buildNetshArgs(rule, false)
	return runCmd("netsh", args...)
}

func (m *Manager) removeWindows(rule *model.FirewallRule) error {
	// 通过规则名称删除
	ruleName := m.windowsRuleName(rule)
	return runCmd("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s", ruleName))
}

func (m *Manager) windowsRuleName(rule *model.FirewallRule) string {
	return fmt.Sprintf("NetPanel_%d_%s", rule.ID, rule.Name)
}

func (m *Manager) buildNetshArgs(rule *model.FirewallRule, delete bool) []string {
	ruleName := m.windowsRuleName(rule)
	if delete {
		return []string{"advfirewall", "firewall", "delete", "rule",
			fmt.Sprintf("name=%s", ruleName)}
	}

	dir := "in"
	if rule.Direction == "out" {
		dir = "out"
	}
	action := "allow"
	if rule.Action == "deny" {
		action = "block"
	}

	args := []string{
		"advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", ruleName),
		fmt.Sprintf("dir=%s", dir),
		fmt.Sprintf("action=%s", action),
	}

	proto := rule.Protocol
	if proto == "tcp+udp" {
		proto = "any"
	} else if proto == "all" {
		proto = "any"
	}
	if proto != "" {
		args = append(args, fmt.Sprintf("protocol=%s", proto))
	}
	if rule.Port != "" && proto != "icmp" && proto != "any" {
		args = append(args, fmt.Sprintf("localport=%s", rule.Port))
	}
	if rule.SrcIP != "" {
		args = append(args, fmt.Sprintf("remoteip=%s", rule.SrcIP))
	}
	if rule.Interface != "" {
		args = append(args, fmt.Sprintf("interface=%s", rule.Interface))
	}
	return args
}

// ─── 读取系统现有规则 ──────────────────────────────────────────────────────────

// SystemRule 从系统防火墙读取到的原始规则（用于展示/导入）
type SystemRule struct {
	Name      string `json:"name"`
	Direction string `json:"direction"`
	Action    string `json:"action"`
	Protocol  string `json:"protocol"`
	SrcIP     string `json:"src_ip"`
	DstIP     string `json:"dst_ip"`
	Port      string `json:"port"`
	Interface string `json:"interface"`
	Raw       string `json:"raw"` // 原始命令行输出
}

// ListSystemRules 从当前系统防火墙读取已有规则列表
func (m *Manager) ListSystemRules() ([]SystemRule, error) {
	backend := m.DetectBackend()
	switch backend {
	case "windows":
		return m.listWindowsRules()
	case "ufw":
		return m.listUfwRules()
	case "firewalld":
		return m.listFirewalldRules()
	case "nftables":
		return m.listNftablesRules()
	case "iptables", "openwrt":
		return m.listIptablesRules()
	default:
		return nil, fmt.Errorf("未检测到受支持的防火墙后端")
	}
}

// listWindowsRules 读取 Windows 高级防火墙规则
// 使用 netsh advfirewall firewall show rule name=all verbose
func (m *Manager) listWindowsRules() ([]SystemRule, error) {
	out, err := exec.Command("netsh", "advfirewall", "firewall",
		"show", "rule", "name=all", "verbose").Output()
	if err != nil {
		return nil, fmt.Errorf("读取 Windows 防火墙规则失败: %w", err)
	}
	return parseWindowsRules(string(out)), nil
}

// parseWindowsRules 解析 netsh 输出，每条规则以空行分隔
func parseWindowsRules(output string) []SystemRule {
	var rules []SystemRule
	// 按双换行或"规则名称:"分块
	blocks := splitWindowsBlocks(output)
	for _, block := range blocks {
		rule := parseWindowsBlock(block)
		if rule.Name != "" {
			rules = append(rules, rule)
		}
	}
	return rules
}

func splitWindowsBlocks(output string) []string {
	var blocks []string
	var cur strings.Builder
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if cur.Len() > 0 {
				blocks = append(blocks, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteString(line)
		cur.WriteString("\n")
	}
	if cur.Len() > 0 {
		blocks = append(blocks, cur.String())
	}
	return blocks
}

func parseWindowsBlock(block string) SystemRule {
	rule := SystemRule{Raw: strings.TrimSpace(block)}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// 支持中英文冒号
		idx := strings.IndexAny(line, ":：")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// 兼容中英文字段名
		switch {
		case key == "Rule Name" || key == "规则名称":
			rule.Name = val
		case key == "Direction" || key == "方向":
			if strings.EqualFold(val, "In") || val == "入站" {
				rule.Direction = "in"
			} else {
				rule.Direction = "out"
			}
		case key == "Action" || key == "操作":
			if strings.EqualFold(val, "Allow") || val == "允许" {
				rule.Action = "allow"
			} else {
				rule.Action = "deny"
			}
		case key == "Protocol" || key == "协议":
			rule.Protocol = strings.ToLower(val)
		case key == "LocalPort" || key == "本地端口":
			if val != "Any" && val != "任意" {
				rule.Port = val
			}
		case key == "RemoteIP" || key == "远程 IP":
			if val != "Any" && val != "任意" {
				rule.SrcIP = val
			}
		case key == "LocalIP" || key == "本地 IP":
			if val != "Any" && val != "任意" {
				rule.DstIP = val
			}
		case key == "InterfaceTypes" || key == "接口类型":
			if val != "Any" && val != "任意" {
				rule.Interface = val
			}
		}
	}
	return rule
}

// listIptablesRules 读取 iptables 规则（-S 输出格式）
func (m *Manager) listIptablesRules() ([]SystemRule, error) {
	out, err := exec.Command("iptables", "-S").Output()
	if err != nil {
		return nil, fmt.Errorf("读取 iptables 规则失败: %w", err)
	}
	return parseIptablesOutput(string(out)), nil
}

// parseIptablesOutput 解析 iptables -S 输出
func parseIptablesOutput(output string) []SystemRule {
	var rules []SystemRule
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// 只处理 -A 规则行
		if !strings.HasPrefix(line, "-A ") {
			continue
		}
		rule := SystemRule{Raw: line}
		parts := strings.Fields(line)
		for i := 0; i < len(parts); i++ {
			switch parts[i] {
			case "-A":
				if i+1 < len(parts) {
					chain := parts[i+1]
					if chain == "INPUT" {
						rule.Direction = "in"
					} else if chain == "OUTPUT" {
						rule.Direction = "out"
					}
					i++
				}
			case "-p":
				if i+1 < len(parts) {
					rule.Protocol = parts[i+1]
					i++
				}
			case "-s":
				if i+1 < len(parts) {
					rule.SrcIP = parts[i+1]
					i++
				}
			case "-d":
				if i+1 < len(parts) {
					rule.DstIP = parts[i+1]
					i++
				}
			case "--dport":
				if i+1 < len(parts) {
					rule.Port = strings.ReplaceAll(parts[i+1], ":", "-")
					i++
				}
			case "-i":
				if i+1 < len(parts) {
					rule.Interface = parts[i+1]
					i++
				}
			case "-j":
				if i+1 < len(parts) {
					target := parts[i+1]
					if target == "ACCEPT" {
						rule.Action = "allow"
					} else {
						rule.Action = "deny"
					}
					i++
				}
			}
		}
		// 生成规则名称（用于展示）
		rule.Name = buildRuleName(rule)
		rules = append(rules, rule)
	}
	return rules
}

// listNftablesRules 读取 nftables 规则
func (m *Manager) listNftablesRules() ([]SystemRule, error) {
	out, err := exec.Command("nft", "list", "ruleset").Output()
	if err != nil {
		return nil, fmt.Errorf("读取 nftables 规则失败: %w", err)
	}
	var rules []SystemRule
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ip") && !strings.HasPrefix(line, "tcp") &&
			!strings.HasPrefix(line, "udp") && !strings.HasPrefix(line, "meta") {
			continue
		}
		rule := SystemRule{Raw: line, Protocol: "all"}
		if strings.Contains(line, "accept") {
			rule.Action = "allow"
		} else if strings.Contains(line, "drop") || strings.Contains(line, "reject") {
			rule.Action = "deny"
		} else {
			continue
		}
		if strings.Contains(line, "saddr") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "saddr" && i+1 < len(parts) {
					rule.SrcIP = parts[i+1]
				}
			}
		}
		if strings.Contains(line, "dport") {
			parts := strings.Fields(line)
			for i, p := range parts {
				if p == "dport" && i+1 < len(parts) {
					rule.Port = parts[i+1]
				}
			}
		}
		rule.Name = buildRuleName(rule)
		rules = append(rules, rule)
	}
	return rules, nil
}

// listUfwRules 读取 ufw 规则（ufw status verbose）
func (m *Manager) listUfwRules() ([]SystemRule, error) {
	out, err := exec.Command("ufw", "status", "verbose").Output()
	if err != nil {
		return nil, fmt.Errorf("读取 ufw 规则失败: %w", err)
	}
	var rules []SystemRule
	inRules := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "To ") || strings.HasPrefix(line, "--") {
			inRules = true
			continue
		}
		if !inRules || line == "" {
			continue
		}
		rule := SystemRule{Raw: line, Direction: "in"}
		// 格式：端口/协议  动作  来源
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		// 解析端口/协议
		portProto := parts[0]
		if strings.Contains(portProto, "/") {
			pp := strings.SplitN(portProto, "/", 2)
			rule.Port = pp[0]
			rule.Protocol = pp[1]
		} else {
			rule.Port = portProto
		}
		// 解析动作
		if len(parts) > 1 {
			if strings.EqualFold(parts[1], "ALLOW") {
				rule.Action = "allow"
			} else {
				rule.Action = "deny"
			}
		}
		// 解析来源
		if len(parts) > 2 && parts[2] != "Anywhere" {
			rule.SrcIP = parts[2]
		}
		rule.Name = buildRuleName(rule)
		rules = append(rules, rule)
	}
	return rules, nil
}

// listFirewalldRules 读取 firewalld 开放端口
func (m *Manager) listFirewalldRules() ([]SystemRule, error) {
	out, err := exec.Command("firewall-cmd", "--list-all").Output()
	if err != nil {
		return nil, fmt.Errorf("读取 firewalld 规则失败: %w", err)
	}
	var rules []SystemRule
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ports:") {
			continue
		}
		portStr := strings.TrimPrefix(line, "ports:")
		for _, p := range strings.Fields(portStr) {
			if p == "" {
				continue
			}
			rule := SystemRule{Raw: p, Direction: "in", Action: "allow"}
			if strings.Contains(p, "/") {
				pp := strings.SplitN(p, "/", 2)
				rule.Port = pp[0]
				rule.Protocol = pp[1]
			} else {
				rule.Port = p
			}
			rule.Name = buildRuleName(rule)
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

// buildRuleName 根据规则内容自动生成名称
func buildRuleName(r SystemRule) string {
	action := "拒绝"
	if r.Action == "allow" {
		action = "允许"
	}
	dir := "入站"
	if r.Direction == "out" {
		dir = "出站"
	}
	proto := r.Protocol
	if proto == "" {
		proto = "all"
	}
	port := r.Port
	if port == "" {
		port = "任意端口"
	}
	src := r.SrcIP
	if src == "" {
		src = "任意IP"
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", action, dir, proto, port, src)
}

// ─── 工具函数 ─────────────────────────────────────────────────────────────────

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行 %s %v 失败: %v, 输出: %s", name, args, err, string(out))
	}
	return nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func fileExists(path string) bool {
	_, err := exec.Command("test", "-f", path).Output()
	return err == nil
}
