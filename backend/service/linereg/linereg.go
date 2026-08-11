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
	"fmt"
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

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager 创建线路注册中心。tolerance <= 0 时使用 selector 默认值（50ms）。
func NewManager(db *gorm.DB, log *logrus.Logger, tolerance time.Duration) *Manager {
	if log == nil {
		log = logrus.New()
	}
	return &Manager{
		db:       db,
		log:      log,
		interval: DefaultInterval,
		selector: selector.NewSelector(nil, tolerance),
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
	m.log.Infof("[线路选择] 共 %d 条线路，当前线路: %q", len(lines), sel.LineID)
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
