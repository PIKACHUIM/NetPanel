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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/svcutil"
	"github.com/netpanel/netpanel/service/selector"
)

// tunnelUUIDRe 匹配标准 8-4-4-4-12 格式 UUID（cftunnel 公网入口要求 UUID）
var tunnelUUIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// DefaultInterval 默认的线路刷新与测速间隔。
const DefaultInterval = 60 * time.Second

// 探测参数在 SystemConfig 中的键名。
const (
	cfgKeyIntervalSec      = "probe_interval_sec"
	cfgKeyFailureThreshold = "probe_failure_threshold"
	cfgKeyToleranceMs      = "probe_tolerance_ms"
	cfgKeyMaxConcurrent    = "probe_max_concurrent"
	cfgKeyToolFilter       = "probe_tool_filter"
)

// 探测参数默认值（与 selector 默认一致）。
const (
	defaultIntervalSec      = 60
	defaultFailureThreshold = 2
	defaultToleranceMs      = 50
	defaultMaxConcurrent    = 8
	// minIntervalSec 探测间隔下限：过小的间隔会造成无意义的探测压力，
	// 且 0/负数会使 time.NewTimer 直接 panic
	minIntervalSec = 5
)

// LineHealthEvent selector.HealthEvent 的类型别名：供上层（main）注入健康
// 事件外送（如桥接 callback 任务触发）时引用，避免上层依赖 selector 包。
type LineHealthEvent = selector.HealthEvent

// HealthEventSink 线路健康事件外送通道（SystemLog 之外的可选通知，如触发
// callback 任务）。由上层注入；实现须快速返回（在探测循环内被调用）。
type HealthEventSink func(ev LineHealthEvent)

// Manager 线路注册中心：持有 selector，负责周期刷新线路并驱动测速选线。
type Manager struct {
	db       *gorm.DB
	log      *logrus.Logger
	interval time.Duration
	selector *selector.Selector
	// eventSink 健康转换事件外送（进入不可达 / 恢复时触发），可为 nil。
	eventSink HealthEventSink
	// cfgMu 保护 interval 等运行期可变配置：写方为 HTTP handler goroutine，
	// 读方为后台探测循环，无锁访问会构成 data race。
	cfgMu sync.RWMutex
	// reload 用于通知后台循环配置已变更，需按新间隔重排定时器
	reload chan struct{}

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewManager 创建线路注册中心。tolerance <= 0 时使用 selector 默认值（50ms）。
func NewManager(db *gorm.DB, log *logrus.Logger, tolerance time.Duration) *Manager {
	if log == nil {
		log = logrus.New()
	}
	m := &Manager{
		db:       db,
		log:      log,
		interval: DefaultInterval,
		selector: selector.NewSelector(nil, tolerance),
		reload:   make(chan struct{}, 1),
	}
	// 注册健康转换观察者：进入不可达 / 恢复时写日志、落 SystemLog 并外送事件。
	m.selector.SetHealthObserver(func(ev selector.HealthEvent) {
		m.handleHealthEvent(ev)
	})
	return m
}

// Selector 返回内部选择器，供 API / UI 读取状态或手动锁线。
func (m *Manager) Selector() *selector.Selector {
	return m.selector
}

// SetProber 透传设置探测器（测试注入 / 即时测速复用）。
func (m *Manager) SetProber(p selector.Prober) {
	m.selector.SetProber(p)
}

// SetHealthEventSink 注册健康转换事件外送通道（可选，如桥接 callback 任务）。
// 事件已由 linereg 内部写入运行日志与 SystemLog（Service=linereg），sink 仅
// 用于额外通知；在 Start 前调用即可。
func (m *Manager) SetHealthEventSink(fn HealthEventSink) {
	m.eventSink = fn
}

// SetInterval 设置线路刷新间隔。支持运行期热更新：
// 写入后会唤醒后台循环，使新间隔在下一轮生效，无需重启进程。
func (m *Manager) SetInterval(d time.Duration) {
	if d <= 0 {
		return
	}
	m.cfgMu.Lock()
	m.interval = d
	m.cfgMu.Unlock()

	// 通知后台循环按新间隔重排定时器（非阻塞，避免未启动时卡住）
	select {
	case m.reload <- struct{}{}:
	default:
	}
}

// currentInterval 读取当前刷新间隔（加锁，避免与 SetInterval 形成 data race）。
func (m *Manager) currentInterval() time.Duration {
	m.cfgMu.RLock()
	defer m.cfgMu.RUnlock()
	if m.interval <= 0 {
		return DefaultInterval
	}
	return m.interval
}

// SetMaxConcurrent 透传设置探测并发上限（须在 Start 前调用）。
func (m *Manager) SetMaxConcurrent(n int) {
	m.selector.SetMaxConcurrent(n)
}

// SetFailureThreshold 透传设置连续失败阈值（须在 Start 前调用）。
func (m *Manager) SetFailureThreshold(n int) {
	m.selector.SetFailureThreshold(n)
}

// SetTolerance 透传设置选线防抖容差（须在 Start 前调用）。
func (m *Manager) SetTolerance(d time.Duration) {
	m.selector.SetTolerance(d)
}

// SetToolFilter 设置参与自动选线的工具集合（逗号分隔的工具名；空 = 全部）。
// 透传到 selector，仅 Tool 命中的线路参与自动选线，其余线路仍可展示/手动锁定。
func (m *Manager) SetToolFilter(filter string) {
	var tools []string
	for _, t := range strings.Split(filter, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tools = append(tools, t)
		}
	}
	m.selector.SetToolFilter(tools)
}

// loadConfigInt 读取单个整型配置项，缺失或非法时返回默认值。
//
// 关键：每次查询必须使用**全新的局部变量**。原实现复用同一个
// model.SystemConfig 变量做四次 First 查询，第一次查询后该变量的主键 ID
// 已被填充，GORM 会把主键并入后续查询的 WHERE 条件
// （变成 `WHERE key = ? AND id = <前一条的ID>`），导致后三项配置
// 必然查不到而静默退回默认值——表现为 UI 显示的值与实际运行值不一致。
func loadConfigInt(db *gorm.DB, key string, def int) int {
	var cfg model.SystemConfig
	if err := db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return def
	}
	v, err := strconv.Atoi(cfg.Value)
	if err != nil {
		return def
	}
	return v
}

// loadConfigStr 读取单个字符串配置项，缺失时返回默认值。
// 与 loadConfigInt 相同，必须使用全新的局部变量避免 GORM 主键污染。
func loadConfigStr(db *gorm.DB, key string, def string) string {
	var cfg model.SystemConfig
	if err := db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return def
	}
	return cfg.Value
}

// LoadProbeConfig 从 SystemConfig 读取探测策略四项参数并应用。
// 缺失的键使用默认值。
func (m *Manager) LoadProbeConfig() error {
	if m.db == nil {
		return nil
	}

	intervalSec := loadConfigInt(m.db, cfgKeyIntervalSec, defaultIntervalSec)
	failureThreshold := loadConfigInt(m.db, cfgKeyFailureThreshold, defaultFailureThreshold)
	toleranceMs := loadConfigInt(m.db, cfgKeyToleranceMs, defaultToleranceMs)
	maxConcurrent := loadConfigInt(m.db, cfgKeyMaxConcurrent, defaultMaxConcurrent)

	// 下限校验：interval 为 0 或负数会导致 time.NewTimer panic；
	// 其余参数非正数会使探测行为异常
	if intervalSec < minIntervalSec {
		m.log.Warnf("[线路选择] 探测间隔 %ds 过小，已修正为 %ds", intervalSec, minIntervalSec)
		intervalSec = minIntervalSec
	}
	if failureThreshold <= 0 {
		failureThreshold = defaultFailureThreshold
	}
	if toleranceMs < 0 {
		toleranceMs = defaultToleranceMs
	}
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}
	toolFilter := loadConfigStr(m.db, cfgKeyToolFilter, "")

	m.SetInterval(time.Duration(intervalSec) * time.Second)
	m.SetFailureThreshold(failureThreshold)
	m.SetTolerance(time.Duration(toleranceMs) * time.Millisecond)
	m.SetMaxConcurrent(maxConcurrent)
	// 工具过滤：逗号分隔的工具名列表；空 = 全部工具参与自动选线。
	var tools []string
	for _, t := range strings.Split(toolFilter, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tools = append(tools, t)
		}
	}
	m.selector.SetToolFilter(tools)
	return nil
}

// Start 启动后台守护：立即执行一轮刷新与测速，之后按 interval 周期循环。
func (m *Manager) Start() {
	// 启动前加载探测策略参数（覆盖默认值）
	if m.db != nil {
		if err := m.LoadProbeConfig(); err != nil {
			m.log.Warnf("[线路选择] 加载探测策略失败，使用默认值: %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.wg.Add(1)
	svcutil.SafeGo(m.log, "linereg.run", true, func() { m.run(ctx) })
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

// handleHealthEvent 处理 selector 健康转换事件（进入不可达 / 恢复）：
// 写入运行日志与 SystemLog（Service=linereg），并外送事件供上层通知
// （如触发用户配置的 callback 任务）。由 selector.ProbeAll 的调用方
// goroutine（linereg 后台循环）内触发，需快速返回。
func (m *Manager) handleHealthEvent(ev selector.HealthEvent) {
	msg := fmt.Sprintf("[线路健康] 线路 %s (%s): %s → %s（连续失败 %d 次）",
		ev.LineID, ev.Tool, ev.From, ev.To, ev.ConsecutiveFailures)
	level := "info"
	if ev.To == selector.HealthUnreachable {
		level = "warn"
		m.log.Warn(msg)
	} else {
		m.log.Info(msg)
	}
	if err := m.db.Create(&model.SystemLog{
		Level:   level,
		Service: "linereg",
		Message: msg,
		LogTime: time.Now(),
	}).Error; err != nil {
		m.log.Warnf("[线路健康] SystemLog 写入失败: %v", err)
	}
	if m.eventSink != nil {
		m.eventSink(ev)
	}
}

func (m *Manager) run(ctx context.Context) {
	defer m.wg.Done()
	m.refresh(ctx)

	// 使用 Timer 而非 Ticker：SetInterval 修改间隔后可立即按新值重排，
	// 原实现在 run 入口固定创建 Ticker，导致间隔修改必须重启进程才生效。
	timer := time.NewTimer(m.currentInterval())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.reload:
			// 间隔配置已变更：立即按新间隔重排
			if !timer.Stop() {
				// 抽干已触发但未消费的信号，避免立即重复执行
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(m.currentInterval())
		case <-timer.C:
			svcutil.BeatEngineHeartbeat("linereg")
			m.refresh(ctx)
			timer.Reset(m.currentInterval())
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

// History 返回指定线路的探测历史（延迟趋势），按时间正序排列，供前端趋势图使用。
// limit <= 0 或 > 500 时默认取 100 条；lineID 不存在则返回空 map。
func (m *Manager) History(lineID string, limit int) ([]model.ProbeHistory, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var history []model.ProbeHistory
	if err := m.db.Where("line_id = ?", lineID).
		Order("id desc").Limit(limit).Find(&history).Error; err != nil {
		return nil, err
	}
	// 时间正序（趋势图从左到右）
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}
	return history, nil
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
	// 不注册为线路。named 模式公网入口为 <UUID>.cfargotunnel.com；隧道名称
	// 无法本地解析为 UUID（需调 CF API），未解析出 UUID 前不注册线路，
	// 避免生成恒失败的探测（cftunnel manager 启动时会解析并写回 tunnel_id）。
	var cfts []model.CftunnelConfig
	if err := db.Where("enable = ? AND mode = ?", true, "named").Find(&cfts).Error; err == nil {
		for _, c := range cfts {
			entry := strings.TrimSpace(c.TunnelID)
			if entry == "" && tunnelUUIDRe.MatchString(c.TunnelName) {
				entry = c.TunnelName
			}
			if entry == "" {
				continue
			}
			lines = append(lines, selector.Line{
				ID:      fmt.Sprintf("cftunnel:%d", c.ID),
				Name:    c.Name,
				Tool:    "cloudflare",
				Layer:   "domain",
				Address: entry + ".cfargotunnel.com:443",
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
