package tunservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/linereg"
	"github.com/netpanel/netpanel/service/selector"
)

// fakeProber 注入式探测器：按线路 id 返回固定延迟/错误。
type fakeProber struct {
	latencies map[string]time.Duration
	errors    map[string]error
}

func (f *fakeProber) Probe(_ context.Context, line selector.Line) selector.ProbeResult {
	if err := f.errors[line.ID]; err != nil {
		return selector.ProbeResult{LineID: line.ID, Err: err}
	}
	return selector.ProbeResult{LineID: line.ID, TCPLatency: f.latencies[line.ID]}
}

// newTestDB 创建内存 SQLite 并迁移 TunService 模型。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.TunService{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

// newTestManager 构造 Manager：linereg 注入 fake prober 与固定线路。
func newTestManager(t *testing.T, lines []selector.Line, lat map[string]time.Duration, errs map[string]error) *Manager {
	t.Helper()
	db := newTestDB(t)
	lineregMgr := linereg.NewManager(db, nil, 0)
	lineregMgr.Selector().SetToolFilter(nil)
	lineregMgr.Selector().SetLines(lines)
	lineregMgr.Selector().SetProber(&fakeProber{latencies: lat, errors: errs})
	return NewManager(db, nil, lineregMgr, nil, nil, nil, nil, nil)
}

func TestSpeedtestSortsByLatency(t *testing.T) {
	lines := []selector.Line{
		{ID: "frp:1", Name: "阿里 frps", Tool: "frp", Address: "1.2.3.4:7000"},
		{ID: "wg:1", Name: "WG 对端", Tool: "wireguard", Address: "1.2.3.5:51820"},
		{ID: "nps:1", Name: "nps 节点", Tool: "nps", Address: "5.6.7.8:8024"},
	}
	m := newTestManager(t, lines, map[string]time.Duration{
		"frp:1": 300 * time.Millisecond,
		"wg:1":  100 * time.Millisecond,
		"nps:1": 200 * time.Millisecond,
	}, nil)
	db := m.db
	db.Create(&model.TunService{
		Name:          "SSH 服务",
		Enable:        true,
		TargetAddress: "192.168.1.10",
		TargetPort:    22,
		Protocol:      "tcp",
		LineRefs:      `["frp:1","wg:1","nps:1"]`,
	})

	out, err := m.Speedtest(1)
	if err != nil {
		t.Fatalf("Speedtest 失败: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("期望 3 条结果, got %d: %+v", len(out), out)
	}
	// 延迟升序：wg(100) < nps(200) < frp(300)
	want := []string{"wg:1", "nps:1", "frp:1"}
	for i, id := range want {
		if out[i].ID != id {
			t.Errorf("排序错误: 位置 %d 期望 %s, got %s", i, id, out[i].ID)
		}
		if out[i].Latency <= 0 {
			t.Errorf("线路 %s 应有正延迟, got %d", id, out[i].Latency)
		}
	}
}

func TestSpeedtestFailuresLast(t *testing.T) {
	lines := []selector.Line{
		{ID: "frp:1", Name: "阿里 frps", Tool: "frp", Address: "1.2.3.4:7000"},
		{ID: "wg:1", Name: "WG 对端", Tool: "wireguard", Address: "1.2.3.5:51820"},
	}
	m := newTestManager(t, lines, map[string]time.Duration{
		"frp:1": 50 * time.Millisecond,
	}, map[string]error{
		"wg:1": errors.New("connect refused"),
	})
	db := m.db
	db.Create(&model.TunService{
		Name:          "SSH 服务",
		Enable:        true,
		TargetAddress: "192.168.1.10",
		TargetPort:    22,
		Protocol:      "tcp",
		LineRefs:      `["frp:1","wg:1"]`,
	})

	out, err := m.Speedtest(1)
	if err != nil {
		t.Fatalf("Speedtest 失败: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("期望 2 条结果, got %d", len(out))
	}
	// 成功线路在前，失败线路排最后
	if out[0].ID != "frp:1" || out[0].Error != "" {
		t.Errorf("期望成功线路 frp:1 在前, got %+v", out[0])
	}
	if out[1].ID != "wg:1" || out[1].Error == "" {
		t.Errorf("期望失败线路 wg:1 排最后并带错误, got %+v", out[1])
	}
}

func TestSpeedtestUnknownService(t *testing.T) {
	m := newTestManager(t, nil, nil, nil)
	if _, err := m.Speedtest(999); err == nil {
		t.Fatal("不存在的服务应返回错误")
	}
}
