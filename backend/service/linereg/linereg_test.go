package linereg

import (
	"context"
	"errors"
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
		&model.SystemLog{},
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

	// CF 隧道：仅 named 模式且能解析出隧道 UUID 的注册为线路
	db.Create(&model.CftunnelConfig{Name: "cf named", Enable: true, Mode: "named", TunnelName: "d40b1a2c-3f4e-4a5b-8c6d-7e8f9a0b1c2d", LocalURL: "http://127.0.0.1:8080"})
	db.Create(&model.CftunnelConfig{Name: "cf quick", Enable: true, Mode: "quick", LocalURL: "http://127.0.0.1:8081"})
	db.Create(&model.CftunnelConfig{Name: "cf token", Enable: true, Mode: "token", Token: "eyJhIjoi"})
	db.Create(&model.CftunnelConfig{Name: "cf 停用", Enable: false, Mode: "named", TunnelName: "off-tunnel"})
	db.Create(&model.CftunnelConfig{Name: "cf 无名", Enable: true, Mode: "named", TunnelName: ""})
	// 隧道名称无法本地解析为 UUID：不注册，等 cftunnel 启动时从凭据文件解析写回
	db.Create(&model.CftunnelConfig{Name: "cf 名称未解析", Enable: true, Mode: "named", TunnelName: "my-tunnel"})
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

	// cftunnel：仅 named 模式且可解析出隧道 UUID 的注册为线路
	if l, ok := byID["cftunnel:1"]; !ok {
		t.Error("缺少 cftunnel:1")
	} else if l.Address != "d40b1a2c-3f4e-4a5b-8c6d-7e8f9a0b1c2d.cfargotunnel.com:443" || l.Tool != "cloudflare" {
		t.Errorf("cftunnel:1 地址/工具错误: %+v", l)
	} else if l.Layer != "domain" {
		t.Errorf("cftunnel:1 应为域名层(domain), got %q", l.Layer)
	}
	for _, id := range []string{"cftunnel:2", "cftunnel:3", "cftunnel:4", "cftunnel:5", "cftunnel:6"} {
		if _, ok := byID[id]; ok {
			t.Errorf("%s 不应被收集（quick/token/停用/无名/名称未解析）", id)
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

// errInjectProber 可注入故障的探测器（健康事件集成测试用）。
type errInjectProber struct {
	err error
}

func (p *errInjectProber) Probe(_ context.Context, line selector.Line) selector.ProbeResult {
	if p.err != nil {
		return selector.ProbeResult{LineID: line.ID, Err: &selector.ProbeError{Reason: p.err.Error()}}
	}
	return selector.ProbeResult{LineID: line.ID, TCPLatency: 10 * time.Millisecond}
}

// TestHealthEventSinkAndSystemLog 验证健康转换事件闭环：
// selector 探测失败 → linereg observer → SystemLog 落库 + sink 外送；恢复同理。
func TestHealthEventSinkAndSystemLog(t *testing.T) {
	db := newTestDB(t)
	m := NewManager(db, nil, 0)
	m.SetFailureThreshold(1)

	var sink []selector.HealthEvent
	m.SetHealthEventSink(func(ev selector.HealthEvent) { sink = append(sink, ev) })

	m.SetProber(&errInjectProber{err: errors.New("down")})
	m.selector.SetLines([]selector.Line{{ID: "frp:1", Name: "测试线路", Tool: "frp", Address: "127.0.0.1:1"}})

	// 失败一轮 → 进入不可达：1 个事件 + 1 条 warn SystemLog。
	m.selector.ProbeAll(context.Background())
	if len(sink) != 1 || sink[0].To != selector.HealthUnreachable {
		t.Fatalf("进入不可达事件不符: %+v", sink)
	}
	var cnt int64
	db.Model(&model.SystemLog{}).Where("service = ?", "linereg").Count(&cnt)
	if cnt != 1 {
		t.Fatalf("SystemLog 应有 1 条 linereg 记录, got %d", cnt)
	}

	// 恢复一轮 → healthy：事件 +1，SystemLog +1（info）。
	m.SetProber(&errInjectProber{})
	m.selector.ProbeAll(context.Background())
	if len(sink) != 2 || sink[1].From != selector.HealthUnreachable || sink[1].To != selector.HealthHealthy {
		t.Fatalf("恢复事件不符: %+v", sink)
	}
	db.Model(&model.SystemLog{}).Where("service = ?", "linereg").Count(&cnt)
	if cnt != 2 {
		t.Fatalf("SystemLog 应有 2 条 linereg 记录, got %d", cnt)
	}
}
