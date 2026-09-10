package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== 域名解析 =====

type DomainRecordHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainRecordHandler(db *gorm.DB, log *logrus.Logger) *DomainRecordHandler {
	return &DomainRecordHandler{db: db, log: log}
}

func (h *DomainRecordHandler) List(c *gin.Context) {
	domainInfoID := c.Query("domain_info_id")
	accountID := c.Query("account_id")
	var records []model.DomainRecord
	query := h.db.Order("id desc")
	if domainInfoID != "" {
		query = query.Where("domain_info_id = ?", domainInfoID)
	} else if accountID != "" {
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
	// 从 DomainInfo 获取 account_id
	if record.DomainInfoID > 0 && record.DomainAccountID == 0 {
		var domain model.DomainInfo
		if err := h.db.First(&domain, record.DomainInfoID).Error; err == nil {
			record.DomainAccountID = domain.AccountID
			record.Domain = domain.Name
		}
	}
	h.db.Create(&record)
	logger.WriteLog("info", "domain", fmt.Sprintf("创建解析记录 [%d] %s %s", record.ID, record.RecordType, record.Host))
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
	logger.WriteLog("info", "domain", fmt.Sprintf("更新解析记录 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DomainRecordHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainRecord{}, id)
	logger.WriteLog("info", "domain", fmt.Sprintf("删除解析记录 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *DomainRecordHandler) SyncFromProvider(c *gin.Context) {
	domainInfoIDStr := c.Param("domainInfoId")
	domainInfoID, err := strconv.ParseUint(domainInfoIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数格式错误"})
		return
	}
	var domain model.DomainInfo
	if err := h.db.First(&domain, domainInfoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "域名不存在"})
		return
	}
	var acc model.DomainAccount
	if err := h.db.First(&acc, domain.AccountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}

	h.log.Infof("[域名解析] 同步解析记录: domain=%s provider=%s", domain.Name, acc.Provider)

	// 使用 doSyncFromProvider 统一处理同步逻辑
	domainInfoHandler := &DomainInfoHandler{db: h.db, log: h.log}
	count, err := domainInfoHandler.DoSyncFromProvider(c.Request.Context(), uint(domainInfoID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "同步完成", "data": gin.H{"count": count}})
}
