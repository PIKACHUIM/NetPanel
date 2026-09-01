package monitor

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

func newAlertTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "alert_test.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	// 测试结束时关闭底层连接，否则 Windows 下 SQLite 文件句柄未释放，
	// 会导致 t.TempDir() 的 RemoveAll cleanup 失败。
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.AutoMigrate(
		&model.MonitorProbe{},
		&model.MonitorProbeResult{},
		&model.MonitorAlert{},
		&model.MonitorAlertRecord{},
		&model.MonitorServer{},
	); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	return db
}

// seedProbe 写入探测配置与按时间升序的探测结果（最新的最后写入）
func seedProbe(t *testing.T, db *gorm.DB, successes ...bool) model.MonitorProbe {
	t.Helper()
	probe := model.MonitorProbe{
		Name:             "测试探测",
		Enable:           true,
		ProbeType:        "tcp",
		TargetAddr:       "192.0.2.10",
		TargetPort:       8080,
		Interval:         60,
		ServerIDs:        `["1"]`,
		FailThreshold:    3,
		RecoverThreshold: 2,
	}
	if err := db.Create(&probe).Error; err != nil {
		t.Fatalf("创建探测配置失败: %v", err)
	}
	base := time.Now().Add(-time.Duration(len(successes)+1) * time.Minute)
	for i, s := range successes {
		r := model.MonitorProbeResult{
			ProbeID:   probe.ID,
			ServerID:  1,
			Timestamp: base.Add(time.Duration(i) * time.Minute),
			Success:   s,
		}
		if err := db.Create(&r).Error; err != nil {
			t.Fatalf("写入探测结果失败: %v", err)
		}
	}
	return probe
}

func TestResolveProbeTargets(t *testing.T) {
	a := &AlertEngine{}
	got := a.resolveProbeTargets(`["probe:1","probe:22","server:3","group:default"]`)
	if len(got) != 2 || got[0] != 1 || got[1] != 22 {
		t.Fatalf("期望解析出 [1 22]，实际 %v", got)
	}
	if got := a.resolveProbeTargets(`不是 JSON`); got != nil {
		t.Fatalf("非法 JSON 应返回 nil，实际 %v", got)
	}
}

func TestProbeState_ThreeStates(t *testing.T) {
	db := newAlertTestDB(t)
	a := NewAlertEngine(db, nil)

	// 结果不足 FailThreshold → 依据不足
	probe := seedProbe(t, db, false, false)
	if state, _ := a.probeState(probe, 1); state != "" {
		t.Fatalf("结果不足时应为空状态，实际 %q", state)
	}

	// 连续失败达 FailThreshold → trigger
	db.Create(&model.MonitorProbeResult{ProbeID: probe.ID, ServerID: 1, Timestamp: time.Now(), Success: false})
	if state, content := a.probeState(probe, 1); state != "trigger" || content == "" {
		t.Fatalf("连续失败 3 次应触发，实际 %q %q", state, content)
	}

	// 失败后仅 1 次成功 → 依据不足（未达恢复阈值）
	db.Create(&model.MonitorProbeResult{ProbeID: probe.ID, ServerID: 1, Timestamp: time.Now(), Success: true})
	if state, _ := a.probeState(probe, 1); state != "" {
		t.Fatalf("1 次成功未达恢复阈值应为空状态，实际 %q", state)
	}

	// 连续成功达 RecoverThreshold → recover
	db.Create(&model.MonitorProbeResult{ProbeID: probe.ID, ServerID: 1, Timestamp: time.Now(), Success: true})
	if state, _ := a.probeState(probe, 1); state != "recover" {
		t.Fatalf("连续成功 2 次应恢复，实际 %q", state)
	}
}

func TestHandleAlertState_ProbeLifecycle(t *testing.T) {
	db := newAlertTestDB(t)
	a := NewAlertEngine(db, nil)

	probe := seedProbe(t, db, false, false, false)
	alert := model.MonitorAlert{
		Name:            "探测失败告警",
		Enable:          true,
		AlertType:       "probe",
		TargetServers:   `["probe:1"]`,
		NotifyChannels:  `[]`,
		Severity:        "warning",
		SilenceDuration: 3600,
		RateLimit:       300,
	}
	if err := db.Create(&alert).Error; err != nil {
		t.Fatalf("创建告警规则失败: %v", err)
	}

	obj := alertTarget{kind: "probe", targetID: 1, probeID: probe.ID}

	// 首次触发：创建告警记录
	a.handleAlertState(alert, obj, "trigger", "探测失败")
	var records []model.MonitorAlertRecord
	db.Where("alert_id = ?", alert.ID).Find(&records)
	if len(records) != 1 || records[0].RecoverTime != nil {
		t.Fatalf("首次触发应有 1 条未恢复记录，实际 %d 条", len(records))
	}

	// 持续失败（静默期内）：不重复建记录
	a.handleAlertState(alert, obj, "trigger", "探测失败")
	db.Where("alert_id = ?", alert.ID).Find(&records)
	if len(records) != 1 {
		t.Fatalf("静默期内重复触发不应新建记录，实际 %d 条", len(records))
	}

	// 恢复：补齐恢复时间，状态复位
	a.handleAlertState(alert, obj, "recover", "")
	db.Where("alert_id = ?", alert.ID).Find(&records)
	if len(records) != 1 || records[0].RecoverTime == nil {
		t.Fatalf("恢复后记录应带恢复时间")
	}

	// 恢复后再次失败：新建一条记录
	a.handleAlertState(alert, obj, "trigger", "探测失败")
	db.Where("alert_id = ?", alert.ID).Find(&records)
	if len(records) != 2 {
		t.Fatalf("恢复后再次触发应新建记录，实际 %d 条", len(records))
	}
}

func TestTriggerProbeAlert_RespectsRuleTargets(t *testing.T) {
	db := newAlertTestDB(t)
	a := NewAlertEngine(db, nil)

	probe := seedProbe(t, db, false, false, false)
	// 规则目标不含该探测 → 不应产生告警记录
	other := model.MonitorAlert{
		Name:           "其他探测的告警",
		Enable:         true,
		AlertType:      "probe",
		TargetServers:  `["probe:999"]`,
		NotifyChannels: `[]`,
		Severity:       "warning",
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("创建告警规则失败: %v", err)
	}

	a.TriggerProbeAlert(probe, 1)
	var count int64
	db.Model(&model.MonitorAlertRecord{}).Count(&count)
	if count != 0 {
		t.Fatalf("目标不匹配时不应产生告警记录，实际 %d 条", count)
	}

	// 目标匹配 → 产生记录
	matched := model.MonitorAlert{
		Name:           "匹配的探测告警",
		Enable:         true,
		AlertType:      "probe",
		TargetServers:  `["probe:` + strconv.FormatUint(uint64(probe.ID), 10) + `"]`,
		NotifyChannels: `[]`,
		Severity:       "warning",
	}
	if err := db.Create(&matched).Error; err != nil {
		t.Fatalf("创建告警规则失败: %v", err)
	}
	a.TriggerProbeAlert(probe, 1)
	db.Model(&model.MonitorAlertRecord{}).Count(&count)
	if count != 1 {
		t.Fatalf("目标匹配时应产生 1 条告警记录，实际 %d 条", count)
	}
}
