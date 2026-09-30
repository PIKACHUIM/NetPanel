package crud_test

import (
	"context"
	"errors"
	"testing"

	"github.com/netpanel/netpanel/pkg/crud"
)

// newMemRepo 返回预置 3 条记录的内存仓储。
func newMemRepo() *crud.MemoryRepo[testUser] {
	repo := crud.NewMemory[testUser]("ID")
	repo.Seed(
		testUser{ID: 1, Name: "a"},
		testUser{ID: 2, Name: "b"},
		testUser{ID: 3, Name: "c"},
	)
	return repo
}

func TestMemoryCreateAssignsID(t *testing.T) {
	repo := crud.NewMemory[testUser]("ID")
	ctx := context.Background()

	u := &testUser{Name: "new"}
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u.ID != 1 {
		t.Fatalf("首个自增主键应为 1，实际 %d", u.ID)
	}

	u2 := &testUser{Name: "new2"}
	if err := repo.Create(ctx, u2); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if u2.ID != 2 {
		t.Fatalf("第二个自增主键应为 2，实际 %d", u2.ID)
	}
}

func TestMemoryListPagination(t *testing.T) {
	repo := newMemRepo()
	items, err := repo.List(context.Background(), crud.ListOptions{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 || items[0].ID != 2 || items[1].ID != 3 {
		t.Fatalf("分页结果不符: %+v", items)
	}

	// offset 越界应返回空切片而非 nil
	none, err := repo.List(context.Background(), crud.ListOptions{Offset: 99})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if none == nil || len(none) != 0 {
		t.Fatalf("offset 越界应返回空切片，实际 %+v", none)
	}
}

func TestMemoryRecordsListArgs(t *testing.T) {
	repo := newMemRepo()
	if _, err := repo.List(context.Background(), crud.ListOptions{
		OrderBy: "id desc", Limit: 10, Offset: 5,
	}); err != nil {
		t.Fatal(err)
	}
	orderBy, limit, offset := repo.LastListArgs()
	if orderBy != "id desc" || limit != 10 || offset != 5 {
		t.Fatalf("List 参数未被记录: %q %d %d", orderBy, limit, offset)
	}
}

func TestMemoryGetUpdateDelete(t *testing.T) {
	repo := newMemRepo()
	ctx := context.Background()

	got, err := repo.Get(ctx, uint(2))
	if err != nil || got.Name != "b" {
		t.Fatalf("Get: %+v err=%v", got, err)
	}

	got.Name = "b2"
	if err := repo.Update(ctx, uint(2), &got); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, err := repo.Get(ctx, uint(2))
	if err != nil || after.Name != "b2" {
		t.Fatalf("Update 未生效: %+v err=%v", after, err)
	}

	if err := repo.Delete(ctx, uint(2)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.Get(ctx, uint(2)); !errors.Is(err, crud.ErrNotFound) {
		t.Fatalf("删除后应返回 ErrNotFound，实际 %v", err)
	}
	if repo.Len() != 2 {
		t.Fatalf("删除后应剩 2 条，实际 %d", repo.Len())
	}
}

// TestMemoryUpsertAndDeleteSemantics 锁定与 Repo 一致的 upsert / 宽松删除契约。
func TestMemoryUpsertAndDeleteSemantics(t *testing.T) {
	repo := newMemRepo()
	ctx := context.Background()

	// Update 不存在的记录 → 创建
	if err := repo.Update(ctx, uint(42), &testUser{Name: "ghost"}); err != nil {
		t.Fatalf("Update 不存在的记录不应报错: %v", err)
	}
	got, err := repo.Get(ctx, uint(42))
	if err != nil {
		t.Fatalf("upsert 后应能查到: %v", err)
	}
	if got.Name != "ghost" || got.ID != 42 {
		t.Fatalf("upsert 内容不符: %+v", got)
	}

	// Delete 不存在的记录 → nil
	if err := repo.Delete(ctx, uint(9999)); err != nil {
		t.Fatalf("Delete 不存在的记录应返回 nil，实际 %v", err)
	}
}

func TestMemoryGetMissingReturnsErrNotFound(t *testing.T) {
	repo := newMemRepo()
	if _, err := repo.Get(context.Background(), uint(999)); !errors.Is(err, crud.ErrNotFound) {
		t.Fatalf("Get 应返回 ErrNotFound，实际 %v", err)
	}
}

func TestMemorySeedRespectsExplicitIDs(t *testing.T) {
	repo := crud.NewMemory[testUser]("ID")
	repo.Seed(testUser{ID: 10, Name: "ten"})

	// nextID 应跳到 11，避免与预置主键冲突
	u := &testUser{Name: "auto"}
	if err := repo.Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	if u.ID != 11 {
		t.Fatalf("自增主键应避开预置值，实际 %d", u.ID)
	}
}

func TestMemoryNilItemRejected(t *testing.T) {
	repo := newMemRepo()
	if err := repo.Create(context.Background(), nil); !errors.Is(err, crud.ErrNilItem) {
		t.Fatalf("Create(nil) 应返回 ErrNilItem，实际 %v", err)
	}
}

func TestMemoryInvalidKeyRejected(t *testing.T) {
	repo := newMemRepo()
	if _, err := repo.Get(context.Background(), "not-a-key"); !errors.Is(err, crud.ErrInvalidKey) {
		t.Fatalf("非法主键应返回 ErrInvalidKey，实际 %v", err)
	}
}

func TestMemoryCount(t *testing.T) {
	repo := newMemRepo()
	n, err := repo.Count(context.Background(), crud.CountOptions{})
	if err != nil || n != 3 {
		t.Fatalf("Count 期望 3，实际 %d err=%v", n, err)
	}
}
