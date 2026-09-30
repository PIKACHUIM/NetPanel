package crud

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

// MemoryRepo 是 Store 的内存实现，用于 handler 单元测试。
//
// 它只保证 CRUD 与分页语义，不模拟 SQL 排序表达式（OrderBy 仅记录不执行）、
// 也不做 Where 过滤——内存实现不承担这两项语义。
// 因此适合「handler 是否正确调用仓储并渲染响应」这类断言，
// 而 SQL 拼接正确性应由针对 gorm 的集成测试（sqlite）覆盖。
type MemoryRepo[T Model] struct {
	mu    sync.RWMutex
	items map[uint]T
	// idField 主键字段名。
	idField string
	// nextID 自增主键分配器。
	nextID uint
	// lastOrderBy / lastLimit / lastOffset 记录最后一次 List 的选项，
	// 供 handler 测试断言调用参数。
	lastOrderBy        string
	lastLimit, lastOff int
}

// NewMemory 构造空的内存仓储。
// idField 为实体主键字段名，通常是 "ID"；该字段必须存在且可导出。
func NewMemory[T Model](idField string) *MemoryRepo[T] {
	return &MemoryRepo[T]{items: make(map[uint]T), idField: idField, nextID: 1}
}

// LastListArgs 返回最后一次 List 调用的选项，便于 handler 测试断言。
func (m *MemoryRepo[T]) LastListArgs() (orderBy string, limit, offset int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastOrderBy, m.lastLimit, m.lastOff
}

// Len 返回当前记录数。
func (m *MemoryRepo[T]) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.items)
}

// idOf 读取 T 的主键字段值。
func (m *MemoryRepo[T]) idOf(v T) uint {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	f := rv.FieldByName(m.idField)
	if !f.IsValid() {
		panic(fmt.Sprintf("crud: MemoryRepo 找不到主键字段 %q", m.idField))
	}
	return uint(f.Uint())
}

// setID 写入 T 的主键字段值。
func (m *MemoryRepo[T]) setID(v *T, id uint) {
	rv := reflect.ValueOf(v).Elem()
	f := rv.FieldByName(m.idField)
	if !f.IsValid() || !f.CanSet() {
		panic(fmt.Sprintf("crud: MemoryRepo 无法写入主键字段 %q", m.idField))
	}
	f.SetUint(uint64(id))
}

// List 固定按主键升序返回，忽略 OrderBy 与 Where，仅应用 Limit/Offset。
func (m *MemoryRepo[T]) List(_ context.Context, opts ListOptions) ([]T, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastOrderBy = opts.OrderBy
	m.lastLimit = opts.Limit
	m.lastOff = opts.Offset

	out := make([]T, 0, len(m.items))
	for _, v := range m.items {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return m.idOf(out[i]) < m.idOf(out[j]) })

	if opts.Offset > 0 {
		if opts.Offset >= len(out) {
			return []T{}, nil
		}
		out = out[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(out) {
		out = out[:opts.Limit]
	}
	return out, nil
}

// Count 返回全部记录数（忽略过滤条件）。
func (m *MemoryRepo[T]) Count(_ context.Context, _ CountOptions) (int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return int64(len(m.items)), nil
}

// Get 按主键返回记录，未命中返回 ErrNotFound。
func (m *MemoryRepo[T]) Get(_ context.Context, id any) (T, error) {
	key, err := toKey(id)
	if err != nil {
		var zero T
		return zero, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	v, ok := m.items[key]
	if !ok {
		var zero T
		return zero, ErrNotFound
	}
	return v, nil
}

// Create 分配自增主键后存入。
func (m *MemoryRepo[T]) Create(_ context.Context, item *T) error {
	if item == nil {
		return ErrNilItem
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	id := m.nextID
	m.nextID++
	m.setID(item, id)
	m.items[id] = *item
	return nil
}

// Update 覆盖已存在记录；不存在时创建（upsert，语义与 Repo 保持一致）。
func (m *MemoryRepo[T]) Update(_ context.Context, id any, item *T) error {
	if item == nil {
		return ErrNilItem
	}
	key, err := toKey(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	// 对齐 gorm Save：0 行受影响则 INSERT
	if _, ok := m.items[key]; !ok && key >= m.nextID {
		m.nextID = key + 1
	}
	m.setID(item, key)
	m.items[key] = *item
	return nil
}

// Delete 按主键删除。记录不存在时不视为错误（对齐 gorm 行为）。
func (m *MemoryRepo[T]) Delete(_ context.Context, id any) error {
	key, err := toKey(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
	return nil
}

// Seed 预置记录，直接使用 item 自身主键。
func (m *MemoryRepo[T]) Seed(items ...T) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, it := range items {
		id := m.idOf(it)
		m.items[id] = it
		if id >= m.nextID {
			m.nextID = id + 1
		}
	}
}

// toKey 把任意主键表示归一为 uint。
func toKey(id any) (uint, error) {
	switch v := id.(type) {
	case uint:
		return v, nil
	case uint64:
		return uint(v), nil
	case int:
		if v < 0 {
			return 0, ErrInvalidKey
		}
		return uint(v), nil
	default:
		return 0, ErrInvalidKey
	}
}
