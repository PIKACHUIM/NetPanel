package syslog

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/netpanel/netpanel/model"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.SystemLog{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	return NewManager(db, logrus.New())
}

func countLogs(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	db.Model(&model.SystemLog{}).Count(&n)
	return n
}

// TestWriteBatching 验证日志经后台 writer 攒批落库（核心：非阻塞入队）
func TestWriteBatching(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	const total = 250
	for i := 0; i < total; i++ {
		m.Write("info", "test", fmt.Sprintf("log-%d", i))
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if countLogs(t, m.db) == total {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("3 秒内未全部落库: got %d, want %d", countLogs(t, m.db), total)
}

// TestStopDrainsQueue 停止时应排空队列再退出
func TestStopDrainsQueue(t *testing.T) {
	m := newTestManager(t)

	const total = 50
	for i := 0; i < total; i++ {
		m.Write("warn", "test", fmt.Sprintf("drain-%d", i))
	}
	m.Stop()

	// Stop 返回后（排空完成）再计数
	if got := countLogs(t, m.db); got != total {
		t.Fatalf("停止后应全部落库: got %d, want %d", got, total)
	}
}

// TestWriteNeverBlocks 队列远超容量时写入不阻塞、不 panic（丢弃计数）
func TestWriteNeverBlocks(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < logQueueSize+1000; i++ {
			m.Write("info", "flood", "flood-entry")
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("入队在高负载下不应阻塞")
	}
}
