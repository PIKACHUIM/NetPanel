package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/service/access"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== WOL =====

type WolHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewWolHandler(db *gorm.DB, log *logrus.Logger) *WolHandler {
	return &WolHandler{db: db, log: log}
}

func (h *WolHandler) List(c *gin.Context) {
	var devices []model.WolDevice
	h.db.Order("id desc").Find(&devices)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": devices})
}

func (h *WolHandler) Create(c *gin.Context) {
	var device model.WolDevice
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&device)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": device, "message": "创建成功"})
}

func (h *WolHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WolDevice
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WolHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.WolDevice{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WolHandler) Wake(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var device model.WolDevice
	if err := h.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "设备不存在"})
		return
	}
	if err := sendWakePacket(device.MACAddress, device.BroadcastIP, device.NetInterface, device.Port); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "唤醒失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "唤醒包已发送"})
}

// ===== 域名账号 =====

type DomainAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainAccountHandler(db *gorm.DB, log *logrus.Logger) *DomainAccountHandler {
	return &DomainAccountHandler{db: db, log: log}
}

func (h *DomainAccountHandler) List(c *gin.Context) {
	var accounts []model.DomainAccount
	h.db.Order("id desc").Find(&accounts)
	// 隐藏 Secret
	for i := range accounts {
		if len(accounts[i].AccessSecret) > 4 {
			accounts[i].AccessSecret = "****" + accounts[i].AccessSecret[len(accounts[i].AccessSecret)-4:]
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *DomainAccountHandler) Create(c *gin.Context) {
	var account model.DomainAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *DomainAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DomainAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// ===== 域名证书 =====

type CertHandler struct {
	db     *gorm.DB
	log    *logrus.Logger
	config *config.Config
}

func NewCertHandler(db *gorm.DB, log *logrus.Logger, cfg *config.Config) *CertHandler {
	return &CertHandler{db: db, log: log, config: cfg}
}

func (h *CertHandler) List(c *gin.Context) {
	var certs []model.DomainCert
	h.db.Order("id desc").Find(&certs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": certs})
}

func (h *CertHandler) Create(c *gin.Context) {
	var cert model.DomainCert
	if err := c.ShouldBindJSON(&cert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cert.Status = "pending"
	h.db.Create(&cert)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cert, "message": "创建成功"})
}

func (h *CertHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainCert
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CertHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainCert{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CertHandler) Renew(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// TODO: 实现 ACME 证书续期
	h.log.Infof("触发证书续期: %d", id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "证书续期任务已提交"})
}

// ===== 域名解析 =====

type DomainRecordHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainRecordHandler(db *gorm.DB, log *logrus.Logger) *DomainRecordHandler {
	return &DomainRecordHandler{db: db, log: log}
}

func (h *DomainRecordHandler) List(c *gin.Context) {
	accountID := c.Query("account_id")
	var records []model.DomainRecord
	query := h.db.Order("id desc")
	if accountID != "" {
		query = query.Where("domain_account_id = ?", accountID)
	}
	query.Find(&records)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": records})
}

func (h *DomainRecordHandler) Create(c *gin.Context) {
	var record model.DomainRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&record)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": record, "message": "创建成功"})
}

func (h *DomainRecordHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DomainRecordHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainRecord{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *DomainRecordHandler) SyncFromProvider(c *gin.Context) {
	// TODO: 从服务商同步解析记录
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "同步任务已提交"})
}

// ===== IP 地址库 =====

type IPDBHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewIPDBHandler(db *gorm.DB, log *logrus.Logger) *IPDBHandler {
	return &IPDBHandler{db: db, log: log}
}

func (h *IPDBHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	var entries []model.IPDBEntry
	var total int64
	query := h.db.Model(&model.IPDBEntry{})
	if keyword != "" {
		query = query.Where("cidr LIKE ? OR location LIKE ? OR tags LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Count(&total)
	query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      entries,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

func (h *IPDBHandler) Create(c *gin.Context) {
	var entry model.IPDBEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&entry)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": entry, "message": "创建成功"})
}

func (h *IPDBHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.IPDBEntry
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *IPDBHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.IPDBEntry{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *IPDBHandler) Import(c *gin.Context) {
	var entries []model.IPDBEntry
	if err := c.ShouldBindJSON(&entries); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.CreateInBatches(&entries, 100)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入成功", "data": gin.H{"count": len(entries)}})
}

func (h *IPDBHandler) Query(c *gin.Context) {
	var req struct {
		IP string `json:"ip"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	// TODO: 实现 IP 归属地查询
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"ip": req.IP, "location": "未知"}})
}

// ===== 访问控制 =====

type AccessHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *access.Manager
}

func NewAccessHandler(db *gorm.DB, log *logrus.Logger, mgr *access.Manager) *AccessHandler {
	return &AccessHandler{db: db, log: log, mgr: mgr}
}

func (h *AccessHandler) List(c *gin.Context) {
	var rules []model.AccessRule
	h.db.Order("id desc").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rules})
}

func (h *AccessHandler) Create(c *gin.Context) {
	var rule model.AccessRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&rule)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rule, "message": "创建成功"})
}

func (h *AccessHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.AccessRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *AccessHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.AccessRule{}, id)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// ===== 回调账号 =====

type CallbackAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *callback.Manager
}

func NewCallbackAccountHandler(db *gorm.DB, log *logrus.Logger, mgr *callback.Manager) *CallbackAccountHandler {
	return &CallbackAccountHandler{db: db, log: log, mgr: mgr}
}

func (h *CallbackAccountHandler) List(c *gin.Context) {
	var accounts []model.CallbackAccount
	h.db.Order("id desc").Find(&accounts)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *CallbackAccountHandler) Create(c *gin.Context) {
	var account model.CallbackAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *CallbackAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CallbackAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CallbackAccountHandler) Test(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.TestAccount(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "测试失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "测试成功"})
}

// ===== 回调任务 =====

type CallbackTaskHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewCallbackTaskHandler(db *gorm.DB, log *logrus.Logger) *CallbackTaskHandler {
	return &CallbackTaskHandler{db: db, log: log}
}

func (h *CallbackTaskHandler) List(c *gin.Context) {
	var tasks []model.CallbackTask
	h.db.Order("id desc").Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tasks})
}

func (h *CallbackTaskHandler) Create(c *gin.Context) {
	var task model.CallbackTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&task)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": task, "message": "创建成功"})
}

func (h *CallbackTaskHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackTaskHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CallbackTask{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}
