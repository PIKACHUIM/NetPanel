package retention

import (
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/test.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&model.MonitorMetric{},
		&model.MonitorProbeResult{},
		&model.WafLog{},
		&model.SystemLog{},
		&model.DDNSHistory{},
		&model.AiCronLog{},
		&model.SystemConfig{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCleanupTableDeletesOldRows(t *testing.T) {
	db := testDB(t)
	c := New(db, logrus.New())

	old := time.Now().AddDate(0, 0, -60)
	recent := time.Now().AddDate(0, 0, -1)
	db.Create(&model.SystemLog{LogTime: old, Message: "old"})
	db.Create(&model.SystemLog{LogTime: old, Message: "old2"})
	db.Create(&model.SystemLog{LogTime: recent, Message: "new"})

	n := c.cleanupTable(&model.SystemLog{}, "log_time", time.Now().AddDate(0, 0, -30))
	if n != 2 {
		t.Fatalf("期望删除 2 条旧日志，实际 %d", n)
	}
	var count int64
	db.Model(&model.SystemLog{}).Count(&count)
	if count != 1 {
		t.Fatalf("期望剩余 1 条，实际 %d", count)
	}
}

func TestRetentionDaysDefaults(t *testing.T) {
	db := testDB(t)
	c := New(db, logrus.New())

	// 无配置 → 默认 30
	if d := c.retentionDays(); d != defaultRetentionDays {
		t.Fatalf("默认应为 %d，实际 %d", defaultRetentionDays, d)
	}

	// 非法值 → 默认
	db.Create(&model.SystemConfig{Key: cfgKeyRetentionDays, Value: "abc"})
	if d := c.retentionDays(); d != defaultRetentionDays {
		t.Fatalf("非法值应回落 %d，实际 %d", defaultRetentionDays, d)
	}

	// 正常值生效；超上限钳制到 365
	db.Where("key = ?", cfgKeyRetentionDays).Delete(&model.SystemConfig{})
	db.Create(&model.SystemConfig{Key: cfgKeyRetentionDays, Value: "90"})
	if d := c.retentionDays(); d != 90 {
		t.Fatalf("应读配置 90，实际 %d", d)
	}
	db.Where("key = ?", cfgKeyRetentionDays).Delete(&model.SystemConfig{})
	db.Create(&model.SystemConfig{Key: cfgKeyRetentionDays, Value: "99999"})
	if d := c.retentionDays(); d != 365 {
		t.Fatalf("超上限应钳制 365，实际 %d", d)
	}
}

func TestCleanupAllUsesPerTableRetention(t *testing.T) {
	db := testDB(t)
	c := New(db, logrus.New())

	old := time.Now().AddDate(0, 0, -20) // 早于 SystemLog 的 7 天保留，晚于全局 30 天
	db.Create(&model.SystemLog{LogTime: old})
	db.Create(&model.DDNSHistory{}) // 新记录，CreatedAt 为现在，不会被删

	c.cleanupAll()

	var sysCount int64
	db.Model(&model.SystemLog{}).Count(&sysCount)
	if sysCount != 0 {
		t.Fatalf("SystemLog 20 天前记录应被清理（7 天保留）")
	}
	var ddnsCount int64
	db.Model(&model.DDNSHistory{}).Count(&ddnsCount)
	if ddnsCount != 1 {
		t.Fatalf("DDNSHistory 新记录不应被清理")
	}
}
