package crud_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/pkg/crud"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// testUser 复刻 model.BaseModel 的形状，验证泛型对真实模型的适配性。
type testUser struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	Enable    bool      `gorm:"default:false" json:"enable"`
}

func (testUser) TableName() string { return "test_users" }

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	// 共享 cache 会让并行用例互相污染，这里用唯一 DSN 隔离
	if err := db.Migrator().DropTable(&testUser{}); err != nil {
		t.Fatalf("drop table: %v", err)
	}
	if err := db.AutoMigrate(&testUser{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return db
}

func TestCreateGetUpdateDelete(t *testing.T) {
	repo := crud.New[testUser](newDB(t))
	ctx := context.Background()

	u := &testUser{Name: "alice", Enable: true}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID == 0 {
		t.Fatal("Create 应回填自增主键")
	}
	if u.CreatedAt.IsZero() {
		t.Fatal("Create 应回填 CreatedAt")
	}

	got, err := repo.Get(ctx, u.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "alice" || !got.Enable {
		t.Fatalf("Get 返回内容不符: %+v", got)
	}

	got.Name = "alice2"
	if err := repo.Update(ctx, u.ID, &got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, err := repo.Get(ctx, u.ID)
	if err != nil || after.Name != "alice2" {
		t.Fatalf("Update 未生效: %+v err=%v", after, err)
	}

	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, u.ID); !errors.Is(err, crud.ErrNotFound) {
		t.Fatalf("删除后 Get 应返回 ErrNotFound，实际 %v", err)
	}
}

func TestListReturnsEmptySliceNotNil(t *testing.T) {
	repo := crud.New[testUser](newDB(t))
	items, err := repo.List(context.Background(), crud.ListOptions{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if items == nil {
		t.Fatal("List 无结果时应返回空切片而非 nil（否则 JSON 序列化为 null）")
	}
	if len(items) != 0 {
		t.Fatalf("期望 0 条，实际 %d", len(items))
	}
}

func TestListOptionsOrderLimitOffset(t *testing.T) {
	repo := crud.New[testUser](newDB(t))
	ctx := context.Background()

	for _, n := range []string{"a", "b", "c", "d", "e"} {
		u := &testUser{Name: n}
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	all, err := repo.List(ctx, crud.ListOptions{OrderBy: "id desc"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("期望 5 条，实际 %d", len(all))
	}
	if all[0].Name != "e" {
		t.Fatalf("id desc 排序未生效，首条 = %s", all[0].Name)
	}

	page, err := repo.List(ctx, crud.ListOptions{OrderBy: "id asc", Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("List 分页: %v", err)
	}
	if len(page) != 2 || page[0].Name != "b" || page[1].Name != "c" {
		t.Fatalf("分页结果不符: %+v", page)
	}
}

func TestListWhereFilter(t *testing.T) {
	repo := crud.New[testUser](newDB(t))
	ctx := context.Background()

	if err := repo.Create(ctx, &testUser{Name: "on", Enable: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(ctx, &testUser{Name: "off", Enable: false}); err != nil {
		t.Fatal(err)
	}

	items, err := repo.List(ctx, crud.ListOptions{Where: map[string]any{"enable = ?": true}})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Name != "on" {
		t.Fatalf("Where 过滤失效: %+v", items)
	}

	n, err := repo.Count(ctx, crud.CountOptions{Where: map[string]any{"enable = ?": true}})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n != 1 {
		t.Fatalf("Count 期望 1，实际 %d", n)
	}
}

// TestUpsertAndDeleteSemantics 锁定迁移必须保持的 API 契约：
// Update 走 gorm Save 的 upsert 语义（不存在则创建），Delete 对缺失记录不报错。
// 改造前 handler 就是 h.db.Save / h.db.Delete，若改为「不存在即 404」会改变 API 行为。
func TestUpsertAndDeleteSemantics(t *testing.T) {
	repo := crud.New[testUser](newDB(t))
	ctx := context.Background()

	// Update 不存在的记录 → upsert 创建
	if err := repo.Update(ctx, uint(4242), &testUser{ID: 4242, Name: "ghost"}); err != nil {
		t.Fatalf("Update 不存在的记录不应报错: %v", err)
	}
	got, err := repo.Get(ctx, uint(4242))
	if err != nil {
		t.Fatalf("upsert 后应能查到记录: %v", err)
	}
	if got.Name != "ghost" {
		t.Fatalf("upsert 内容不符: %+v", got)
	}

	// Delete 不存在的记录 → 返回 nil
	if err := repo.Delete(ctx, uint(9999)); err != nil {
		t.Fatalf("Delete 不存在的记录应返回 nil，实际 %v", err)
	}
}

func TestNilItemRejected(t *testing.T) {
	repo := crud.New[testUser](newDB(t))
	ctx := context.Background()

	if err := repo.Create(ctx, nil); !errors.Is(err, crud.ErrNilItem) {
		t.Fatalf("Create(nil) 应返回 ErrNilItem，实际 %v", err)
	}
	if err := repo.Update(ctx, uint(1), nil); !errors.Is(err, crud.ErrNilItem) {
		t.Fatalf("Update(nil) 应返回 ErrNilItem，实际 %v", err)
	}
}

func TestContextCancelledPropagates(t *testing.T) {
	repo := crud.New[testUser](newDB(t))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := repo.List(ctx, crud.ListOptions{}); err == nil {
		t.Fatal("已取消的 context 应导致查询报错")
	}
}

func TestMustRepoPanicsOnNilDB(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("MustRepo(nil) 应 panic")
		}
	}()
	crud.MustRepo[testUser](nil)
}

// 编译期断言：Repo 与 MemoryRepo 均满足 Store 契约。
var (
	_ crud.Store[testUser] = (*crud.Repo[testUser])(nil)
	_ crud.Store[testUser] = (*crud.MemoryRepo[testUser])(nil)
)
