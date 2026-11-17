package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/cert"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)



type CertAccountHandler struct {
	db      *gorm.DB
	log     *logrus.Logger
	certMgr *cert.Manager
}

func NewCertAccountHandler(db *gorm.DB, log *logrus.Logger, certMgr *cert.Manager) *CertAccountHandler {
	return &CertAccountHandler{db: db, log: log, certMgr: certMgr}
}

func (h *CertAccountHandler) List(c *gin.Context) {
	var accounts []model.CertAccount
	if err := h.db.Order("id desc").Find(&accounts).Error; err != nil {
		h.log.Errorf("获取证书账号失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "获取证书账号失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *CertAccountHandler) Create(c *gin.Context) {
	var account model.CertAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	logger.WriteLog("info", "cert", fmt.Sprintf("创建证书账号 [%d] %s", account.ID, account.Email))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *CertAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CertAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	if err := preserveEmptySecretFields(h.db, req.ID, &req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "证书账号不存在"})
		return
	}
	if err := h.db.Save(&req).Error; err != nil {
		h.log.Errorf("更新证书账号失败 [%d]: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新证书账号失败"})
		return
	}
	logger.WriteLog("info", "cert", fmt.Sprintf("更新证书账号 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CertAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CertAccount{}, id)
	logger.WriteLog("info", "cert", fmt.Sprintf("删除证书账号 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CertAccountHandler) Verify(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var account model.CertAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}
	if h.certMgr == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "证书服务未初始化"})
		return
	}
	if err := h.certMgr.VerifyAccount(account.Email, account.Type, account.EabKid, account.EabHmacKey.String()); err != nil {
		h.log.Warnf("[证书账号] 验证账号失败: id=%d type=%s email=%s err=%v", id, account.Type, account.Email, err)
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.log.Infof("[证书账号] 验证账号成功: id=%d type=%s email=%s", id, account.Type, account.Email)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "账号验证成功"})
}

// ===== 域名证书 =====

type CertHandler struct {
	db      *gorm.DB
	log     *logrus.Logger
	config  *config.Config
	certMgr *cert.Manager
}

func NewCertHandler(db *gorm.DB, log *logrus.Logger, cfg *config.Config, certMgr *cert.Manager) *CertHandler {
	return &CertHandler{db: db, log: log, config: cfg, certMgr: certMgr}
}

func (h *CertHandler) List(c *gin.Context) {
	var certs []model.DomainCert
	h.db.Order("id desc").Find(&certs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": maskSensitiveResponse(c, certs)})
}

func (h *CertHandler) Create(c *gin.Context) {
	var cert model.DomainCert
	if err := c.ShouldBindJSON(&cert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cert.Status = "pending"
	cert.AcmeStep = 0

	// 如果是 ACME 类型且使用 DNS 验证，校验域名是否在 DNS 域名解析中
	if cert.CertType != "manual" && cert.ChallengeType == "dns" && cert.DomainAccountID > 0 {
		var domains []string
		if err := json.Unmarshal([]byte(cert.Domains), &domains); err == nil && len(domains) > 0 {
			// 查询该 DNS 账号下的所有域名
			var domainInfos []model.DomainInfo
			h.db.Where("account_id = ?", cert.DomainAccountID).Find(&domainInfos)

			accountDomains := make(map[string]bool)
			for _, di := range domainInfos {
				accountDomains[strings.ToLower(di.Name)] = true
			}

			// 检查每个域名的根域名是否在 DNS 解析中
			var missingDomains []string
			for _, d := range domains {
				d = strings.TrimPrefix(d, "*.")
				rootDomain := extractRootDomain(d)
				if !accountDomains[strings.ToLower(rootDomain)] {
					missingDomains = append(missingDomains, rootDomain)
				}
			}

			// 如果有域名不在 DNS 解析中，且未指定手动模式，则自动设为手动模式
			if len(missingDomains) > 0 && cert.DnsMode != "manual" {
				cert.DnsMode = "manual"
			}
		}
	}

	// 如果没有选择 DNS 账号，自动设为手动模式
	if cert.CertType != "manual" && cert.ChallengeType == "dns" && cert.DomainAccountID == 0 {
		cert.DnsMode = "manual"
	}

	h.db.Create(&cert)
	logger.WriteLog("info", "cert", fmt.Sprintf("创建域名证书 [%d]", cert.ID))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cert, "message": "创建成功"})
}

// extractRootDomain 从域名中提取根域名
// 例如: www.example.com -> example.com, _acme-challenge.sub.example.com -> example.com
func extractRootDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func (h *CertHandler) Download(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var cert model.DomainCert
	if err := h.db.First(&cert, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "证书不存在"})
		return
	}
	certPEM, keyPEM := cert.CertContent, cert.KeyContent
	if cert.CertFile != "" && cert.KeyFile != "" {
		certBytes, certErr := os.ReadFile(filepath.Clean(cert.CertFile))
		keyBytes, keyErr := os.ReadFile(filepath.Clean(cert.KeyFile))
		if certErr == nil && keyErr == nil {
			certPEM, keyPEM = string(certBytes), string(keyBytes)
		} else if certPEM == "" || keyPEM == "" {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "证书文件不可用"})
			return
		}
	}
	if certPEM == "" || keyPEM == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "证书内容不可用"})
		return
	}
	var archiveBuffer bytes.Buffer
	zipWriter := zip.NewWriter(&archiveBuffer)
	for name, content := range map[string]string{"certificate.pem": certPEM, "private-key.pem": keyPEM} {
		entry, err := zipWriter.Create(name)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成证书归档失败"})
			return
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "写入证书归档失败"})
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "完成证书归档失败"})
		return
	}
	filename := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == '\r' || r == '\n' || r == '"' {
			return '_'
		}
		return r
	}, cert.Name)
	if filename == "" {
		filename = fmt.Sprintf("certificate-%d", id)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, filename))
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	logger.WriteLog("warn", "cert", fmt.Sprintf("用户 %s 下载证书及私钥 [%d]", c.GetString("username"), id))
	c.Data(http.StatusOK, "application/zip", archiveBuffer.Bytes())
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
	logger.WriteLog("info", "cert", fmt.Sprintf("更新域名证书 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CertHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainCert{}, id)
	logger.WriteLog("info", "cert", fmt.Sprintf("删除域名证书 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// Apply 一键申请证书（自动执行全部 ACME 流程）
func (h *CertHandler) Apply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.log.Infof("触发证书申请: %d", id)
	logger.WriteLog("info", "cert", fmt.Sprintf("触发证书申请 [%d]", id))

	go func() {
		if err := h.certMgr.StartApply(uint(id)); err != nil {
			h.log.Errorf("证书申请失败 [%d]: %v", id, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "证书申请任务已提交"})
}

// Renew 续期证书（兼容旧接口，等同于 Apply）
func (h *CertHandler) Renew(c *gin.Context) {
	h.Apply(c)
}

// GetStatus 获取证书 ACME 流程状态
func (h *CertHandler) GetStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	status := h.certMgr.GetStatus(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": status})
}

// StepCreateOrder 手动触发步骤1：创建订单
func (h *CertHandler) StepCreateOrder(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logger.WriteLog("info", "cert", fmt.Sprintf("手动触发创建订单 [%d]", id))

	go func() {
		if err := h.certMgr.StepCreateOrder(uint(id)); err != nil {
			h.log.Errorf("创建订单失败 [%d]: %v", id, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "创建订单任务已提交"})
}

// StepSetDNS 手动触发步骤2：设置 DNS
func (h *CertHandler) StepSetDNS(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logger.WriteLog("info", "cert", fmt.Sprintf("手动触发设置DNS [%d]", id))

	go func() {
		if err := h.certMgr.StepSetDNS(uint(id)); err != nil {
			h.log.Errorf("设置DNS失败 [%d]: %v", id, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "设置DNS任务已提交"})
}

// StepValidate 手动触发步骤3：提交验证
func (h *CertHandler) StepValidate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logger.WriteLog("info", "cert", fmt.Sprintf("手动触发提交验证 [%d]", id))

	go func() {
		if err := h.certMgr.StepValidate(uint(id)); err != nil {
			h.log.Errorf("提交验证失败 [%d]: %v", id, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "提交验证任务已提交"})
}

// StepObtain 手动触发步骤4：获取证书
func (h *CertHandler) StepObtain(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logger.WriteLog("info", "cert", fmt.Sprintf("手动触发获取证书 [%d]", id))

	go func() {
		if err := h.certMgr.StepObtain(uint(id)); err != nil {
			h.log.Errorf("获取证书失败 [%d]: %v", id, err)
		}
	}()

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "获取证书任务已提交"})
}

// ConfirmDNS 手动确认 DNS 已设置（手动模式下用户设置完 DNS 后调用）
func (h *CertHandler) ConfirmDNS(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	logger.WriteLog("info", "cert", fmt.Sprintf("手动确认DNS已设置 [%d]", id))

	if err := h.certMgr.ConfirmDNS(uint(id)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已确认 DNS 设置，即将提交验证"})
}

// ===== 域名解析 =====
