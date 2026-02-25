package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/caddy"
	"github.com/netpanel/netpanel/service/cron"
	"github.com/netpanel/netpanel/service/dnsmasq"
	"github.com/netpanel/netpanel/service/storage"
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
	if req.Enable {
		h.mgr.Start(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CaddyHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.CaddySite{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CaddyHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.CaddySite{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *CaddyHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.CaddySite{}).Where("id = ?", id).Update("enable", false)
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
	h.db.First(&cfg)
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
	if req.ID == 0 {
		h.db.Create(&req)
	} else {
		h.db.Save(&req)
	}
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
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *DnsmasqHandler) Stop(c *gin.Context) {
	h.mgr.Stop()
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
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DnsmasqHandler) DeleteRecord(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DnsmasqRecord{}, id)
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

func (h *CronHandler) Create(c *gin.Context) {
	var task model.CronTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&task)
	if task.Enable {
		h.mgr.AddTask(&task)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": task, "message": "创建成功"})
}

func (h *CronHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CronTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.RemoveTask(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	if req.Enable {
		h.mgr.AddTask(&req)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CronHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.RemoveTask(uint(id))
	h.db.Delete(&model.CronTask{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CronHandler) RunNow(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
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
	if req.Enable {
		h.mgr.Start(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *StorageHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.StorageConfig{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *StorageHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *StorageHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.StorageConfig{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}
