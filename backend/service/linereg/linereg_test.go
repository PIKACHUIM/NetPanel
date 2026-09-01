package linereg

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/selector"
)

// newTestDB 创建内存 SQLite 并迁移所需模型。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(
		&model.FrpcConfig{},
		&model.NpsClientConfig{},
		&model.EasytierClient{},
		&model.WireguardConfig{},
		&model.WireguardPeer{},
		&model.CftunnelConfig{},
		&model.ProbeHistory{},
		&model.TunService{},
	); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

func seedData(db *gorm.DB) {
	db.Create(&model.FrpcConfig{Name: "阿里 frps", Enable: true, ServerAddr: "1.2.3.4", ServerPort: 7000})
	db.Create(&model.FrpcConfig{Name: "停用线路", Enable: false, ServerAddr: "9.9.9.9", ServerPort: 7000})
	// 端口非法：ServerPort 带 default tag（7000），Create 时零值会被默认值覆盖，
	// 需用 Update 强制写入 0（Update 不受 default tag 影响）。
	frpc3 := model.FrpcConfig{Name: "端口非法", Enable: true, ServerAddr: "8.8.8.8", ServerPort: 7000}
	db.Create(&frpc3)
	db.Model(&frpc3).Update("server_port", 0)
	db.Create(&model.NpsClientConfig{Name: "nps 节点", Enable: true, ServerAddr: "5.6.7.8", ServerPort: 8024})
	db.Create(&model.EasytierClient{Name: "et 双入口", Enable: true, ServerAddr: "tcp://10.0.0.1:11010, tcp://10.0.0.2:11010"})
	db.Create(&model.EasytierClient{Name: "et 停用", Enable: false, ServerAddr: "tcp://10.0.0.9:11010"})

	wg := model.WireguardConfig{Name: "wg 接口", Enable: true, ListenPort: 51820, Address: "10.0.0.1/24"}
	db.Create(&wg)
	db.Create(&model.WireguardPeer{WireguardID: wg.ID, Name: "远端 A", Enable: true, PublicKey: "AAA", Endpoint: "1.2.3.5:51820"})
	// 远端 B 停用：Enable 带 default tag（true），Create 时零值会被默认值覆盖，
	// 需用 Update 强制写入 false。
	peerB := model.WireguardPeer{WireguardID: wg.ID, Name: "远端 B", Enable: true, PublicKey: "BBB", Endpoint: "1.2.3.6:51820"}
	db.Create(&peerB)
	db.Model(&peerB).Update("enable", false)
	db.Create(&model.WireguardPeer{WireguardID: wg.ID, Name: "无端点", Enable: true, PublicKey: "CCC", Endpoint: ""})

	// CF 隧道：仅 named 模式注册为线路
	db.Create(&model.CftunnelConfig{Name: "cf named", Enable: true, Mode: "named", TunnelName: "my-tunnel", LocalURL: "http://127.0.0.1:8080"})
	db.Create(&model.CftunnelConfig{Name: "cf quick", Enable: true, Mode: "quick", LocalURL: "http://127.0.0.1:8081"})
	db.Create(&model.CftunnelConfig{Name: "cf token", Enable: true, Mode: "token", Token: "eyJhIjoi"})
	db.Create(&model.CftunnelConfig{Name: "cf 停用", Enable: false, Mode: "named", TunnelName: "off-tunnel"})
	db.Create(&model.CftunnelConfig{Name: "cf 无名", Enable: true, Mode: "named", TunnelName: ""})
}

func TestBuildLines(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	lines := BuildLines(db)

	byID := make(map[string]selector.Line, len(lines))
	for _, l := range lines {
		byID[l.ID] = l
	}

	// frp：启用且端口合法 -> 1 条；停用/端口非法不收集
	if l, ok := byID["frp:1"]; !ok {
		t.Error("缺少 frp:1")
	} else if l.Address != "1.2.3.4:7000" || l.Tool != "frp" {
		t.Errorf("frp:1 地址/工具错误: %+v", l)
	}
	for _, id := range []string{"frp:2", "frp:3"} {
		if _, ok := byID[id]; ok {
			t.Errorf("%s 不应被收集", id)
		}
	}

	// nps
	if l, ok := byID["nps:1"]; !ok {
		t.Error("缺少 nps:1")
	} else if l.Address != "5.6.7.8:8024" || l.Tool != "nps" {
		t.Errorf("nps:1 地址/工具错误: %+v", l)
	}

	// easytier：双入口生成两条，协议前缀被剥离
	if _, ok := byID["easytier:1:0"]; !ok {
		t.Error("缺少 easytier:1:0")
	}
	if l, ok := byID["easytier:1:1"]; !ok {
		t.Error("缺少 easytier:1:1")
	} else if l.Address != "10.0.0.2:11010" || l.Tool != "easytier" {
		t.Errorf("easytier:1:1 地址/工具错误: %+v", l)
	}
	if _, ok := byID["easytier:2:0"]; ok {
		t.Error("停用的 easytier 不应被收集")
	}

	// wireguard：仅启用且有 Endpoint 的对端
	if l, ok := byID["wg:1"]; !ok {
		t.Error("缺少 wg:1")
	} else if l.Address != "1.2.3.5:51820" || l.Tool != "wireguard" {
		t.Errorf("wg:1 地址/工具错误: %+v", l)
	}
	for _, id := range []string{"wg:2", "wg:3"} {
		if _, ok := byID[id]; ok {
			t.Errorf("%s 不应被收集", id)
		}
	}

	// cftunnel：仅 named 模式且有隧道名的注册为线路
	if l, ok := byID["cftunnel:1"]; !ok {
		t.Error("缺少 cftunnel:1")
	} else if l.Address != "my-tunnel.cfargotunnel.com:443" || l.Tool != "cloudflare" {
		t.Errorf("cftunnel:1 地址/工具错误: %+v", l)
	} else if l.Layer != "domain" {
		t.Errorf("cftunnel:1 应为域名层(domain), got %q", l.Layer)
	}
	for _, id := range []string{"cftunnel:2", "cftunnel:3", "cftunnel:4", "cftunnel:5"} {
		if _, ok := byID[id]; ok {
			t.Errorf("%s 不应被收集（quick/token/停用/无名）", id)
		}
	}
}

func TestBuildLinesNilDB(t *testing.T) {
	if lines := BuildLines(nil); len(lines) != 0 {
		t.Fatalf("nil db 应返回空列表, got %d", len(lines))
	}
}

// fakeProber 注入式探测器：全部返回固定延迟，用于验证 Manager 接线。
type fakeProber struct {
	latencies map[string]time.Duration
}

func (f *fakeProber) Probe(_ context.Context, line selector.Line) selector.ProbeResult {
	lat := time.Duration(0)
	if f.latencies != nil {
		lat = f.latencies[line.ID]
	}
	if lat <= 0 {
		lat = 10 * time.Millisecond
	}
	return selector.ProbeResult{LineID: line.ID, TCPLatency: lat}
}

func TestManagerRefreshWiresLines(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	m := NewManager(db, nil, 0)
	m.selector = selector.NewSelector(&fakeProber{}, 0)
	m.SetFailureThreshold(1)
	m.refresh(context.Background())

	sel := m.Selector().Select()
	if sel.LineID == "" {
		t.Fatal("refresh 后应有可用线路")
	}
	if len(m.Selector().Lines()) == 0 {
		t.Fatal("refresh 后线路集合不应为空")
	}
	// 内部状态应包含全部有效线路（frp1 + nps1 + et2 + wg1 + cftunnel1 = 6 条）
	if got := len(m.Selector().Lines()); got != 6 {
		t.Fatalf("期望 6 条线路, got %d", got)
	}
}

func TestStripScheme(t *testing.T) {
	cases := map[string]string{
		"tcp://1.2.3.4:11010": "1.2.3.4:11010",
		"udp://x:11011":       "x:11011",
		"tcp://h:11010/path":  "h:11010",
		"  1.2.3.4:11010  ":   "1.2.3.4:11010",
		"":                    "",
	}
	for in, want := range cases {
		if got := stripScheme(in); got != want {
			t.Errorf("stripScheme(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRefreshWritesProbeHistory(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	m := NewManager(db, nil, 0)
	m.selector = selector.NewSelector(&fakeProber{}, 0)
	m.refresh(context.Background())

	var count int64
	if err := db.Model(&model.ProbeHistory{}).Count(&count).Error; err != nil {
		t.Fatalf("查询历史失败: %v", err)
	}
	// seedData 共 6 条有效线路，refresh 后每线路应写入一条历史
	if count != 6 {
		t.Fatalf("期望 6 条探测历史, got %d", count)
	}

	var rec model.ProbeHistory
	if err := db.Where("line_id = ?", "cftunnel:1").First(&rec).Error; err != nil {
		t.Fatalf("读取 cftunnel:1 历史失败: %v", err)
	}
	if rec.Tool != "cloudflare" || rec.Layer != "domain" {
		t.Errorf("历史记录工具/层次错误: %+v", rec)
	}
	if !rec.Available {
		t.Errorf("fakeProber 全部成功，期望 Available=true, got %+v", rec)
	}
}

func TestRefreshPrunesHistory(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	m := NewManager(db, nil, 0)
	m.selector = selector.NewSelector(&fakeProber{}, 0)

	// 循环刷新，历史应被 pruneHistory 限制在 maxHistoryPerLine 以内
	rounds := maxHistoryPerLine + 20
	for i := 0; i < rounds; i++ {
		m.refresh(context.Background())
	}
	var count int64
	if err := db.Model(&model.ProbeHistory{}).Where("line_id = ?", "frp:1").Count(&count).Error; err != nil {
		t.Fatalf("查询历史失败: %v", err)
	}
	if count > maxHistoryPerLine {
		t.Fatalf("历史超出上限: got %d, want <= %d", count, maxHistoryPerLine)
	}
}

func TestApplyCaddySwitch(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	// 创建绑定 Caddy 站点(站点 ID=9)且关联 cftunnel:1 线路的穿透服务
	db.Create(&model.TunService{
		Name:          "测试服务",
		Enable:        true,
		TargetAddress: "127.0.0.1",
		TargetPort:    8080,
		LineRefs:      `["frp:1","cftunnel:1"]`,
		CaddySiteID:   9,
	})

	// 注入 Caddy 切换回调，记录调用
	var gotSiteID uint
	var gotUpstream string
	var calls int
	m := NewManager(db, nil, 0)
	m.selector = selector.NewSelector(&fakeProber{latencies: map[string]time.Duration{
		"frp:1": 1 * time.Millisecond,
	}}, 0)
	m.SetCaddyUpdater(func(siteID uint, upstream string) error {
		calls++
		gotSiteID = siteID
		gotUpstream = upstream
		return nil
	})

	m.refresh(context.Background())

	// fakeProber 全部线路延迟相同，应选到第一条 frp:1
	if calls == 0 {
		t.Fatal("期望触发 Caddy 切换回调")
	}
	if gotSiteID != 9 {
		t.Errorf("期望更新站点 9, got %d", gotSiteID)
	}
	if gotUpstream != "1.2.3.4:7000" {
		t.Errorf("期望上游指向 frp:1 的地址 1.2.3.4:7000, got %q", gotUpstream)
	}
}

func TestApplyCaddySwitchNoBinding(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	// 服务未绑定 Caddy 站点（CaddySiteID=0），不应触发回调
	db.Create(&model.TunService{
		Name:          "无绑定服务",
		Enable:        true,
		TargetAddress: "127.0.0.1",
		TargetPort:    8080,
		LineRefs:      `["frp:1"]`,
	})

	calls := 0
	m := NewManager(db, nil, 0)
	m.selector = selector.NewSelector(&fakeProber{}, 0)
	m.SetCaddyUpdater(func(siteID uint, upstream string) error {
		calls++
		return nil
	})

	m.refresh(context.Background())
	if calls != 0 {
		t.Fatalf("未绑定 Caddy 站点不应触发切换, got %d calls", calls)
	}
}

func TestApplyDNSSwitch(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	// 创建配置了 Domain 且关联 frp:1（入口 1.2.3.4:7000，IP 地址）的穿透服务
	db.Create(&model.TunService{
		Name:          "DNS 服务",
		Enable:        true,
		TargetAddress: "127.0.0.1",
		TargetPort:    8080,
		Domain:        "nas.example.com",
		LineRefs:      `["frp:1","cftunnel:1"]`,
	})

	// 注入 DNS 切换回调，记录调用
	var gotDomain, gotIP string
	var calls int
	m := NewManager(db, nil, 0)
	m.selector = selector.NewSelector(&fakeProber{latencies: map[string]time.Duration{
		"frp:1": 1 * time.Millisecond,
	}}, 0)
	m.SetDNSUpdater(func(domain, ip string) error {
		calls++
		gotDomain = domain
		gotIP = ip
		return nil
	})

	m.refresh(context.Background())

	if calls == 0 {
		t.Fatal("期望触发 DNS 切换回调")
	}
	if gotDomain != "nas.example.com" {
		t.Errorf("期望域名 nas.example.com, got %q", gotDomain)
	}
	if gotIP != "1.2.3.4" {
		t.Errorf("期望解析到 1.2.3.4（frp:1 入口 IP）, got %q", gotIP)
	}
}

func TestApplyDNSSwitchDomainEntrySkipped(t *testing.T) {
	db := newTestDB(t)
	seedData(db)

	// 服务配置了 Domain，但选中线路 cftunnel:1 是域名入口（cfargotunnel.com），
	// 无 IP 可解析，不应触发 DNS 切换（域名层走 Caddy）
	db.Create(&model.TunService{
		Name:          "域名入口服务",
		Enable:        true,
		TargetAddress: "127.0.0.1",
		TargetPort:    8080,
		Domain:        "web.example.com",
		LineRefs:      `["cftunnel:1"]`,
	})

	calls := 0
	m := NewManager(db, nil, 0)
	// 只给 frp:1 一条线路无法复现，这里直接构造：让 selector 选中 cftunnel:1
	// 通过 fakeProber 全同延迟时会选到 cftunnel:1（seedData 中仅该服务关联它）
	m.selector = selector.NewSelector(&fakeProber{}, 0)
	m.SetDNSUpdater(func(domain, ip string) error {
		calls++
		return nil
	})

	m.refresh(context.Background())
	// seedData 有 6 条线路，fakeProber 全同延迟，选中的是第一条（frp:1），
	// 但该服务只关联 cftunnel:1，所以不应触发
	if calls != 0 {
		t.Fatalf("域名入口线路不应触发 DNS 切换, got %d calls", calls)
	}
}

func TestApplyCaddySwitchRollback(t *testing.T) {
	db := newTestDB(t)
	seedData(db)
	db.Create(&model.TunService{
		Name:          "回滚测试服务",
		Enable:        true,
		TargetAddress: "127.0.0.1",
		TargetPort:    8080,
		LineRefs:      `["frp:1","nps:1"]`,
		CaddySiteID:   9,
	})

	prober := &fakeProber{latencies: map[string]time.Duration{
		"frp:1": 1 * time.Millisecond,
	}}
	m := NewManager(db, nil, 0)
	m.selector = selector.NewSelector(prober, 0)

	var applied []string
	m.SetCaddyUpdater(func(siteID uint, upstream string) error {
		// 模拟切换到 nps:1（5.6.7.8:8024）时 Caddy API 失败
		if upstream == "5.6.7.8:8024" {
			return fmt.Errorf("模拟 Caddy API 失败")
		}
		applied = append(applied, upstream)
		return nil
	})

	// 第一次刷新：frp:1 延迟最低，成功切换到 1.2.3.4:7000
	m.refresh(context.Background())
	if len(applied) != 1 || applied[0] != "1.2.3.4:7000" {
		t.Fatalf("第一次切换期望成功到 1.2.3.4:7000, got %v", applied)
	}

	// 第二次刷新：nps:1 延迟最低，切换 5.6.7.8:8024 失败，应回滚到上次成功的 1.2.3.4:7000
	prober.latencies = map[string]time.Duration{
		"nps:1": 1 * time.Millisecond,
		"frp:1": 100 * time.Millisecond,
	}
	m.refresh(context.Background())
	if len(applied) != 2 {
		t.Fatalf("切换失败后应触发回滚调用, applied=%v", applied)
	}
	if applied[1] != "1.2.3.4:7000" {
		t.Errorf("回滚应恢复到上次成功的 1.2.3.4:7000, got %q", applied[1])
	}
}

// TestApplyPortSwitch 端口层切换：仅未绑定 Caddy/DNS 的服务触发重绑回调。
func TestApplyPortSwitch(t *testing.T) {
	db := newTestDB(t)
	seedData(db)
	// 端口层服务：无 Caddy 站点、无域名，关联 frp:1
	portSvc := model.TunService{
		Name:          "SSH 服务",
		Enable:        true,
		TargetAddress: "192.168.1.10",
		TargetPort:    22,
		Protocol:      "tcp",
		LineRefs:      `["frp:1"]`,
	}
	db.Create(&portSvc)
	// 域名层服务：绑定了 Caddy 站点，不应触发端口层重绑
	db.Create(&model.TunService{
		Name:          "Web 服务",
		Enable:        true,
		TargetAddress: "192.168.1.10",
		TargetPort:    80,
		Protocol:      "tcp",
		LineRefs:      `["frp:1"]`,
		CaddySiteID:   9,
	})
	// DNS 层服务：配置了域名，不应触发端口层重绑
	db.Create(&model.TunService{
		Name:          "带域名服务",
		Enable:        true,
		TargetAddress: "192.168.1.10",
		TargetPort:    8080,
		Protocol:      "tcp",
		LineRefs:      `["frp:1"]`,
		Domain:        "svc.example.com",
	})

	m := NewManager(db, nil, 0)
	var calls []string
	m.SetPortRebinder(func(svcID uint, lineID string) error {
		calls = append(calls, fmt.Sprintf("%d:%s", svcID, lineID))
		return nil
	})

	m.applyPortSwitch("frp:1")
	if len(calls) != 1 || calls[0] != fmt.Sprintf("%d:frp:1", portSvc.ID) {
		t.Fatalf("端口层重绑应只触发端口层服务 %d, got %v", portSvc.ID, calls)
	}

	// 未关联线路不触发
	m.applyPortSwitch("easytier:1:0")
	if len(calls) != 1 {
		t.Fatalf("未关联线路不应触发重绑, got %d calls", len(calls))
	}

	// 重绑失败写入 last_error，成功清空
	m.SetPortRebinder(func(svcID uint, lineID string) error {
		return fmt.Errorf("模拟重绑失败")
	})
	m.applyPortSwitch("frp:1")
	var got model.TunService
	if err := db.First(&got, portSvc.ID).Error; err != nil {
		t.Fatal(err)
	}
	if got.LastError == "" {
		t.Fatal("重绑失败应写入 last_error")
	}
}

func TestRebindModeManualQueuesPending(t *testing.T) {
	db := newTestDB(t)
	seedData(db)
	portSvc := model.TunService{
		Name:          "SSH 服务",
		Enable:        true,
		TargetAddress: "192.168.1.10",
		TargetPort:    22,
		Protocol:      "tcp",
		LineRefs:      `["frp:1"]`,
	}
	db.Create(&portSvc)

	m := NewManager(db, nil, 0)
	var calls []string
	m.SetPortRebinder(func(svcID uint, lineID string) error {
		calls = append(calls, fmt.Sprintf("%d:%s", svcID, lineID))
		return nil
	})

	// manual 模式：不自动重绑，只记录待重绑清单
	m.SetRebindMode(RebindModeManual)
	m.applyPortSwitch("frp:1")
	if len(calls) != 0 {
		t.Fatalf("manual 模式不应自动重绑, got %v", calls)
	}
	pending := m.PendingRebinds()
	if len(pending) != 1 || pending[portSvc.ID] != "frp:1" {
		t.Fatalf("manual 模式应记录待重绑 %d->frp:1, got %v", portSvc.ID, pending)
	}

	// 手动触发：调用重绑回调并清空清单
	applied, err := m.ApplyPendingRebinds()
	if err != nil {
		t.Fatalf("ApplyPendingRebinds 失败: %v", err)
	}
	if applied != 1 || len(calls) != 1 || calls[0] != fmt.Sprintf("%d:frp:1", portSvc.ID) {
		t.Fatalf("手动重绑应触发 1 次, applied=%d calls=%v", applied, calls)
	}
	if len(m.PendingRebinds()) != 0 {
		t.Fatalf("手动重绑后待重绑清单应清空, got %v", m.PendingRebinds())
	}
}

func TestRebindModeOffSkips(t *testing.T) {
	db := newTestDB(t)
	seedData(db)
	portSvc := model.TunService{
		Name:          "SSH 服务",
		Enable:        true,
		TargetAddress: "192.168.1.10",
		TargetPort:    22,
		Protocol:      "tcp",
		LineRefs:      `["frp:1"]`,
	}
	db.Create(&portSvc)

	m := NewManager(db, nil, 0)
	var calls []string
	m.SetPortRebinder(func(svcID uint, lineID string) error {
		calls = append(calls, fmt.Sprintf("%d:%s", svcID, lineID))
		return nil
	})

	m.SetRebindMode(RebindModeOff)
	m.applyPortSwitch("frp:1")
	if len(calls) != 0 {
		t.Fatalf("off 模式不应重绑, got %v", calls)
	}
	if len(m.PendingRebinds()) != 0 {
		t.Fatalf("off 模式不应记录待重绑, got %v", m.PendingRebinds())
	}
}

func TestLineHost(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4:7000":                   "1.2.3.4",
		"my-tunnel.cfargotunnel.com:443": "my-tunnel.cfargotunnel.com",
		"plain-host":                     "plain-host",
		"":                               "",
	}
	for in, want := range cases {
		if got := lineHost(in); got != want {
			t.Errorf("lineHost(%q) = %q, want %q", in, got, want)
		}
	}
}
