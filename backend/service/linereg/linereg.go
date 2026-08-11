// Package linereg 线路注册中心（P2）：
// 把各穿透/组网工具（frp / nps / easytier / wireguard）的可用入口统一
// 注册为 selector.Line，并驱动 selector 定时测速选线。
//
// 设计原则：
//   - 不改动各工具 manager 的现有行为，只做「读配置 -> 汇总入口」；
//   - 与 selector 解耦：本包只依赖 selector 的公开 API；
//   - 线路变更通过 selector.SetLines 全量刷新，自动清理失效线路与锁线。
package linereg

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/selector"
)

// DefaultInterval 默认的线路刷新与测速间隔。
const DefaultInterval = 60 * time.Second

// Manager 线路注册中心：持有 selector，负责周期刷新线路并驱动测速选线。
type Manager struct {
	db       *gorm.DB
	log      *logrus.Logger
	interval time.Duration
	selector *selector.Selector

	// caddyUpdater 选线切换回调：把选中线路的入口同步到绑定的 Caddy 站点。
	// 由上层注入（main.go），避免本包依赖 caddy。
	caddyUpdater func(siteID uint, upstream string) error
	// dnsUpdater 选线切换回调：把服务域名指向选中线路入口 IP（DNS 层切换）。
	// 由上层注入（main.go），避免本包依赖 dnsmasq。
	dnsUpdater func(domain, ip string) error

	// lastUpstream 记录每个 Caddy 站点最近一次成功切换的上游目标（siteID -> upstream）。
	// 切换失败时回滚到该值，避免 Caddy 反代目标与选中线路不一致。
	lastUpstream map[uint]string
	lastMu       sync.Mutex

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager 创建线路注册中心。tolerance <= 0 时使用 selector 默认值（50ms）。
func NewManager(db *gorm.DB, log *logrus.Logger, tolerance time.Duration) *Manager {
	if log == nil {
		log = logrus.New()
	}
	return &Manager{
		db:           db,
		log:          log,
		interval:     DefaultInterval,
		selector:     selector.NewSelector(nil, tolerance),
		lastUpstream: make(map[uint]string),
	}
}

// Selector 返回内部选择器，供 API / UI 读取状态或手动锁线。
func (m *Manager) Selector() *selector.Selector {
	return m.selector
}

// SetInterval 设置线路刷新间隔（须在 Start 前调用）。
func (m *Manager) SetInterval(d time.Duration) {
	if d > 0 {
		m.interval = d
	}
}

// SetMaxConcurrent 透传设置探测并发上限（须在 Start 前调用）。
func (m *Manager) SetMaxConcurrent(n int) {
	m.selector.SetMaxConcurrent(n)
}

// SetFailureThreshold 透传设置连续失败阈值（须在 Start 前调用）。
func (m *Manager) SetFailureThreshold(n int) {
	m.selector.SetFailureThreshold(n)
}

// SetCaddyUpdater 注入 Caddy 反代目标切换回调（域名层切换落地）。
// 选线结果变化时，会把选中线路的入口地址同步到绑定了该线路的 Caddy 站点。
func (m *Manager) SetCaddyUpdater(fn func(siteID uint, upstream string) error) {
	m.caddyUpdater = fn
}

// SetDNSUpdater 注入 DNS 解析切换回调（DNS 层切换落地）。
// 选线结果变化时，会把服务域名指向选中线路入口 IP。
func (m *Manager) SetDNSUpdater(fn func(domain, ip string) error) {
	m.dnsUpdater = fn
}

// Start 启动后台守护：立即执行一轮刷新与测速，之后按 interval 周期循环。
func (m *Manager) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.wg.Add(1)
	go m.run(ctx)
	m.log.Info("[线路选择] 后台测速选线已启动")
}

// Stop 停止后台守护并等待退出。
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.wg.Wait()
	m.log.Info("[线路选择] 后台测速选线已停止")
}

func (m *Manager) run(ctx context.Context) {
	defer m.wg.Done()
	m.refresh(ctx)
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refresh(ctx)
		}
	}
}

// refresh 从数据库重建线路集合，刷新 selector 并执行一轮测速选线。
func (m *Manager) refresh(ctx context.Context) {
	lines := BuildLines(m.db)
	m.selector.SetLines(lines)
	if len(lines) == 0 {
		return
	}
	m.selector.ProbeAll(ctx)
	sel := m.selector.Select()
	m.saveHistory()
	// 切换落地：选线结果同步到绑定的 Caddy 站点（域名层）与 DNS 解析（DNS 层）
	m.applyCaddySwitch(sel.LineID)
	m.applyDNSSwitch(sel.LineID)
	m.log.Infof("[线路选择] 共 %d 条线路，当前线路: %q", len(lines), sel.LineID)
}

// applyCaddySwitch 将当前选中线路的入口地址同步到绑定了该线路的 Caddy 站点。
// 通过 caddyUpdater 回调（main.go 注入）热加载 Caddy 反代目标，实现域名层自动切换。
func (m *Manager) applyCaddySwitch(lineID string) {
	if lineID == "" || m.caddyUpdater == nil {
		return
	}
	// 找到选中线路的地址（作为 Caddy 反代目标入口）
	var line selector.Line
	for _, l := range m.selector.Lines() {
		if l.ID == lineID {
			line = l
			break
		}
	}
	if line.Address == "" {
		return
	}
	// 查找绑定了 Caddy 站点且 LineRefs 包含该线路的服务
	var services []model.TunService
	if err := m.db.Where("caddy_site_id > ?", 0).Find(&services).Error; err != nil {
		m.log.Warnf("[线路选择] 查询绑定 Caddy 的服务失败: %v", err)
		return
	}
	for _, svc := range services {
		var refs []string
		if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err != nil {
			continue
		}
		for _, ref := range refs {
			if ref != lineID {
				continue
			}
			if err := m.caddyUpdater(svc.CaddySiteID, line.Address); err != nil {
				m.log.Errorf("[线路选择] 更新 Caddy 站点 %d 失败: %v", svc.CaddySiteID, err)
				// 回滚到上一次成功切换的上游目标，避免 Caddy 与新线路不一致
				m.lastMu.Lock()
				old, ok := m.lastUpstream[svc.CaddySiteID]
				m.lastMu.Unlock()
				if ok && old != "" && old != line.Address {
					if rerr := m.caddyUpdater(svc.CaddySiteID, old); rerr != nil {
						m.log.Errorf("[线路选择] 回滚 Caddy 站点 %d 到 %s 失败: %v", svc.CaddySiteID, old, rerr)
					} else {
						m.log.Warnf("[线路选择] Caddy 站点 %d 已回滚到 %s", svc.CaddySiteID, old)
					}
				}
				// 记录错误供 UI 排查
				m.db.Model(&model.TunService{}).Where("id = ?", svc.ID).Update("last_error", err.Error())
			} else {
				m.lastMu.Lock()
				m.lastUpstream[svc.CaddySiteID] = line.Address
				m.lastMu.Unlock()
				m.db.Model(&model.TunService{}).Where("id = ?", svc.ID).Update("last_error", "")
				m.log.Infof("[线路选择] Caddy 站点 %d 反代目标已切换为 %s", svc.CaddySiteID, line.Address)
			}
			break
		}
	}
}

// applyDNSSwitch 将服务域名指向当前选中线路的入口 IP（DNS 层切换）。
// 仅当服务配置了 Domain 且线路入口为 IP 时生效（域名入口走 Caddy 域名层）。
// 通过 dnsUpdater 回调（main.go 注入 dnsmasq.SetRecord）写入自定义解析记录。
func (m *Manager) applyDNSSwitch(lineID string) {
	if lineID == "" || m.dnsUpdater == nil {
		return
	}
	// 找到选中线路的地址，仅对 IP 入口做 DNS 切换
	var line selector.Line
	for _, l := range m.selector.Lines() {
		if l.ID == lineID {
			line = l
			break
		}
	}
	host := lineHost(line.Address)
	if host == "" || net.ParseIP(host) == nil {
		return
	}
	// 查找配置了 Domain 且 LineRefs 包含该线路的服务
	var services []model.TunService
	if err := m.db.Where("domain != ?", "").Find(&services).Error; err != nil {
		m.log.Warnf("[线路选择] 查询配置域名服务失败: %v", err)
		return
	}
	for _, svc := range services {
		var refs []string
		if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err != nil {
			continue
		}
		for _, ref := range refs {
			if ref != lineID {
				continue
			}
			if err := m.dnsUpdater(svc.Domain, host); err != nil {
				m.log.Warnf("[线路选择] 更新 DNS 解析 %s -> %s 失败: %v", svc.Domain, host, err)
				// 记录错误供 UI 排查
				m.db.Model(&model.TunService{}).Where("id = ?", svc.ID).Update("last_error", err.Error())
			} else {
				m.db.Model(&model.TunService{}).Where("id = ?", svc.ID).Update("last_error", "")
				m.log.Infof("[线路选择] DNS 解析 %s -> %s 已切换", svc.Domain, host)
			}
			break
		}
	}
}

// lineHost 从 host:port 地址中提取 host（无端口时视为纯 host）。
func lineHost(address string) string {
	if address == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return address
	}
	return host
}

// maxHistoryPerLine 每条线路保留的探测历史上限
const maxHistoryPerLine = 200

// saveHistory 将最近一次探测结果写入历史表，并清理每线路超出上限的最旧记录。
func (m *Manager) saveHistory() {
	st := m.selector.Snapshot()
	lines := st.Lines
	for id, r := range st.Results {
		var line selector.Line
		for _, l := range lines {
			if l.ID == id {
				line = l
				break
			}
		}
		rec := model.ProbeHistory{
			LineID:      id,
			Tool:        line.Tool,
			Layer:       line.Layer,
			Address:     line.Address,
			TCPLatency:  int64(r.TCPLatency),
			HTTPLatency: int64(r.HTTPLatency),
			Available:   r.Err == nil,
		}
		if r.Err != nil {
			rec.ErrorMsg = r.Err.Error()
		}
		if err := m.db.Create(&rec).Error; err != nil {
			m.log.Warnf("[线路选择] 写入探测历史失败 (%s): %v", id, err)
			continue
		}
		m.pruneHistory(id)
	}
}

// pruneHistory 删除指定线路超出上限的最旧历史记录。
func (m *Manager) pruneHistory(lineID string) {
	var count int64
	if err := m.db.Model(&model.ProbeHistory{}).Where("line_id = ?", lineID).Count(&count).Error; err != nil {
		return
	}
	if count <= maxHistoryPerLine {
		return
	}
	excess := count - maxHistoryPerLine
	m.db.Where("line_id = ?", lineID).Order("id asc").Limit(int(excess)).Delete(&model.ProbeHistory{})
}

// BuildLines 从数据库汇总各工具「启用且入口可用」的线路。
//
// 各工具入口来源：
//   - frp      客户端配置的 frps 地址（ServerAddr:ServerPort）
//   - nps      客户端配置的 nps 桥接地址（ServerAddr:ServerPort）
//   - easytier 客户端 ServerAddr 中的每个地址（tcp://ip:port，可多个）
//   - wireguard 启用的对端节点 Endpoint（host:port）
//   - cftunnel named 模式的隧道入口（{tunnel_name}.cfargotunnel.com:443）；
//     quick/token 模式无固定外部入口，不注册为线路
func BuildLines(db *gorm.DB) []selector.Line {
	if db == nil {
		return nil
	}
	var lines []selector.Line

	// ---- frp 客户端 ----
	var frpcs []model.FrpcConfig
	if err := db.Where("enable = ?", true).Find(&frpcs).Error; err == nil {
		for _, c := range frpcs {
			addr := joinHostPort(c.ServerAddr, c.ServerPort)
			if addr == "" {
				continue
			}
			lines = append(lines, selector.Line{
				ID:      fmt.Sprintf("frp:%d", c.ID),
				Name:    c.Name,
				Tool:    "frp",
				Address: addr,
			})
		}
	}

	// ---- nps 客户端 ----
	var npscs []model.NpsClientConfig
	if err := db.Where("enable = ?", true).Find(&npscs).Error; err == nil {
		for _, c := range npscs {
			addr := joinHostPort(c.ServerAddr, c.ServerPort)
			if addr == "" {
				continue
			}
			lines = append(lines, selector.Line{
				ID:      fmt.Sprintf("nps:%d", c.ID),
				Name:    c.Name,
				Tool:    "nps",
				Address: addr,
			})
		}
	}

	// ---- easytier 客户端（ServerAddr 可含多个入口，逗号分隔）----
	var ets []model.EasytierClient
	if err := db.Where("enable = ?", true).Find(&ets).Error; err == nil {
		for _, c := range ets {
			for i, raw := range strings.Split(c.ServerAddr, ",") {
				host := stripScheme(strings.TrimSpace(raw))
				if host == "" {
					continue
				}
				lines = append(lines, selector.Line{
					ID:      fmt.Sprintf("easytier:%d:%d", c.ID, i),
					Name:    c.Name,
					Tool:    "easytier",
					Address: host,
				})
			}
		}
	}

	// ---- wireguard 对端节点（Endpoint 为可探测的远端入口）----
	var wgcs []model.WireguardConfig
	if err := db.Where("enable = ?", true).Find(&wgcs).Error; err == nil {
		for _, c := range wgcs {
			var peers []model.WireguardPeer
			if err := db.Where("wireguard_id = ? AND enable = ?", c.ID, true).Find(&peers).Error; err != nil {
				continue
			}
			for _, p := range peers {
				if p.Endpoint == "" {
					continue
				}
				lines = append(lines, selector.Line{
					ID:      fmt.Sprintf("wg:%d", p.ID),
					Name:    p.Name,
					Tool:    "wireguard",
					Address: p.Endpoint,
				})
			}
		}
	}

	// ---- Cloudflare Tunnel（named 模式入口固定，可探测）----
	// quick/token 模式无固定外部入口（trycloudflare 随机域名 / 远程配置），
	// 不注册为线路；named 模式用 {tunnel_name}.cfargotunnel.com:443 探测。
	var cfts []model.CftunnelConfig
	if err := db.Where("enable = ? AND mode = ?", true, "named").Find(&cfts).Error; err == nil {
		for _, c := range cfts {
			if c.TunnelName == "" {
				continue
			}
			lines = append(lines, selector.Line{
				ID:      fmt.Sprintf("cftunnel:%d", c.ID),
				Name:    c.Name,
				Tool:    "cloudflare",
				Layer:   "domain",
				Address: c.TunnelName + ".cfargotunnel.com:443",
			})
		}
	}

	return lines
}

// joinHostPort 拼接 host:port；host 或 port 非法时返回空串。
func joinHostPort(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" || port <= 0 || port > 65535 {
		return ""
	}
	return host + ":" + strconv.Itoa(port)
}

// stripScheme 去掉地址前缀的协议与路径，仅保留 host:port。
// 例如 "tcp://1.2.3.4:11010" -> "1.2.3.4:11010"，"udp://x:11010/path" -> "x:11010"。
func stripScheme(raw string) string {
	if i := strings.Index(raw, "://"); i >= 0 {
		raw = raw[i+3:]
	}
	if i := strings.IndexByte(raw, '/'); i >= 0 {
		raw = raw[:i]
	}
	return strings.TrimSpace(raw)
}
