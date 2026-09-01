package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// WafAnalyticsHandler WAF 事件分析处理器
type WafAnalyticsHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewWafAnalyticsHandler(db *gorm.DB, log *logrus.Logger) *WafAnalyticsHandler {
	return &WafAnalyticsHandler{db: db, log: log}
}

// GetAnalytics 获取 WAF 分析数据
func (h *WafAnalyticsHandler) GetAnalytics(c *gin.Context) {
	// 查询参数
	startStr := c.Query("start")
	endStr := c.Query("end")
	ruleID := c.Query("rule_id")
	severities := c.Query("severity")

	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()
	if startStr != "" {
		if t, err := time.Parse("2006-01-02", startStr); err == nil {
			startTime = t
		}
	}
	if endStr != "" {
		if t, err := time.Parse("2006-01-02", endStr); err == nil {
			endTime = t
		}
	}

	// 基础查询
	query := h.db.Model(&model.WafLog{}).Where("log_time BETWEEN ? AND ?", startTime, endTime)
	if ruleID != "" {
		query = query.Where("rule_id = ?", ruleID)
	}
	if severities != "" {
		query = query.Where("severity IN (?)", severities)
	}

	// 攻击类型分布
	type AttackType struct {
		RuleMsg string `json:"rule_msg"`
		Count   int64  `json:"count"`
	}
	var attackTypes []AttackType
	query.Model(&model.WafLog{}).Select("rule_msg, COUNT(*) as count").Group("rule_msg").Order("count DESC").Limit(10).Find(&attackTypes)

	//  severity 分布
	type SeverityCount struct {
		Severity string `json:"severity"`
		Count    int64  `json:"count"`
	}
	var severityCounts []SeverityCount
	query.Model(&model.WafLog{}).Select("severity, COUNT(*) as count").Group("severity").Order("count DESC").Find(&severityCounts)

	// 时间序列（按小时）
	type TimeSeriesPoint struct {
		Timestamp string `json:"timestamp"`
		Count     int64  `json:"count"`
	}
	var timeSeries []TimeSeriesPoint
	query.Select("strftime('%Y-%m-%d %H:00', log_time) as timestamp, COUNT(*) as count").
		Group("timestamp").
		Order("timestamp ASC").
		Find(&timeSeries)

	// Top IP
	type TopIP struct {
		IP    string `json:"ip"`
		Count int64  `json:"count"`
	}
	var topIPs []TopIP
	query.Select("client_ip as ip, COUNT(*) as count").Group("client_ip").Order("count DESC").Limit(10).Find(&topIPs)

	// 总数
	var total int64
	query.Count(&total)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"total":        total,
			"attack_types": attackTypes,
			"severities":   severityCounts,
			"time_series":  timeSeries,
			"top_ips":      topIPs,
		},
	})
}

// GetLogs 获取 WAF 日志列表（分页）
func (h *WafAnalyticsHandler) GetLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}

	query := h.db.Model(&model.WafLog{}).Order("id DESC")

	// 筛选
	if ruleID := c.Query("rule_id"); ruleID != "" {
		query = query.Where("rule_id = ?", ruleID)
	}
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if severity := c.Query("severity"); severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if keyword := c.Query("keyword"); keyword != "" {
		query = query.Where("rule_msg LIKE ? OR uri LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var total int64
	query.Count(&total)

	var logs []model.WafLog
	offset := (page - 1) * pageSize
	query.Offset(offset).Limit(pageSize).Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"data":    logs,
		"total":   total,
		"page":    page,
		"page_size": pageSize,
	})
}
