// Package retention 数据保留清理器：
// 定时分批清理时序型数据（监控指标、探测结果、WAF 日志、系统日志、
// DDNS 历史、AI 定时任务日志），防止数据库无限膨胀导致越用越慢。
//
// 设计要点：
//   - 保留天数从 SystemConfig 读取（key: retention_days），默认 30 天；
//   - 启动后延迟 5 分钟再跑第一轮，避免抢启动窗口的写锁；
//   - 之后每 24 小时清理一次；
//   - 分批删除（每批 500 行），避免单条大 DELETE 长时间占用写锁；
//   - 所有删除失败仅记日志，不影响主流程。
package retention

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

const (
	// cfgKeyRetentionDays 保留天数在 SystemConfig 中的键名
	cfgKeyRetentionDays = "retention_days"
	// defaultRetentionDays 默认保留天数
	defaultRetentionDays = 30
	// systemLogRetentionDays 系统日志保留天数（日志量大、价值随时间衰减快）
	systemLogRetentionDays = 7
	// batchSize 分批删除的批次大小
	batchSize = 500
	// startupDelay 启动后延迟首轮清理的时间
	startupDelay = 5 * time.Minute
	// cleanupInterval 清理周期
	cleanupInterval = 24 * time.Hour
)

// tableSpec 一张待清理表：模型 + 时间列 + 保留天数（0 表示用全局默认）
type tableSpec struct {
	model        interface{}
	timeColumn   string
	retentionDays int
}

// Cleaner 数据保留清理器
type Cleaner struct {
	db  *gorm.DB
	log *logrus.Logger
}

// New 创建清理器
func New(db *gorm.DB, log *logrus.Logger) *Cleaner {
	return &Cleaner{db: db, log: log}
}

// Start 启动后台清理循环（非阻塞），返回停止函数
func (c *Cleaner) Start() (stop func()) {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		select {
		case <-time.After(startupDelay):
			c.cleanupAll()
		case <-done:
			return
		}

		for {
			select {
			case <-ticker.C:
				c.cleanupAll()
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}

// specs 待清理表清单
func (c *Cleaner) specs() []tableSpec {
	return []tableSpec{
		{model: &model.MonitorMetric{}, timeColumn: "timestamp"},
		{model: &model.MonitorProbeResult{}, timeColumn: "timestamp"},
		{model: &model.WafLog{}, timeColumn: "created_at"},
		{model: &model.SystemLog{}, timeColumn: "log_time", retentionDays: systemLogRetentionDays},
		{model: &model.DDNSHistory{}, timeColumn: "created_at"},
		{model: &model.AiCronLog{}, timeColumn: "created_at"},
	}
}

// cleanupAll 清理所有表
func (c *Cleaner) cleanupAll() int64 {
	days := c.retentionDays()
	var total int64
	for _, spec := range c.specs() {
		keep := spec.retentionDays
		if keep == 0 {
			keep = days
		}
		cutoff := time.Now().AddDate(0, 0, -keep)
		if n := c.cleanupTable(spec.model, spec.timeColumn, cutoff); n > 0 {
			total += n
			c.log.Infof("[retention] 已清理 %s %d 条（保留 %d 天，截止 %s）",
				fmt.Sprintf("%T", spec.model), n, keep, cutoff.Format("2006-01-02"))
		}
	}
	return total
}

// cleanupTable 分批删除某表中时间早于 cutoff 的记录，返回删除总行数
func (c *Cleaner) cleanupTable(model interface{}, timeColumn string, cutoff time.Time) int64 {
	var total int64
	for i := 0; i < 200; i++ { // 单表单轮上限 10 万行，防止首轮积压跑太久
		res := c.db.Where(fmt.Sprintf("%s < ?", timeColumn), cutoff).Limit(batchSize).Delete(model)
		if res.Error != nil {
			c.log.Warnf("[retention] 清理失败（%T）: %v", model, res.Error)
			return total
		}
		total += res.RowsAffected
		if res.RowsAffected < batchSize {
			return total
		}
	}
	return total
}

// CleanupNow 手动触发一轮全量清理，返回清理总行数（供 API 调用）
func (c *Cleaner) CleanupNow() (total int64, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("清理失败: %v", r)
		}
	}()
	return c.cleanupAll(), nil
}

// retentionDays 从 SystemConfig 读取保留天数，缺失/非法时回落默认值
func (c *Cleaner) retentionDays() int {
	var cfg model.SystemConfig
	if err := c.db.Where("key = ?", cfgKeyRetentionDays).First(&cfg).Error; err != nil {
		return defaultRetentionDays
	}
	var n int
	if _, err := fmt.Sscanf(cfg.Value, "%d", &n); err != nil || n <= 0 {
		return defaultRetentionDays
	}
	if n > 365 {
		n = 365
	}
	return n
}
