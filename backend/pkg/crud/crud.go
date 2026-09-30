// Package crud 提供基于 GORM 的泛型 CRUD 仓储层。
//
// 背景：项目中大量 handler 直接在 API 层裸写 gorm 查询（Find/First/Save/Delete），
// 业务逻辑与数据访问耦合，既无法单元测试，也重复了大量样板代码。
// 本包把「纯 CRUD」的数据访问收敛为可复用、可单测的泛型实现；
// 带业务逻辑的 handler（firewall 校验、frp 配置生成、STUN 真实转发等）不适用。
//
// 设计约束：
//   - 仅覆盖单表 CRUD，不引入 JOIN / 聚合 / 事务编排
//   - 零值安全：T 必须是可寻址类型（struct 或指针），由 new(T) 分配
//   - 不吞错误：所有方法返回 error，由调用方决定 HTTP 响应
package crud

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// ErrNotFound 表示按主键查询未命中。
var ErrNotFound = gorm.ErrRecordNotFound

// ErrNilItem 表示传入的实体指针为 nil。
var ErrNilItem = errors.New("crud: item 不能为 nil")

// ErrInvalidKey 表示主键类型不受支持。
var ErrInvalidKey = errors.New("crud: 主键必须是 uint/int 类型")

// Model 是 T 的类型约束。
//
// 这里用 `any` 而非 `Table() string` 之类的接口方法：gorm 本身已经能从结构体
// 类型推断表名，要求每个 model 额外实现接口会给 60+ 个既有 model 增加无收益的改动。
// 真正的类型安全由 Repo 方法体内 `new(T)` / `&item` 的可寻址要求在编译期保证——
// 若 T 不是结构体，编译器会在实例化时报错。
type Model interface {
	any
}

// ListOptions 列表查询选项。零值即「不限排序、不分页」。
type ListOptions struct {
	// OrderBy 排序表达式，如 "id desc" 或 "created_at asc, id desc"。
	// 为空时不加 ORDER BY。
	OrderBy string
	// Limit 限制条数，<=0 表示不限制。
	Limit int
	// Offset 跳过条数，<0 视为 0。
	Offset int
	// Where 附加过滤条件，等价于 db.Where(cond, args...)。
	Where map[string]any
	// WhereArgs Where 条件对应的值，按 key 的字典序应用。
	// 与 Where 分离是为了避免调用方构造 []interface{} 时顺序漂移。
	WhereArgs map[string]any
}

// applyTo 将选项应用到 gorm 查询链。
func (o ListOptions) applyTo(db *gorm.DB) *gorm.DB {
	if len(o.Where) > 0 {
		// map 遍历顺序随机，但每个 key 条件互相独立（AND 连接），
		// 因此排序仅为可测试性服务，不影响语义。
		for _, k := range sortedKeys(o.Where) {
			db = db.Where(k, o.Where[k])
		}
	}
	for _, k := range sortedKeys(o.WhereArgs) {
		db = db.Where(k, o.WhereArgs[k])
	}
	if o.OrderBy != "" {
		db = db.Order(o.OrderBy)
	}
	if o.Limit > 0 {
		db = db.Limit(o.Limit)
	}
	if o.Offset > 0 {
		db = db.Offset(o.Offset)
	}
	return db
}

// CountOptions 计数查询选项。
type CountOptions struct {
	Where     map[string]any
	WhereArgs map[string]any
}

// Store 是 T 的通用仓储接口，handler 只依赖它，不依赖具体实现。
//
// 方法集刻意保持最小：List / Count / Get / Create / Update / Delete。
// 任何需要额外能力（如软删除过滤、关联预加载）的场景，
// 应通过 gorm 扩展 Options 而非扩张接口。
//
// 语义约定（迁移必须保持，否则会改变 API 行为）：
//   - Update 为 upsert 语义，沿用 gorm Save
//   - Delete 对不存在的记录返回 nil，不视为错误
//   - List 无结果时返回空切片而非 nil，避免 JSON 序列化为 null
//
// 生产用 Repo（gorm），测试用 MemoryRepo，两者行为契约一致。
type Store[T Model] interface {
	List(ctx context.Context, opts ListOptions) ([]T, error)
	Count(ctx context.Context, opts CountOptions) (int64, error)
	Get(ctx context.Context, id any) (T, error)
	Create(ctx context.Context, item *T) error
	Update(ctx context.Context, id any, item *T) error
	Delete(ctx context.Context, id any) error
}

// Repo 是基于 gorm 的 Store 实现。
type Repo[T Model] struct {
	db *gorm.DB
}

// New 构造 T 的仓储。
func New[T Model](db *gorm.DB) *Repo[T] {
	return &Repo[T]{db: db}
}

// DB 暴露底层 gorm 实例，供需要关联预加载（Preload）的调用方使用。
func (r *Repo[T]) DB() *gorm.DB { return r.db }

// List 按选项返回多条记录。返回值始终非 nil 切片（无结果时为空切片），
// 避免调用方拿到 nil 后 json 序列化成 null。
func (r *Repo[T]) List(ctx context.Context, opts ListOptions) ([]T, error) {
	var items []T
	if err := opts.applyTo(r.db.WithContext(ctx)).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("crud list: %w", err)
	}
	if items == nil {
		items = []T{}
	}
	return items, nil
}

// Count 返回符合条件的记录总数。
func (r *Repo[T]) Count(ctx context.Context, opts CountOptions) (int64, error) {
	var n int64
	q := r.db.WithContext(ctx).Model(new(T))
	for _, k := range sortedKeys(opts.Where) {
		q = q.Where(k, opts.Where[k])
	}
	for _, k := range sortedKeys(opts.WhereArgs) {
		q = q.Where(k, opts.WhereArgs[k])
	}
	if err := q.Count(&n).Error; err != nil {
		return 0, fmt.Errorf("crud count: %w", err)
	}
	return n, nil
}

// Get 按主键查询单条记录。未命中返回 ErrNotFound。
func (r *Repo[T]) Get(ctx context.Context, id any) (T, error) {
	var item T
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		var zero T
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return zero, ErrNotFound
		}
		return zero, fmt.Errorf("crud get: %w", err)
	}
	return item, nil
}

// Create 插入记录并回填自增主键与时间戳。
func (r *Repo[T]) Create(ctx context.Context, item *T) error {
	if item == nil {
		return ErrNilItem
	}
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("crud create: %w", err)
	}
	return nil
}

// Update 按主键写入记录，沿用 gorm Save 的 upsert 语义
// （主键非零时先 UPDATE，影响 0 行则 INSERT）。
// 记录不存在时会创建它，与改造前 h.db.Save 的行为一致。
func (r *Repo[T]) Update(ctx context.Context, id any, item *T) error {
	if item == nil {
		return ErrNilItem
	}
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("crud update: %w", err)
	}
	return nil
}

// Delete 按主键删除记录。记录不存在时不视为错误（沿用 gorm 行为），
// 与改造前 h.db.Delete(...) 忽略返回值直接返回 200 的行为一致。
func (r *Repo[T]) Delete(ctx context.Context, id any) error {
	if err := r.db.WithContext(ctx).Delete(new(T), id).Error; err != nil {
		return fmt.Errorf("crud delete: %w", err)
	}
	return nil
}

// MustRepo 适用于包级单例场景：db 为 nil 时直接 panic，
// 让配置错误在启动阶段暴露，而不是等到首个请求变成 500。
func MustRepo[T Model](db *gorm.DB) *Repo[T] {
	if db == nil {
		panic("crud: db 不能为 nil")
	}
	return New[T](db)
}

// sortedKeys 返回 map 的有序 key 列表。
func sortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 插入排序：key 数量是配置项级别的小规模，避免引入 sort 依赖
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && strings.Compare(keys[j-1], keys[j]) > 0; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
