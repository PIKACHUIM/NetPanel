package syslog

import (
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager 系统日志管理器
type Manager struct {
	db  *gorm.DB
	log *logrus.Logger
	mu  sync.Mutex
}

// NewManager 创建日志管理器
func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// Write 实现 logger.DBLogWriter 接口，将日志写入数据库
func (m *Manager) Write(level, service, message string) {
	entry := &model.SystemLog{
		Level:   level,
		Service: service,
		Message: message,
		LogTime: time.Now(),
	}
	m.db.Create(entry)
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
