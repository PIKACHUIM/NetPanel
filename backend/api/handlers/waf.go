package handlers

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/firewall"
	"github.com/netpanel/netpanel/service/waf"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== WAF 防火墙 / 安全中心 =====

type WafHandler struct {
	db          *gorm.DB
	log         *logrus.Logger
	firewallMgr *firewall.Manager
}

func NewWafHandler(db *gorm.DB, log *logrus.Logger, firewallMgr *firewall.Manager) *WafHandler {
	return &WafHandler{db: db, log: log, firewallMgr: firewallMgr}
}

// clampPage 归一化分页参数：page 从 1 起，page_size 限制在 1..100
func clampPage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func (h *WafHandler) List(c *gin.Context) {
	var configs []model.WafConfig
	h.db.Order("id desc").Find(&configs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *WafHandler) Create(c *gin.Context) {
	var cfg model.WafConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	logger.WriteLog("info", "waf", fmt.Sprintf("创建WAF配置 [%d] %s", cfg.ID, cfg.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *WafHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WafConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "waf", fmt.Sprintf("更新WAF配置 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WafHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.WafConfig{}, id)
	h.db.Where("waf_config_id = ?", id).Delete(&model.WafLog{})
	logger.WriteLog("info", "waf", fmt.Sprintf("删除WAF配置 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WafHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var cfg model.WafConfig
	if err := h.db.First(&cfg, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "WAF 配置不存在"})
		return
	}
	if waf.Default == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "WAF 引擎未初始化"})
		return
	}
	if err := waf.Default.Start(uint(id)); err != nil {
		h.log.Errorf("[WAF] 启动失败: id=%d err=%v", id, err)
		h.db.Model(&model.WafConfig{}).Where("id = ?", id).Updates(map[string]any{
			"enable":     false,
			"status":     "error",
			"last_error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.log.Infof("[WAF] 启动: id=%d name=%s", id, cfg.Name)
	logger.WriteLog("info", "waf", fmt.Sprintf("启动WAF [%d] %s", cfg.ID, cfg.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *WafHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if waf.Default == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "WAF 引擎未初始化"})
		return
	}
	if err := waf.Default.Stop(uint(id)); err != nil {
		h.log.Errorf("[WAF] 停止失败: id=%d err=%v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.log.Infof("[WAF] 停止: id=%d", id)
	logger.WriteLog("info", "waf", fmt.Sprintf("停止WAF [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *WafHandler) GetLogs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = clampPage(page, pageSize)

	var logs []model.WafLog
	var total int64
	h.db.Model(&model.WafLog{}).Where("waf_config_id = ?", id).Count(&total)
	h.db.Where("waf_config_id = ?", id).
		Order("id desc").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&logs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

func (h *WafHandler) TestRule(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		Rule string `json:"rule"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if waf.Default == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "WAF 引擎未初始化"})
		return
	}
	if err := waf.Default.TestRule(req.Rule); err != nil {
		h.log.Errorf("[WAF] 规则校验失败: id=%d err=%v", id, err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": fmt.Sprintf("规则语法错误: %v", err)})
		return
	}

	h.log.Infof("[WAF] 测试规则: id=%d rule=%s", id, req.Rule)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "规则语法正确"})
}

// ===== 安全中心：攻击事件与统计 =====

// EventList 攻击事件列表（全局，按严重级别与时间过滤）
// GET /api/v1/security/waf/events?severity=&keyword=&page=&page_size=
func (h *WafHandler) EventList(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	page, pageSize = clampPage(page, pageSize)
	severity := c.Query("severity")
	keyword := c.Query("keyword")

	query := h.db.Model(&model.WafLog{})
	if severity != "" {
		query = query.Where("severity = ?", severity)
	}
	if keyword != "" {
		query = query.Where("client_ip LIKE ? OR uri LIKE ? OR rule_msg LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	query.Count(&total)

	var logs []model.WafLog
	query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

// Stats 安全中心态势统计
// GET /api/v1/security/waf/stats
func (h *WafHandler) Stats(c *gin.Context) {
	// 今日零点用本地时区计算；Truncate(24h) 会截断到 UTC 午夜，东八区下"今日"会从早上 8 点起算
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	var todayBlocked int64
	h.db.Model(&model.WafLog{}).Where("action = ? AND created_at >= ?", "block", today).Count(&todayBlocked)
	var todayTotal int64
	h.db.Model(&model.WafLog{}).Where("created_at >= ?", today).Count(&todayTotal)
	var banned int64
	h.db.Model(&model.WafBan{}).Where("type = ? AND enable = ?", "black", true).Count(&banned)
	var activeConfigs int64
	h.db.Model(&model.WafConfig{}).Where("status = ?", "running").Count(&activeConfigs)

	// 拦截率：无请求时按 0 处理（避免展示不真实指标），有请求时基于真实事件计算
	blockRate := 0.0
	if todayTotal > 0 {
		blockRate = float64(todayBlocked) / float64(todayTotal) * 100
	}

	// 攻击来源 TOP10
	type srcRow struct {
		ClientIP string
		Cnt      int64
	}
	var sources []srcRow
	h.db.Model(&model.WafLog{}).
		Select("client_ip, COUNT(*) as cnt").
		Where("created_at >= ?", now.Add(-24*time.Hour)).
		Group("client_ip").Order("cnt desc").Limit(10).Scan(&sources)

	// 近 24h 每小时趋势：应用层按"绝对小时桶"（日期+小时）聚合，
	// 既避免依赖 SQLite 专用 strftime 方言，也避免 24h 窗口跨天时昨天与今天同一小时混入同一桶
	type trendRow struct {
		Hour string
		Cnt  int64
	}
	var trendLogs []struct {
		CreatedAt time.Time
	}
	curBucket := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	h.db.Model(&model.WafLog{}).
		Where("created_at >= ?", curBucket.Add(-23*time.Hour)).
		Select("created_at").Scan(&trendLogs)
	bucketCount := make(map[int64]int64)
	for _, l := range trendLogs {
		t := l.CreatedAt.Local()
		b := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
		bucketCount[b.Unix()]++
	}
	trend := make([]trendRow, 0, 24)
	for i := 23; i >= 0; i-- {
		b := curBucket.Add(-time.Duration(i) * time.Hour)
		trend = append(trend, trendRow{Hour: b.Format("01-02 15:00"), Cnt: bucketCount[b.Unix()]})
	}

	// 最近 5 条事件
	var recent []model.WafLog
	h.db.Order("id desc").Limit(5).Find(&recent)

	// 封禁名单最新 5 条
	var recentBans []model.WafBan
	h.db.Where("type = ?", "black").Order("id desc").Limit(5).Find(&recentBans)

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"today_blocked":  todayBlocked,
		"today_total":    todayTotal,
		"block_rate":     blockRate,
		"banned":         banned,
		"active_configs": activeConfigs,
		"top_sources":    sources,
		"trend_24h":      trend,
		"recent_events":  recent,
		"recent_bans":    recentBans,
	}})
}

// ===== 安全中心：封禁 / 黑白名单 =====

// BanList 封禁/黑白名单列表
// GET /api/v1/security/waf/bans?type=&enable=&keyword=
func (h *WafHandler) BanList(c *gin.Context) {
	query := h.db.Model(&model.WafBan{})
	if t := c.Query("type"); t != "" {
		query = query.Where("type = ?", t)
	}
	if k := c.Query("keyword"); k != "" {
		query = query.Where("ip LIKE ? OR reason LIKE ? OR remark LIKE ?", "%"+k+"%", "%"+k+"%", "%"+k+"%")
	}
	var bans []model.WafBan
	query.Order("id desc").Find(&bans)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": bans})
}

// BanCreate 新增封禁/白名单
// POST /api/v1/security/waf/bans  body: {ip, type, reason, expire_at?, remark?}
func (h *WafHandler) BanCreate(c *gin.Context) {
	var req struct {
		IP       string     `json:"ip" binding:"required"`
		Type     string     `json:"type" binding:"required,oneof=black white"`
		Reason   string     `json:"reason"`
		ExpireAt *time.Time `json:"expire_at"`
		Remark   string     `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	// 校验 IP/CIDR 格式，避免非法输入直接写入防火墙规则
	if net.ParseIP(req.IP) == nil {
		if _, _, err := net.ParseCIDR(req.IP); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 IP 或 CIDR 格式"})
			return
		}
	}
	// 检查重复
	var dup int64
	h.db.Model(&model.WafBan{}).Where("ip = ? AND type = ?", req.IP, req.Type).Count(&dup)
	if dup > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "该 IP 已存在于名单中"})
		return
	}

	ban := model.WafBan{
		IP:       req.IP,
		Type:     req.Type,
		Source:   "manual",
		Reason:   req.Reason,
		ExpireAt: req.ExpireAt,
		Enable:   true,
		Remark:   req.Remark,
	}
	if err := h.db.Create(&ban).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}

	// 黑名单自动联动系统防火墙
	if req.Type == "black" {
		h.applyBanToFirewall(&ban)
	}
	logger.WriteLog("info", "waf", fmt.Sprintf("新增封禁名单 [%d] %s (%s)", ban.ID, ban.IP, ban.Type))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": ban, "message": "添加成功"})
}

// BanDelete 删除封禁/白名单（同时解除防火墙规则）
// DELETE /api/v1/security/waf/bans/:id
func (h *WafHandler) BanDelete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var ban model.WafBan
	if err := h.db.First(&ban, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "记录不存在"})
		return
	}
	if ban.FirewallRuleID > 0 {
		h.removeBanFromFirewall(&ban)
	}
	h.db.Delete(&model.WafBan{}, id)
	logger.WriteLog("info", "waf", fmt.Sprintf("删除封禁名单 [%d] %s", id, ban.IP))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// BanApply 手动应用到系统防火墙
// POST /api/v1/security/waf/bans/:id/apply
func (h *WafHandler) BanApply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var ban model.WafBan
	if err := h.db.First(&ban, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "记录不存在"})
		return
	}
	if ban.Type != "black" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "仅黑名单可应用到防火墙"})
		return
	}
	h.applyBanToFirewall(&ban)
	status := "applied"
	msg := "已应用"
	if ban.ApplyStatus == "error" {
		status = "error"
		msg = ban.LastError
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": ban, "message": msg, "status": status})
}

// BanRemove 解除防火墙规则（不删除记录）
// POST /api/v1/security/waf/bans/:id/remove
func (h *WafHandler) BanRemove(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var ban model.WafBan
	if err := h.db.First(&ban, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "记录不存在"})
		return
	}
	h.removeBanFromFirewall(&ban)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已解除"})
}

// applyBanToFirewall 将黑名单应用为系统防火墙 deny 规则
func (h *WafHandler) applyBanToFirewall(ban *model.WafBan) {
	if h.firewallMgr == nil {
		return
	}
	// 已有规则则先移除，并删除旧 FirewallRule 记录，避免反复 apply 积累孤儿规则
	if ban.FirewallRuleID > 0 {
		var old model.FirewallRule
		if err := h.db.First(&old, ban.FirewallRuleID).Error; err == nil {
			_ = h.firewallMgr.RemoveRule(&old)
			h.db.Delete(&model.FirewallRule{}, old.ID)
		}
	}

	rule := model.FirewallRule{
		Name:      fmt.Sprintf("WAF封禁-%s", ban.IP),
		Enable:    true,
		Direction: "in",
		Action:    "deny",
		Protocol:  "all",
		SrcIP:     ban.IP,
		Priority:  1,
		Remark:    fmt.Sprintf("由安全中心自动创建（%s）", ban.Reason),
	}
	if err := h.db.Create(&rule).Error; err != nil {
		ban.ApplyStatus = "error"
		ban.LastError = "创建防火墙规则失败: " + err.Error()
		ban.FirewallRuleID = 0
		h.db.Model(ban).Updates(map[string]any{"apply_status": ban.ApplyStatus, "last_error": ban.LastError})
		return
	}

	if err := h.firewallMgr.ApplyRule(&rule); err != nil {
		// 应用失败时删除已落库的规则记录，避免残留"已启用但未生效"的脏规则
		h.db.Delete(&model.FirewallRule{}, rule.ID)
		ban.FirewallRuleID = 0
		ban.ApplyStatus = "error"
		ban.LastError = err.Error()
	} else {
		ban.FirewallRuleID = rule.ID
		ban.ApplyStatus = "applied"
		ban.LastError = ""
	}
	h.db.Model(ban).Updates(map[string]any{
		"firewall_rule_id": ban.FirewallRuleID,
		"apply_status":     ban.ApplyStatus,
		"last_error":       ban.LastError,
	})
	h.log.Infof("[WAF] 封禁联动防火墙: %s status=%s", ban.IP, ban.ApplyStatus)
}

// removeBanFromFirewall 移除黑名单对应的防火墙规则
func (h *WafHandler) removeBanFromFirewall(ban *model.WafBan) {
	if h.firewallMgr == nil || ban.FirewallRuleID == 0 {
		return
	}
	var rule model.FirewallRule
	if err := h.db.First(&rule, ban.FirewallRuleID).Error; err == nil {
		_ = h.firewallMgr.RemoveRule(&rule)
		h.db.Delete(&model.FirewallRule{}, rule.ID)
	}
	h.db.Model(ban).Updates(map[string]any{
		"firewall_rule_id": 0,
		"apply_status":     "pending",
		"last_error":       "",
	})
	h.log.Infof("[WAF] 解除封禁联动: %s", ban.IP)
}
