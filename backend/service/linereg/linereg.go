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

// 探测参数在 SystemConfig 中的键名。
const (
	cfgKeyIntervalSec      = "probe_interval_sec"
	cfgKeyFailureThreshold = "probe_failure_threshold"
	cfgKeyToleranceMs      = "probe_tolerance_ms"
	cfgKeyMaxConcurrent    = "probe_max_concurrent"
	cfgKeyToolFilter       = "probe_tool_filter"
	cfgKeyRebindMode       = "port_rebind_mode"
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

	// caddyUpdater 选线切换回调：把选中线路的入口同步到绑定的 Caddy 站点。
	// 由上层注入（main.go），避免本包依赖 caddy。
	caddyUpdater func(siteID uint, upstream string) error
	// dnsUpdater 选线切换回调：把服务域名指向选中线路入口 IP（DNS 层切换）。
	// 由上层注入（main.go），避免本包依赖 dnsmasq。
	dnsUpdater func(domain, ip string) error
	// portRebinder 选线切换回调：重建端口层服务的穿透规则（先停后起，
	// 短暂抖动可接受），把本地监听入口重新绑定到当前选中线路。
	// 仅作用于未绑定 Caddy 站点、未配置域名的纯 TCP/UDP 端口层服务。
	// 由上层注入（main.go），避免本包依赖各工具 manager。
	portRebinder func(svcID uint, lineID string) error

	// lastUpstream 记录每个 Caddy 站点最近一次成功切换的上游目标（siteID -> upstream）。
	// 切换失败时回滚到该值，避免 Caddy 反代目标与选中线路不一致。
	lastUpstream map[uint]string
	lastMu       sync.Mutex

	// rebindMode 端口层重绑模式："auto"（选线变化自动重绑，默认）/
	// "manual"（半自动：只记录待重绑清单，由用户手动触发）/ "off"（关闭重绑）。
	rebindMode string
	// pendingRebinds manual 模式下记录待重绑的服务（svcID -> 目标线路）。
	// 每次选线变化时更新；用户手动触发后清空。
	pendingRebinds map[uint]string
	pendingMu      sync.Mutex
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
		db:             db,
		log:            log,
		interval:       DefaultInterval,
		selector:       selector.NewSelector(nil, tolerance),
		lastUpstream:   make(map[uint]string),
		rebindMode:     RebindModeAuto,
		pendingRebinds: make(map[uint]string),
		reload:         make(chan struct{}, 1),
	}
	// 注册健康转换观察者：进入不可达 / 恢复时写日志、落 SystemLog 并外送事件。
	m.selector.SetHealthObserver(func(ev selector.HealthEvent) {
		m.handleHealthEvent(ev)
	})
	return m
}

// 端口层重绑模式取值。
const (
	RebindModeAuto   = "auto"   // 选线变化自动重绑（默认）
	RebindModeManual = "manual" // 半自动：只记录待重绑清单，由用户手动触发
	RebindModeOff    = "off"    // 关闭端口层重绑
)

// SetRebindMode 设置端口层重绑模式。非法值忽略。
func (m *Manager) SetRebindMode(mode string) {
	switch mode {
	case RebindModeAuto, RebindModeManual, RebindModeOff:
		m.rebindMode = mode
	}
}

// RebindMode 返回当前端口层重绑模式。
func (m *Manager) RebindMode() string {
	if m.rebindMode == "" {
		return RebindModeAuto
	}
	return m.rebindMode
}

// PendingRebinds 返回 manual 模式下待重绑的服务（svcID -> 目标线路）快照。
func (m *Manager) PendingRebinds() map[uint]string {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()
	out := make(map[uint]string, len(m.pendingRebinds))
	for k, v := range m.pendingRebinds {
		out[k] = v
	}
	return out
}

// ApplyPendingRebinds 手动触发所有待重绑服务（manual 模式使用）。
// 逐个调用 portRebinder；全部成功后清空待重绑清单，返回处理数量。
func (m *Manager) ApplyPendingRebinds() (int, error) {
	if m.portRebinder == nil {
		return 0, fmt.Errorf("端口层重绑回调未注入")
	}
	pending := m.PendingRebinds()
	applied := 0
	var firstErr error
	for svcID, lineID := range pending {
		if err := m.portRebinder(svcID, lineID); err != nil {
			m.log.Errorf("[线路选择] 手动重绑端口层服务 %d 到线路 %s 失败: %v", svcID, lineID, err)
			m.db.Model(&model.TunService{}).Where("id = ?", svcID).Update("last_error", err.Error())
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		m.db.Model(&model.TunService{}).Where("id = ?", svcID).Update("last_error", "")
		m.log.Infof("[线路选择] 手动重绑端口层服务 %d 到线路 %s 成功", svcID, lineID)
		applied++
	}
	if firstErr == nil {
		m.pendingMu.Lock()
		m.pendingRebinds = make(map[uint]string)
		m.pendingMu.Unlock()
	}
	return applied, firstErr
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
	rebindMode := loadConfigStr(m.db, cfgKeyRebindMode, RebindModeAuto)

	m.SetInterval(time.Duration(intervalSec) * time.Second)
	m.SetFailureThreshold(failureThreshold)
	m.SetTolerance(time.Duration(toleranceMs) * time.Millisecond)
	m.SetMaxConcurrent(maxConcurrent)
	m.SetRebindMode(rebindMode)
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

// SetPortRebinder 注入端口层切换回调（端口层切换落地）。
// 选线结果变化时，会重建未绑定 Caddy/DNS 的纯 TCP/UDP 服务的穿透规则，
// 使本地监听入口跟随当前选中线路。
func (m *Manager) SetPortRebinder(fn func(svcID uint, lineID string) error) {
	m.portRebinder = fn
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
	// 切换落地：选线结果同步到绑定的 Caddy 站点（域名层）、DNS 解析（DNS 层）
	// 与未绑定 Caddy/DNS 的端口层服务（端口层重绑）
	m.applyCaddySwitch(sel.LineID)
	m.applyDNSSwitch(sel.LineID)
	m.applyPortSwitch(sel.LineID)
	m.log.Infof("[线路选择] 共 %d 条线路，当前线路: %q", len(lines), sel.LineID)
}

// effectiveLine 返回某服务在本次选线中的有效线路。
// 若服务设置了 LockedLine 且该线路存在于其 LineRefs，则使用 LockedLine
// （服务级锁线优先于全局选线）；否则使用全局选中线路 globalLineID。
func (m *Manager) effectiveLine(svc model.TunService, globalLineID string) string {
	if svc.LockedLine != "" {
		var refs []string
		if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err == nil {
			for _, ref := range refs {
				if ref == svc.LockedLine {
					return svc.LockedLine
				}
			}
		}
	}
	return globalLineID
}

// applyCaddySwitch 将当前选中线路的入口地址同步到绑定了该线路的 Caddy 站点。
// 通过 caddyUpdater 回调（main.go 注入）热加载 Caddy 反代目标，实现域名层自动切换。
func (m *Manager) applyCaddySwitch(lineID string) {
	if lineID == "" || m.caddyUpdater == nil {
		return
	}
	// 查找绑定了 Caddy 站点的服务
	var services []model.TunService
	if err := m.db.Where("caddy_site_id > ?", 0).Find(&services).Error; err != nil {
		m.log.Warnf("[线路选择] 查询绑定 Caddy 的服务失败: %v", err)
		return
	}
	for _, svc := range services {
		// 服务级锁线：若 LockedLine 存在于 LineRefs，使用锁定线路；否则用全局线路
		effLineID := m.effectiveLine(svc, lineID)
		if effLineID == "" {
			continue
		}
		// 找到有效线路的地址（作为 Caddy 反代目标入口）
		var line selector.Line
		for _, l := range m.selector.Lines() {
			if l.ID == effLineID {
				line = l
				break
			}
		}
		if line.Address == "" {
			continue
		}
		// 校验该有效线路确实在服务的 LineRefs 中
		var refs []string
		if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err != nil {
			continue
		}
		found := false
		for _, ref := range refs {
			if ref == effLineID {
				found = true
				break
			}
		}
		if !found {
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
	}
}

// applyDNSSwitch 将服务域名指向当前选中线路的入口 IP（DNS 层切换）。
// 仅当服务配置了 Domain 且线路入口为 IP 时生效（域名入口走 Caddy 域名层）。
// 通过 dnsUpdater 回调（main.go 注入 dnsmasq.SetRecord）写入自定义解析记录。
func (m *Manager) applyDNSSwitch(lineID string) {
	if lineID == "" || m.dnsUpdater == nil {
		return
	}
	// 查找配置了 Domain 的服务
	var services []model.TunService
	if err := m.db.Where("domain != ?", "").Find(&services).Error; err != nil {
		m.log.Warnf("[线路选择] 查询配置域名服务失败: %v", err)
		return
	}
	for _, svc := range services {
		// 服务级锁线：若 LockedLine 存在于 LineRefs，使用锁定线路；否则用全局线路
		effLineID := m.effectiveLine(svc, lineID)
		if effLineID == "" {
			continue
		}
		// 找到有效线路的地址，仅对 IP 入口做 DNS 切换
		var line selector.Line
		for _, l := range m.selector.Lines() {
			if l.ID == effLineID {
				line = l
				break
			}
		}
		host := lineHost(line.Address)
		if host == "" || net.ParseIP(host) == nil {
			continue
		}
		// 校验该有效线路确实在服务的 LineRefs 中
		var refs []string
		if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err != nil {
			continue
		}
		found := false
		for _, ref := range refs {
			if ref == effLineID {
				found = true
				break
			}
		}
		if !found {
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
	}
}

// applyPortSwitch 将当前选中线路同步到端口层服务（端口层切换落地）。
// 端口层服务 = 未绑定 Caddy 站点（caddy_site_id=0）且未配置域名
// （domain 为空）的纯 TCP/UDP 服务——HTTP/HTTPS 走域名层（Caddy），
// 带域名的走 DNS 层，其余走端口映射。选线变化时重建受影响规则，
// 把本地监听入口重新绑定到当前线路（先停后起，短暂抖动可接受）。
func (m *Manager) applyPortSwitch(lineID string) {
	if lineID == "" || m.portRebinder == nil {
		return
	}
	// off 模式：关闭端口层重绑，直接返回。
	if m.RebindMode() == RebindModeOff {
		return
	}
	var services []model.TunService
	if err := m.db.Where("caddy_site_id = ? AND (domain IS NULL OR domain = '')", 0).Find(&services).Error; err != nil {
		m.log.Warnf("[线路选择] 查询端口层服务失败: %v", err)
		return
	}
	for _, svc := range services {
		// 服务级锁线：若 LockedLine 存在于 LineRefs，使用锁定线路；否则用全局线路
		effLineID := m.effectiveLine(svc, lineID)
		if effLineID == "" {
			continue
		}
		// 校验该有效线路确实在服务的 LineRefs 中
		var refs []string
		if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err != nil {
			continue
		}
		found := false
		for _, ref := range refs {
			if ref == effLineID {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		// manual 模式：只记录待重绑清单（svcID -> 目标线路），不自动停起；
		// 由用户通过 ApplyPendingRebinds 手动触发（避免自动重建带来抖动）。
		if m.RebindMode() == RebindModeManual {
			m.pendingMu.Lock()
			m.pendingRebinds[svc.ID] = effLineID
			m.pendingMu.Unlock()
			m.log.Infof("[线路选择] manual 模式：端口层服务 %d 待重绑到线路 %s（等待手动触发）", svc.ID, effLineID)
			continue
		}
		if err := m.portRebinder(svc.ID, effLineID); err != nil {
			m.log.Errorf("[线路选择] 端口层服务 %d 重绑线路 %s 失败: %v", svc.ID, effLineID, err)
			// 记录错误供 UI 排查
			m.db.Model(&model.TunService{}).Where("id = ?", svc.ID).Update("last_error", err.Error())
		} else {
			m.db.Model(&model.TunService{}).Where("id = ?", svc.ID).Update("last_error", "")
			m.log.Infof("[线路选择] 端口层服务 %d 已重绑到线路 %s", svc.ID, effLineID)
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
