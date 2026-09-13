package syslog

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager 系统日志管理器
//
// 写入模型：前台只做非阻塞入队（队列满则丢弃并计数），单一后台 writer
// goroutine 攒批落库。此前 DBHook 对每条日志起一个 goroutine 同步 INSERT，
// 叠加 SQLite 单连接池，日志突发（如 frp 重连风暴）会造成 goroutine 无界堆积。
type Manager struct {
	db  *gorm.DB
	log *logrus.Logger

	queue    chan model.SystemLog
	done     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
	dropped  atomic.Int64 // 队列满被丢弃的条数（周期性汇总告警）
}

const (
	// logQueueSize 日志队列容量：按每秒百条日志可缓冲约 1 分钟
	logQueueSize = 8192
	// logBatchSize 单批落库上限
	logBatchSize = 128
	// logFlushInterval 队列不满时的最大落库延迟
	logFlushInterval = 2 * time.Second
)

// NewManager 创建日志管理器并启动后台批量写入 goroutine
func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	m := &Manager{
		db:    db,
		log:   log,
		queue: make(chan model.SystemLog, logQueueSize),
		done:  make(chan struct{}),
	}
	m.wg.Add(1)
	go m.writerLoop()
	return m
}

// Stop 停止后台写入：关闭入队语义、等待 writer 排空队列后返回
// （优雅关闭时调用；须最后调用，保证其它服务停止期间的日志也能落库）
func (m *Manager) Stop() {
	m.stopOnce.Do(func() { close(m.done) })
	m.wg.Wait()
}

// Write 实现 logger.DBLogWriter 接口：非阻塞入队，由后台 writer 批量落库
func (m *Manager) Write(level, service, message string) {
	entry := model.SystemLog{
		Level:   level,
		Service: service,
		Message: message,
		LogTime: time.Now(),
	}
	select {
	case m.queue <- entry:
	default:
		m.dropped.Add(1)
	}
}

// writerLoop 单一写入者：攒批落库
func (m *Manager) writerLoop() {
	defer m.wg.Done()
	batch := make([]model.SystemLog, 0, logBatchSize)
	ticker := time.NewTicker(logFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case e := <-m.queue:
			batch = append(batch, e)
			batch = m.drain(batch)
			m.flush(batch)
			batch = batch[:0]
		case <-ticker.C:
			if len(batch) > 0 {
				m.flush(batch)
				batch = batch[:0]
			}
		case <-m.done:
			// 排空队列后退出
			for {
				batch = m.drain(batch)
				if len(batch) == 0 {
					return
				}
				m.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

// drain 非阻塞地取尽队列（单批上限内）
func (m *Manager) drain(dst []model.SystemLog) []model.SystemLog {
	for len(dst) < logBatchSize {
		select {
		case e := <-m.queue:
			dst = append(dst, e)
		default:
			return dst
		}
	}
	return dst
}

// flush 批量落库；顺周期性上报丢弃计数
func (m *Manager) flush(batch []model.SystemLog) {
	if len(batch) == 0 {
		return
	}
	if err := m.db.CreateInBatches(batch, len(batch)).Error; err != nil {
		m.log.Warnf("[系统日志] 批量写入失败（%d 条）: %v", len(batch), err)
	}
	if d := m.dropped.Swap(0); d > 0 {
		m.log.Warnf("[系统日志] 队列已满，丢弃 %d 条日志（写入压力过大）", d)
	}
}

// QueryParams 日志查询参数
type QueryParams struct {
	Service  string    // 服务类型筛选，空表示全部
	Level    string    // 日志级别筛选，空表示全部
	Keyword  string    // 关键词搜索
	StartAt  time.Time // 开始时间
	EndAt    time.Time // 结束时间
	Page     int       // 页码（从1开始）
	PageSize int       // 每页数量
	Order    string    // 排序：asc/desc（默认desc）
}

// QueryResult 日志查询结果
type QueryResult struct {
	Total int64             `json:"total"`
	Items []model.SystemLog `json:"items"`
}

// Query 查询日志
func (m *Manager) Query(params QueryParams) (*QueryResult, error) {
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = 50
	}
	if params.PageSize > 500 {
		params.PageSize = 500
	}
	if params.Order != "asc" {
		params.Order = "desc"
	}

	query := m.db.Model(&model.SystemLog{})

	if params.Service != "" {
		query = query.Where("service = ?", params.Service)
	}
	if params.Level != "" {
		query = query.Where("level = ?", params.Level)
	}
	if params.Keyword != "" {
		query = query.Where("message LIKE ?", "%"+params.Keyword+"%")
	}
	if !params.StartAt.IsZero() {
		query = query.Where("log_time >= ?", params.StartAt)
	}
	if !params.EndAt.IsZero() {
		query = query.Where("log_time <= ?", params.EndAt)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var items []model.SystemLog
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order("log_time " + params.Order).
		Offset(offset).Limit(params.PageSize).
		Find(&items).Error; err != nil {
		return nil, err
	}

	return &QueryResult{Total: total, Items: items}, nil
}

// GetServices 获取所有出现过的服务类型列表
func (m *Manager) GetServices() []string {
	var services []string
	m.db.Model(&model.SystemLog{}).
		Distinct("service").
		Pluck("service", &services)
	return services
}

// Cleanup 清理指定天数之前的日志
func (m *Manager) Cleanup(days int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -days)
	result := m.db.Where("log_time < ?", cutoff).Delete(&model.SystemLog{})
	return result.RowsAffected, result.Error
}
