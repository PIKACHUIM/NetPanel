package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/caddy"
	"github.com/netpanel/netpanel/service/cron"
	"github.com/netpanel/netpanel/service/dnsmasq"
	"github.com/netpanel/netpanel/service/storage"
	cronv3 "github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== Caddy =====

type CaddyHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *caddy.Manager
}

func NewCaddyHandler(db *gorm.DB, log *logrus.Logger, mgr *caddy.Manager) *CaddyHandler {
	return &CaddyHandler{db: db, log: log, mgr: mgr}
}

func (h *CaddyHandler) List(c *gin.Context) {
	var sites []model.CaddySite
	h.db.Order("id desc").Find(&sites)
	for i := range sites {
		sites[i].Status = h.mgr.GetStatus(sites[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": sites})
}

func (h *CaddyHandler) Create(c *gin.Context) {
	var site model.CaddySite
	if err := c.ShouldBindJSON(&site); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	site.Status = "stopped"
	h.db.Create(&site)
	logger.WriteLog("info", "caddy", fmt.Sprintf("创建网页服务: ID=%d", site.ID))
	if site.Enable {
		h.mgr.Start(site.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": site, "message": "创建成功"})
}

func (h *CaddyHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CaddySite
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.Stop(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "caddy", fmt.Sprintf("修改网页服务: ID=%d", id))
	if req.Enable {
		h.mgr.Start(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CaddyHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.CaddySite{}, id)
	logger.WriteLog("info", "caddy", fmt.Sprintf("删除网页服务: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CaddyHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.CaddySite{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "caddy", fmt.Sprintf("启动网页服务: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *CaddyHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.CaddySite{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "caddy", fmt.Sprintf("停止网页服务: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// ===== DNSMasq =====

type DnsmasqHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *dnsmasq.Manager
}

func NewDnsmasqHandler(db *gorm.DB, log *logrus.Logger, mgr *dnsmasq.Manager) *DnsmasqHandler {
	return &DnsmasqHandler{db: db, log: log, mgr: mgr}
}

func (h *DnsmasqHandler) GetConfig(c *gin.Context) {
	var cfg model.DnsmasqConfig
	h.db.Order("id desc").First(&cfg)
	cfg.Status = h.mgr.GetStatus()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg})
}

func (h *DnsmasqHandler) UpdateConfig(c *gin.Context) {
	var req model.DnsmasqConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.Stop()
	// 如果前端未传 ID，尝试查找已有配置进行更新（全局单例配置）
	if req.ID == 0 {
		var existing model.DnsmasqConfig
		if err := h.db.First(&existing).Error; err == nil {
			req.ID = existing.ID
		}
	}
	if req.ID == 0 {
		h.db.Create(&req)
	} else {
		h.db.Save(&req)
	}
	logger.WriteLog("info", "dnsmasq", "修改 DNSMasq 配置")
	if req.Enable {
		h.mgr.Start()
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "配置已更新"})
}

func (h *DnsmasqHandler) Start(c *gin.Context) {
	if err := h.mgr.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "dnsmasq", "启动 DNSMasq 服务")
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *DnsmasqHandler) Stop(c *gin.Context) {
	h.mgr.Stop()
	logger.WriteLog("info", "dnsmasq", "停止 DNSMasq 服务")
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *DnsmasqHandler) ListRecords(c *gin.Context) {
	var records []model.DnsmasqRecord
	h.db.Order("id desc").Find(&records)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": records})
}

func (h *DnsmasqHandler) CreateRecord(c *gin.Context) {
	var record model.DnsmasqRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&record)
	logger.WriteLog("info", "dnsmasq", fmt.Sprintf("创建 DNS 记录: ID=%d", record.ID))
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": record, "message": "创建成功"})
}

func (h *DnsmasqHandler) UpdateRecord(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DnsmasqRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "dnsmasq", fmt.Sprintf("修改 DNS 记录: ID=%d", id))
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DnsmasqHandler) DeleteRecord(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DnsmasqRecord{}, id)
	logger.WriteLog("info", "dnsmasq", fmt.Sprintf("删除 DNS 记录: ID=%d", id))
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// ===== Cron =====

type CronHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *cron.Manager
}

func NewCronHandler(db *gorm.DB, log *logrus.Logger, mgr *cron.Manager) *CronHandler {
	return &CronHandler{db: db, log: log, mgr: mgr}
}

func (h *CronHandler) List(c *gin.Context) {
	var tasks []model.CronTask
	h.db.Order("id desc").Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tasks})
}

// validateCronTask 校验计划任务的关键字段。
// cron 表达式非法会导致 AddTask 静默失败（任务永不触发），必须前置校验。
func validateCronTask(task *model.CronTask) error {
	if strings.TrimSpace(task.CronExpr) == "" {
		return fmt.Errorf("cron 表达式不能为空")
	}
	// 与 cron.Manager 使用同一解析规则（cron.WithSeconds()，即 6 段带秒格式），
	// 否则校验通过的表达式仍可能被 AddFunc 拒绝而导致任务静默不触发
	if _, err := cronv3.NewParser(
		cronv3.Second | cronv3.Minute | cronv3.Hour | cronv3.Dom | cronv3.Month | cronv3.Dow | cronv3.Descriptor,
	).Parse(task.CronExpr); err != nil {
		return fmt.Errorf("cron 表达式非法（需为 6 段带秒格式，如 \"0 0 3 * * *\"）: %v", err)
	}
	switch task.TaskType {
	case "shell":
		if strings.TrimSpace(task.Command) == "" {
			return fmt.Errorf("shell 任务的命令不能为空")
		}
		if !cron.ShellTaskEnabled() {
			return fmt.Errorf("shell 任务已禁用（该类型可执行任意系统命令）；如确需启用，请设置环境变量 NETPANEL_ENABLE_SHELL_TASK=1 后重启")
		}
	case "http":
		if strings.TrimSpace(task.HTTPURL) == "" {
			return fmt.Errorf("http 任务的 URL 不能为空")
		}
	case "renew_cert", "update_ddns", "wol", "sync_dns_record":
		if task.TargetID == 0 {
			return fmt.Errorf("该任务类型需要指定目标 ID")
		}
	default:
		return fmt.Errorf("未知任务类型: %q", task.TaskType)
	}
	return nil
}

func (h *CronHandler) Create(c *gin.Context) {
	var task model.CronTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := validateCronTask(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.db.Create(&task).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cron", fmt.Sprintf("创建计划任务: ID=%d type=%s", task.ID, task.TaskType))
	if task.Enable {
		h.mgr.AddTask(&task)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": task, "message": "创建成功"})
}

func (h *CronHandler) Update(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var req model.CronTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := validateCronTask(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	// 确认任务存在，并保留创建时间等原有字段
	var existing model.CronTask
	if err := h.db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "任务不存在"})
		return
	}

	h.mgr.RemoveTask(id)
	// 使用 Updates 按字段更新，避免 Save 全量覆盖将未提交字段写为零值
	updates := map[string]any{
		"name":        req.Name,
		"cron_expr":   req.CronExpr,
		"task_type":   req.TaskType,
		"command":     req.Command,
		"http_url":    req.HTTPURL,
		"http_method": req.HTTPMethod,
		"http_body":   req.HTTPBody,
		"target_id":   req.TargetID,
		"enable":      req.Enable,
		"remark":      req.Remark,
	}
	if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cron", fmt.Sprintf("修改计划任务: ID=%d", id))

	if err := h.db.First(&existing, id).Error; err == nil && existing.Enable {
		h.mgr.AddTask(&existing)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": existing, "message": "更新成功"})
}

func (h *CronHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	h.mgr.RemoveTask(id)
	if err := h.db.Delete(&model.CronTask{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cron", fmt.Sprintf("删除计划任务: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CronHandler) Enable(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var task model.CronTask
	if err := h.db.First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "任务不存在"})
		return
	}
	// 启用前复核：避免历史遗留的非法配置在启用时静默失效
	if err := validateCronTask(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.db.Model(&task).Update("enable", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "启用失败: " + err.Error()})
		return
	}
	task.Enable = true
	h.mgr.AddTask(&task)
	logger.WriteLog("info", "cron", fmt.Sprintf("启用计划任务: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启用"})
}

func (h *CronHandler) Disable(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	h.mgr.RemoveTask(id)
	if err := h.db.Model(&model.CronTask{}).Where("id = ?", id).Update("enable", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "禁用失败: " + err.Error()})
		return
	}
	logger.WriteLog("info", "cron", fmt.Sprintf("禁用计划任务: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已禁用"})
}

func (h *CronHandler) RunNow(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logger.WriteLog("info", "cron", fmt.Sprintf("手动执行计划任务: ID=%d", id))
	if err := h.mgr.RunNow(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已触发执行"})
}

// ===== Storage =====

type StorageHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *storage.Manager
}

func NewStorageHandler(db *gorm.DB, log *logrus.Logger, mgr *storage.Manager) *StorageHandler {
	return &StorageHandler{db: db, log: log, mgr: mgr}
}

func (h *StorageHandler) List(c *gin.Context) {
	var configs []model.StorageConfig
	h.db.Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *StorageHandler) Create(c *gin.Context) {
	var cfg model.StorageConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	logger.WriteLog("info", "storage", fmt.Sprintf("创建网络存储: ID=%d", cfg.ID))
	if cfg.Enable {
		h.mgr.Start(cfg.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *StorageHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.StorageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.Stop(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "storage", fmt.Sprintf("修改网络存储: ID=%d", id))
	if req.Enable {
		h.mgr.Start(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *StorageHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.StorageConfig{}, id)
	logger.WriteLog("info", "storage", fmt.Sprintf("删除网络存储: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *StorageHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "storage", fmt.Sprintf("启动网络存储: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *StorageHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "storage", fmt.Sprintf("停止网络存储: ID=%d", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}
