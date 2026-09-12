package callback

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/netpanel/netpanel/model"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	// 内存 SQLite 多连接时各连接是独立空库，并发 goroutine 会查不到数据，
	// 必须限制为单连接串行访问
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.CallbackAccount{}, &model.CallbackTask{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

func testLogger() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(io.Discard)
	return l
}

// TestTriggerBySTUN 验证 STUN 地址变化 -> 执行绑定回调任务（webhook）的完整链路
func TestTriggerBySTUN(t *testing.T) {
	var hits int32
	gotBody := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		buf, _ := io.ReadAll(r.Body)
		select {
		case gotBody <- string(buf):
		default:
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	db := newTestDB(t)
	m := NewManager(db, testLogger())
	m.Start()
	defer m.Stop()

	acc := model.CallbackAccount{Name: "wh", Type: "webhook", Config: fmt.Sprintf(`{"url":%q}`, srv.URL)}
	if err := db.Create(&acc).Error; err != nil {
		t.Fatal(err)
	}
	// 前端表单保存的 trigger_type 是完整事件名
	task := model.CallbackTask{Name: "stun 回调", Enable: true, AccountID: acc.ID, TriggerType: "stun_ip_change"}
	if err := db.Create(&task).Error; err != nil {
		t.Fatal(err)
	}

	if err := m.TriggerBySTUN(task.ID, "1.2.3.4", 5678); err != nil {
		t.Fatalf("TriggerBySTUN 失败: %v", err)
	}

	select {
	case body := <-gotBody:
		if !strings.Contains(body, "1.2.3.4") || !strings.Contains(body, "5678") {
			t.Errorf("webhook payload 缺少新地址: %s", body)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("webhook 未被调用")
	}
}

func TestTriggerBySTUNValidation(t *testing.T) {
	db := newTestDB(t)
	m := NewManager(db, testLogger())

	// 任务不存在
	if err := m.TriggerBySTUN(999, "1.2.3.4", 1); err == nil {
		t.Error("任务不存在时应报错")
	}

	acc := model.CallbackAccount{Name: "wh", Type: "webhook", Config: `{"url":"http://127.0.0.1:1/x"}`}
	db.Create(&acc)
	task := model.CallbackTask{Name: "停用任务", AccountID: acc.ID, TriggerType: "stun"}
	db.Create(&task)
	db.Model(&task).Update("enable", false)

	if err := m.TriggerBySTUN(task.ID, "1.2.3.4", 1); err == nil {
		t.Error("任务未启用时应报错")
	}
}

// TestHandleEventMatchesBothTriggerTypes 事件分发应同时兼容映射名（stun）
// 与完整事件名（stun_ip_change）两种存储值（历史数据 / 前端表单）
func TestHandleEventMatchesBothTriggerTypes(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer srv.Close()

	db := newTestDB(t)
	m := NewManager(db, testLogger())
	m.Start()
	defer m.Stop()

	acc := model.CallbackAccount{Name: "wh", Type: "webhook", Config: fmt.Sprintf(`{"url":%q}`, srv.URL)}
	db.Create(&acc)

	taskMapped := model.CallbackTask{Name: "映射名任务", Enable: true, AccountID: acc.ID, TriggerType: "stun"}
	db.Create(&taskMapped)
	taskFull := model.CallbackTask{Name: "完整名任务", Enable: true, AccountID: acc.ID, TriggerType: "stun_ip_change"}
	db.Create(&taskFull)

	m.handleEvent(TriggerEvent{Type: "stun_ip_change", NewIP: "5.6.7.8", NewPort: 1234})

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&hits) >= 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("两个任务都应被触发, 实际触发 %d 次", atomic.LoadInt32(&hits))
}
