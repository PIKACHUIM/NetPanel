package linereg

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/netpanel/netpanel/model"
)

// newProbeConfigTestDB 构造仅含 SystemConfig 表的独立内存数据库
func newProbeConfigTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.SystemConfig{}); err != nil {
		t.Fatalf("迁移 SystemConfig 失败: %v", err)
	}
	return db
}

func newTestManager(t *testing.T, db *gorm.DB) *Manager {
	t.Helper()
	log := logrus.New()
	log.SetLevel(logrus.PanicLevel)
	return NewManager(db, log, 0)
}

// TestLoadProbeConfig_AllKeysApplied 验证四项探测参数均能被正确加载。
//
// 回归点：原实现复用同一个 model.SystemConfig 变量做四次 First 查询，
// 首次查询后变量主键被填充，GORM 会把主键并入后续 WHERE 条件，
// 导致后三项配置必然查不到而静默退回默认值。
func TestLoadProbeConfig_AllKeysApplied(t *testing.T) {
	db := newProbeConfigTestDB(t)
	configs := []model.SystemConfig{
		{Key: cfgKeyIntervalSec, Value: "30"},
		{Key: cfgKeyFailureThreshold, Value: "7"},
		{Key: cfgKeyToleranceMs, Value: "123"},
		{Key: cfgKeyMaxConcurrent, Value: "16"},
	}
	for i := range configs {
		if err := db.Create(&configs[i]).Error; err != nil {
			t.Fatalf("写入配置失败: %v", err)
		}
	}

	m := newTestManager(t, db)
	if err := m.LoadProbeConfig(); err != nil {
		t.Fatalf("LoadProbeConfig 返回错误: %v", err)
	}

	if got := m.currentInterval(); got != 30*time.Second {
		t.Errorf("interval = %v, 期望 30s", got)
	}
	// 后三项此前会被静默丢弃，退回默认值
	if got := m.selector.FailureThreshold(); got != 7 {
		t.Errorf("failureThreshold = %d, 期望 7（默认值为 %d，若等于默认值说明配置未生效）",
			got, defaultFailureThreshold)
	}
	if got := m.selector.Tolerance(); got != 123*time.Millisecond {
		t.Errorf("tolerance = %v, 期望 123ms（默认值为 %dms）", got, defaultToleranceMs)
	}
	if got := m.selector.MaxConcurrent(); got != 16 {
		t.Errorf("maxConcurrent = %d, 期望 16（默认值为 %d）", got, defaultMaxConcurrent)
	}
}

// TestLoadProbeConfig_Defaults 配置缺失时应使用默认值
func TestLoadProbeConfig_Defaults(t *testing.T) {
	db := newProbeConfigTestDB(t)
	m := newTestManager(t, db)

	if err := m.LoadProbeConfig(); err != nil {
		t.Fatalf("LoadProbeConfig 返回错误: %v", err)
	}
	if got := m.currentInterval(); got != time.Duration(defaultIntervalSec)*time.Second {
		t.Errorf("interval = %v, 期望默认 %ds", got, defaultIntervalSec)
	}
	if got := m.selector.FailureThreshold(); got != defaultFailureThreshold {
		t.Errorf("failureThreshold = %d, 期望默认 %d", got, defaultFailureThreshold)
	}
}

// TestLoadProbeConfig_ClampsInvalidValues 非法值应被钳制到安全范围。
// 回归点：interval 为 0 或负数会让 time.NewTimer/NewTicker 直接 panic。
func TestLoadProbeConfig_ClampsInvalidValues(t *testing.T) {
	cases := []struct {
		name     string
		interval string
	}{
		{"零值", "0"},
		{"负数", "-10"},
		{"过小", "1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := newProbeConfigTestDB(t)
			db.Create(&model.SystemConfig{Key: cfgKeyIntervalSec, Value: tc.interval})
			db.Create(&model.SystemConfig{Key: cfgKeyMaxConcurrent, Value: "0"})

			m := newTestManager(t, db)
			if err := m.LoadProbeConfig(); err != nil {
				t.Fatalf("LoadProbeConfig 返回错误: %v", err)
			}

			if got := m.currentInterval(); got < minIntervalSec*time.Second {
				t.Errorf("interval = %v, 期望不小于 %ds（否则定时器会 panic）", got, minIntervalSec)
			}
			if got := m.selector.MaxConcurrent(); got <= 0 {
				t.Errorf("maxConcurrent = %d, 期望为正数", got)
			}
		})
	}
}

// TestSetInterval_HotReload 验证间隔可在运行期热更新。
// 回归点：原实现在 run() 入口固定创建 Ticker，之后不再读取 interval，
// 修改间隔必须重启进程才生效，与接口注释描述不符。
func TestSetInterval_HotReload(t *testing.T) {
	db := newProbeConfigTestDB(t)
	m := newTestManager(t, db)

	m.SetInterval(15 * time.Second)
	if got := m.currentInterval(); got != 15*time.Second {
		t.Fatalf("interval = %v, 期望 15s", got)
	}

	// 非法值应被忽略，保留原值
	m.SetInterval(0)
	m.SetInterval(-1 * time.Second)
	if got := m.currentInterval(); got != 15*time.Second {
		t.Errorf("interval = %v, 期望保持 15s（非法值应被忽略）", got)
	}

	// reload 通道容量为 1，多次调用不应阻塞
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			m.SetInterval(time.Duration(20+i) * time.Second)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("SetInterval 发生阻塞：reload 通道未做非阻塞写入")
	}
}

// TestSetInterval_ConcurrentAccess 并发读写 interval 不应触发 data race。
// 需配合 -race 运行才有意义。
func TestSetInterval_ConcurrentAccess(t *testing.T) {
	db := newProbeConfigTestDB(t)
	m := newTestManager(t, db)

	stop := make(chan struct{})
	done := make(chan struct{}, 2)

	// 写方：模拟 HTTP handler goroutine
	go func() {
		defer func() { done <- struct{}{} }()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				m.SetInterval(time.Duration(10+i%50) * time.Second)
			}
		}
	}()

	// 读方：模拟后台探测循环
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			select {
			case <-stop:
				return
			default:
				_ = m.currentInterval()
			}
		}
	}()

	time.Sleep(200 * time.Millisecond)
	close(stop)
	<-done
	<-done
}
