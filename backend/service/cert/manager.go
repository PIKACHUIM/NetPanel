package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/acme"
	"github.com/go-acme/lego/v4/acme/api"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/ddns"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// acmeUser 实现 lego 的 registration.User 接口
type acmeUser struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *acmeUser) GetEmail() string                        { return u.Email }
func (u *acmeUser) GetRegistration() *registration.Resource { return u.Registration }
func (u *acmeUser) GetPrivateKey() crypto.PrivateKey        { return u.key }

// acmeFlowData ACME 流程内部数据（序列化为 JSON 存储在 DomainCert.AcmeData 中）
type acmeFlowData struct {
	// 账号私钥（PEM 编码）
	PrivateKeyPEM string `json:"private_key_pem"`
	// ACME 注册 URI
	RegistrationURI string `json:"registration_uri"`
	// 订单 URL
	OrderURL string `json:"order_url"`
	// 挑战信息：domain -> {token, keyAuth, challengeURL, fqdn, value}
	Challenges map[string]*challengeInfo `json:"challenges"`
}

type challengeInfo struct {
	Token        string `json:"token"`
	KeyAuth      string `json:"key_auth"`
	ChallengeURL string `json:"challenge_url"`
	// DNS-01 验证记录
	FQDN  string `json:"fqdn"`
	Value string `json:"value"`
}

// Manager 域名证书管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	dataDir string
	mu      sync.Mutex
}

func NewManager(db *gorm.DB, log *logrus.Logger, dataDir string) *Manager {
	return &Manager{db: db, log: log, dataDir: dataDir}
}

// StartAll 启动自动续期检查和 ACME 流程定时器
func (m *Manager) StartAll() {
	go m.autoRenewLoop()
	go m.acmeFlowLoop()
}

// autoRenewLoop 每 12 小时检查一次证书到期情况
func (m *Manager) autoRenewLoop() {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	// 启动时先检查一次
	m.checkAndRenew()

	for range ticker.C {
		m.checkAndRenew()
	}
}

// acmeFlowLoop 每 30 秒检查一次是否有待执行的 ACME 流程步骤
func (m *Manager) acmeFlowLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// 启动时先检查一次
	m.processAcmeFlowTasks()

	for range ticker.C {
		m.processAcmeFlowTasks()
	}
}

// processAcmeFlowTasks 处理所有待执行的 ACME 流程任务
func (m *Manager) processAcmeFlowTasks() {
	var certs []model.DomainCert
	now := time.Now()
	// 查找所有有待执行操作且时间已到的证书
	m.db.Where("acme_next_action IS NOT NULL AND acme_next_action <= ? AND status IN ?",
		now, []string{"order_created", "dns_set", "validating"}).Find(&certs)

	for _, c := range certs {
		switch c.Status {
		case "order_created":
			// 订单已创建，自动设置 DNS
			m.log.Infof("[证书][%s] 自动执行：设置 DNS 解析", c.Name)
			if err := m.StepSetDNS(c.ID); err != nil {
				m.log.Errorf("[证书][%s] 设置 DNS 失败: %v", c.Name, err)
			}
		case "dns_set":
			// DNS 已设置，自动提交验证
			m.log.Infof("[证书][%s] 自动执行：提交验证", c.Name)
			if err := m.StepValidate(c.ID); err != nil {
				m.log.Errorf("[证书][%s] 提交验证失败: %v", c.Name, err)
			}
		case "validating":
			// 验证中，自动获取证书
			m.log.Infof("[证书][%s] 自动执行：获取证书", c.Name)
			if err := m.StepObtain(c.ID); err != nil {
				m.log.Errorf("[证书][%s] 获取证书失败: %v", c.Name, err)
			}
		}
	}
}

// checkAndRenew 检查并自动续期即将到期的证书
func (m *Manager) checkAndRenew() {
	var certs []model.DomainCert
	m.db.Where("auto_renew = ? AND status = ? AND cert_type = ?", true, "valid", "acme").Find(&certs)

	for _, c := range certs {
		if c.ExpireAt == nil || c.ExpireAt.IsZero() {
			continue
		}
		renewDays := c.RenewBeforeDays
		if renewDays <= 0 {
			renewDays = 7
		}
		if time.Until(*c.ExpireAt) < time.Duration(renewDays)*24*time.Hour {
			m.log.Infof("[证书][%s] 即将到期（%s），提前 %d 天自动续期", c.Name, c.ExpireAt.Format("2006-01-02"), renewDays)
			if err := m.StartApply(c.ID); err != nil {
				m.log.Errorf("[证书][%s] 自动续期失败: %v", c.Name, err)
			}
		}
	}
}

// ===== ACME 分步流程 =====

// StartApply 开始申请证书（一键自动流程：创建订单 → 立即设置DNS → 2分钟后提交验证 → 获取证书）
func (m *Manager) StartApply(id uint) error {
	// 第一步：创建订单
	if err := m.StepCreateOrder(id); err != nil {
		return err
	}
	return nil
}

// StepCreateOrder 步骤1：创建 ACME 订单，获取挑战信息
func (m *Manager) StepCreateOrder(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cert model.DomainCert
	if err := m.db.Preload("DomainAccount").Preload("CertAccount").First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}

	if cert.CertType == "manual" {
		return fmt.Errorf("手动上传证书不支持 ACME 申请")
	}

	// 更新状态
	m.updateCert(id, map[string]interface{}{
		"status":     "applying",
		"acme_step":  1,
		"last_error": "",
	})

	m.log.Infof("[证书][%s] 步骤1：创建 ACME 订单，CA: %s", cert.Name, cert.CA)

	// 解析域名列表
	var domains []string
	if err := json.Unmarshal([]byte(cert.Domains), &domains); err != nil || len(domains) == 0 {
		return m.setError(id, fmt.Errorf("域名列表解析失败: %w", err))
	}

	// 生成账号私钥
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return m.setError(id, fmt.Errorf("生成私钥失败: %w", err))
	}

	// 获取邮箱和 EAB 信息
	email, eabKid, eabHmacKey := m.getAccountInfo(&cert)

	user := &acmeUser{
		Email: email,
		key:   privateKey,
	}

	// 配置 lego 客户端
	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.RSA2048
	config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	config.CADirURL = m.getCADirURL(cert.CA)

	// 创建 ACME 核心客户端
	core, err := api.New(config.HTTPClient, config.UserAgent, config.CADirURL, "", privateKey)
	if err != nil {
		return m.setError(id, fmt.Errorf("创建 ACME 客户端失败: %w", err))
	}

	// 注册账号
	reg, err := m.registerAccount(core, user, cert.CA, eabKid, eabHmacKey)
	if err != nil {
		return m.setError(id, fmt.Errorf("ACME 账号注册失败: %w", err))
	}
	user.Registration = reg

	// 创建订单
	order, err := core.Orders.New(domains)
	if err != nil {
		return m.setError(id, fmt.Errorf("创建 ACME 订单失败: %w", err))
	}

	// 获取挑战信息
	flowData := &acmeFlowData{
		PrivateKeyPEM:   encodeECPrivateKey(privateKey),
		RegistrationURI: reg.URI,
		OrderURL:        order.Location,
		Challenges:      make(map[string]*challengeInfo),
	}

	var dnsRecords []string
	var dnsValues []string

	for _, authzURL := range order.Authorizations {
		authz, err := core.Authorizations.Get(authzURL)
		if err != nil {
			return m.setError(id, fmt.Errorf("获取授权信息失败: %w", err))
		}

		domain := authz.Identifier.Value

		// 查找 DNS-01 挑战
		for _, chal := range authz.Challenges {
			if chal.Type == "dns-01" {
				keyAuth, err := core.GetKeyAuthorization(chal.Token)
				if err != nil {
					return m.setError(id, fmt.Errorf("获取 KeyAuthorization 失败: %w", err))
				}

				// 计算 DNS TXT 记录值
				dnsValue := dns01KeyAuthDigest(keyAuth)
				fqdn := fmt.Sprintf("_acme-challenge.%s", domain)

				flowData.Challenges[domain] = &challengeInfo{
					Token:        chal.Token,
					KeyAuth:      keyAuth,
					ChallengeURL: chal.URL,
					FQDN:         fqdn,
					Value:        dnsValue,
				}

				dnsRecords = append(dnsRecords, fqdn)
				dnsValues = append(dnsValues, dnsValue)
				break
			}
		}
	}

	if len(flowData.Challenges) == 0 {
		return m.setError(id, fmt.Errorf("未找到 DNS-01 挑战信息"))
	}

	// 序列化流程数据
	flowDataJSON, _ := json.Marshal(flowData)

	// 手动 DNS 模式：不自动执行设置 DNS，等待用户手动设置后确认
	var nextAction *time.Time
	if cert.DnsMode != "manual" {
		// 自动模式：立即执行设置 DNS
		t := time.Now().Add(5 * time.Second)
		nextAction = &t
	}

	updates := map[string]interface{}{
		"status":          "order_created",
		"acme_step":       1,
		"acme_data":       string(flowDataJSON),
		"acme_dns_record": strings.Join(dnsRecords, "\n"),
		"acme_dns_value":  strings.Join(dnsValues, "\n"),
		"last_error":      "",
	}
	if nextAction != nil {
		updates["acme_next_action"] = nextAction
	} else {
		updates["acme_next_action"] = nil
	}
	m.updateCert(id, updates)

	m.log.Infof("[证书][%s] 订单创建成功，需要设置 %d 条 DNS 记录", cert.Name, len(flowData.Challenges))
	return nil
}

// StepSetDNS 步骤2：设置 DNS 解析记录
func (m *Manager) StepSetDNS(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cert model.DomainCert
	if err := m.db.Preload("DomainAccount").First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}

	if cert.Status != "order_created" && cert.Status != "applying" {
		return fmt.Errorf("当前状态 %s 不允许设置 DNS", cert.Status)
	}

	// 手动 DNS 模式：跳过自动设置，直接更新状态为 dns_set（等待用户手动确认验证）
	if cert.DnsMode == "manual" {
		m.log.Infof("[证书][%s] 手动 DNS 模式：跳过自动设置，等待用户手动设置 DNS 记录", cert.Name)
		m.updateCert(id, map[string]interface{}{
			"status":           "dns_set",
			"acme_step":        2,
			"acme_next_action": nil,
			"last_error":       "",
		})
		return nil
	}

	m.log.Infof("[证书][%s] 步骤2：设置 DNS 解析记录", cert.Name)

	// 解析流程数据
	var flowData acmeFlowData
	if err := json.Unmarshal([]byte(cert.AcmeData), &flowData); err != nil {
		return m.setError(id, fmt.Errorf("解析 ACME 流程数据失败: %w", err))
	}

	// 获取 DNS 账号信息
	accessID, accessSecret, provider, err := m.getDNSCredentials(&cert)
	if err != nil {
		return m.setError(id, err)
	}

	// 使用 DDNS provider 设置 DNS TXT 记录
	for domain, chal := range flowData.Challenges {
		m.log.Infof("[证书][%s] 设置 DNS TXT 记录: %s -> %s", cert.Name, chal.FQDN, chal.Value)

		if err := m.setDNSTxtRecord(provider, accessID, accessSecret, domain, chal.Value); err != nil {
			return m.setError(id, fmt.Errorf("设置 DNS 记录失败 (%s): %w", domain, err))
		}
	}

	// 设置下一步操作时间（2 分钟后提交验证，等待 DNS 传播）
	nextAction := time.Now().Add(2 * time.Minute)

	m.updateCert(id, map[string]interface{}{
		"status":           "dns_set",
		"acme_step":        2,
		"acme_next_action": nextAction,
		"last_error":       "",
	})

	m.log.Infof("[证书][%s] DNS 记录设置完成，将在 2 分钟后提交验证", cert.Name)
	return nil
}

// StepValidate 步骤3：提交验证
func (m *Manager) StepValidate(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cert model.DomainCert
	if err := m.db.Preload("CertAccount").First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}

	if cert.Status != "dns_set" {
		return fmt.Errorf("当前状态 %s 不允许提交验证", cert.Status)
	}

	m.log.Infof("[证书][%s] 步骤3：提交验证", cert.Name)

	// 解析流程数据
	var flowData acmeFlowData
	if err := json.Unmarshal([]byte(cert.AcmeData), &flowData); err != nil {
		return m.setError(id, fmt.Errorf("解析 ACME 流程数据失败: %w", err))
	}

	// 恢复私钥
	privateKey, err := decodeECPrivateKey(flowData.PrivateKeyPEM)
	if err != nil {
		return m.setError(id, fmt.Errorf("恢复私钥失败: %w", err))
	}

	// 重建 ACME 客户端（使用已注册的账户 URL 作为 kid）
	email, _, _ := m.getAccountInfo(&cert)
	user := &acmeUser{Email: email, key: privateKey}

	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.RSA2048
	config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	config.CADirURL = m.getCADirURL(cert.CA)

	core, err := api.New(config.HTTPClient, config.UserAgent, config.CADirURL, flowData.RegistrationURI, privateKey)
	if err != nil {
		return m.setError(id, fmt.Errorf("创建 ACME 客户端失败: %w", err))
	}

	// 提交所有挑战验证
	for domain, chal := range flowData.Challenges {
		m.log.Infof("[证书][%s] 提交验证: %s", cert.Name, domain)

		// 通知 CA 验证挑战
		if _, err := core.Challenges.New(chal.ChallengeURL); err != nil {
			return m.setError(id, fmt.Errorf("提交验证失败 (%s): %w", domain, err))
		}
	}

	// 设置下一步操作时间（30 秒后获取证书）
	nextAction := time.Now().Add(30 * time.Second)

	m.updateCert(id, map[string]interface{}{
		"status":           "validating",
		"acme_step":        3,
		"acme_next_action": nextAction,
		"last_error":       "",
	})

	m.log.Infof("[证书][%s] 验证已提交，等待 CA 验证完成", cert.Name)
	return nil
}

// StepObtain 步骤4：获取证书
func (m *Manager) StepObtain(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cert model.DomainCert
	if err := m.db.Preload("CertAccount").Preload("DomainAccount").First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}

	if cert.Status != "validating" {
		return fmt.Errorf("当前状态 %s 不允许获取证书", cert.Status)
	}

	m.log.Infof("[证书][%s] 步骤4：获取证书", cert.Name)

	// 解析流程数据
	var flowData acmeFlowData
	if err := json.Unmarshal([]byte(cert.AcmeData), &flowData); err != nil {
		return m.setError(id, fmt.Errorf("解析 ACME 流程数据失败: %w", err))
	}

	// 恢复私钥
	privateKey, err := decodeECPrivateKey(flowData.PrivateKeyPEM)
	if err != nil {
		return m.setError(id, fmt.Errorf("恢复私钥失败: %w", err))
	}

	// 重建 ACME 客户端（使用已注册的账户 URL 作为 kid）
	email, _, _ := m.getAccountInfo(&cert)
	user := &acmeUser{Email: email, key: privateKey}

	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.RSA2048
	config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	config.CADirURL = m.getCADirURL(cert.CA)

	core, err := api.New(config.HTTPClient, config.UserAgent, config.CADirURL, flowData.RegistrationURI, privateKey)
	if err != nil {
		return m.setError(id, fmt.Errorf("创建 ACME 客户端失败: %w", err))
	}

	// 检查订单状态
	order, err := core.Orders.Get(flowData.OrderURL)
	if err != nil {
		return m.setError(id, fmt.Errorf("获取订单状态失败: %w", err))
	}

	// 如果订单还在 pending 状态，说明验证还没完成
	if order.Status == "pending" || order.Status == "processing" {
		// 重新安排 30 秒后再试
		nextAction := time.Now().Add(30 * time.Second)
		m.updateCert(id, map[string]interface{}{
			"acme_next_action": nextAction,
		})
		m.log.Infof("[证书][%s] 订单状态: %s，30 秒后重试", cert.Name, order.Status)
		return nil
	}

	if order.Status == "invalid" {
		// 检查授权详情获取失败原因
		errMsg := "订单验证失败"
		for _, authzURL := range order.Authorizations {
			authz, aErr := core.Authorizations.Get(authzURL)
			if aErr != nil {
				continue
			}
			if authz.Status == "invalid" {
				for _, chal := range authz.Challenges {
					if chal.Status == "invalid" && chal.Error != nil {
						errMsg = fmt.Sprintf("域名 %s 验证失败: %s", authz.Identifier.Value, chal.Error.Detail)
						break
					}
				}
			}
		}
		return m.setError(id, errors.New(errMsg))
	}

	if order.Status != "ready" && order.Status != "valid" {
		// 重新安排 30 秒后再试
		nextAction := time.Now().Add(30 * time.Second)
		m.updateCert(id, map[string]interface{}{
			"acme_next_action": nextAction,
		})
		m.log.Infof("[证书][%s] 订单状态: %s，30 秒后重试", cert.Name, order.Status)
		return nil
	}

	// 解析域名列表
	var domains []string
	if err := json.Unmarshal([]byte(cert.Domains), &domains); err != nil || len(domains) == 0 {
		return m.setError(id, fmt.Errorf("域名列表解析失败: %w", err))
	}

	// 使用 core API 直接 finalize 订单并获取证书
	certificates, err := m.finalizeOrder(core, order, domains)
	if err != nil {
		return m.setError(id, fmt.Errorf("获取证书失败: %w", err))
	}

	// 保存证书文件
	certDir := filepath.Join(m.dataDir, "certs", fmt.Sprintf("%d", id))
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return m.setError(id, fmt.Errorf("创建证书目录失败: %w", err))
	}

	certFile := filepath.Join(certDir, "cert.pem")
	keyFile := filepath.Join(certDir, "key.pem")

	if err := os.WriteFile(certFile, certificates.Certificate, 0600); err != nil {
		return m.setError(id, fmt.Errorf("保存证书文件失败: %w", err))
	}
	if err := os.WriteFile(keyFile, certificates.PrivateKey, 0600); err != nil {
		return m.setError(id, fmt.Errorf("保存私钥文件失败: %w", err))
	}

	// 解析证书到期时间
	expireAt, _ := parseCertExpiry(certificates.Certificate)

	// 清理 DNS TXT 记录
	go m.cleanupDNSRecords(&cert, &flowData)

	// 更新数据库
	m.updateCert(id, map[string]interface{}{
		"cert_file":        certFile,
		"key_file":         keyFile,
		"status":           "valid",
		"acme_step":        4,
		"acme_data":        "",
		"acme_dns_record":  "",
		"acme_dns_value":   "",
		"acme_next_action": nil,
		"last_error":       "",
		"expire_at":        expireAt,
	})

	m.log.Infof("[证书][%s] 证书获取成功，到期时间: %v", cert.Name, expireAt)
	return nil
}

// Apply 兼容旧接口：一键申请证书（自动执行全部流程）
func (m *Manager) Apply(id uint) error {
	return m.StartApply(id)
}

// GetStatus 获取证书状态
func (m *Manager) GetStatus(id uint) map[string]interface{} {
	var cert model.DomainCert
	if err := m.db.First(&cert, id).Error; err != nil {
		return map[string]interface{}{"status": "unknown"}
	}

	result := map[string]interface{}{
		"status":          cert.Status,
		"acme_step":       cert.AcmeStep,
		"acme_dns_record": cert.AcmeDnsRecord,
		"acme_dns_value":  cert.AcmeDnsValue,
		"last_error":      cert.LastError,
		"dns_mode":        cert.DnsMode,
	}

	if cert.AcmeNextAction != nil {
		result["acme_next_action"] = cert.AcmeNextAction
	}
	if cert.ExpireAt != nil {
		result["expire_at"] = cert.ExpireAt
	}

	return result
}

// ConfirmDNS 手动确认 DNS 已设置（手动模式下，用户设置完 DNS 后调用此方法触发验证）
func (m *Manager) ConfirmDNS(id uint) error {
	var cert model.DomainCert
	if err := m.db.First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}

	// 只有 order_created 或 dns_set 状态才允许确认
	if cert.Status != "order_created" && cert.Status != "dns_set" {
		return fmt.Errorf("当前状态 %s 不允许确认 DNS", cert.Status)
	}

	m.log.Infof("[证书][%s] 用户确认 DNS 已设置，将在 10 秒后提交验证", cert.Name)

	// 更新状态为 dns_set，并设置下一步操作时间（10 秒后提交验证）
	nextAction := time.Now().Add(10 * time.Second)
	m.updateCert(id, map[string]interface{}{
		"status":           "dns_set",
		"acme_step":        2,
		"acme_next_action": nextAction,
		"last_error":       "",
	})

	return nil
}

// ===== 辅助方法 =====

// getAccountInfo 获取邮箱和 EAB 信息
func (m *Manager) getAccountInfo(cert *model.DomainCert) (email, eabKid, eabHmacKey string) {
	if cert.CertAccountID > 0 && cert.CertAccount.ID > 0 {
		email = cert.CertAccount.Email
		eabKid = cert.CertAccount.EabKid
		eabHmacKey = cert.CertAccount.EabHmacKey
	}
	if email == "" {
		email = "admin@netpanel.local"
	}
	return
}

// getCADirURL 获取 CA 目录 URL
func (m *Manager) getCADirURL(ca string) string {
	switch strings.ToLower(ca) {
	case "zerossl":
		return "https://acme.zerossl.com/v2/DV90"
	case "buypass":
		return "https://api.buypass.com/acme/directory"
	case "google":
		return "https://dv.acme-v02.api.pki.goog/directory"
	default:
		return lego.LEDirectoryProduction
	}
}

// registerAccount 注册 ACME 账号（直接使用 core.Accounts API，确保 kid 自动设置到同一个 core 上）
func (m *Manager) registerAccount(core *api.Core, user *acmeUser, ca, eabKid, eabHmacKey string) (*registration.Resource, error) {
	needEAB := strings.ToLower(ca) == "zerossl" || strings.ToLower(ca) == "google"

	if needEAB {
		if eabKid == "" || eabHmacKey == "" {
			return nil, fmt.Errorf("CA %s 需要 EAB 凭据，请在证书账号中配置 EAB Key ID 和 HMAC Key", ca)
		}
	}

	accMsg := acme.Account{
		TermsOfServiceAgreed: true,
		Contact:              []string{"mailto:" + user.Email},
	}

	var account acme.ExtendedAccount
	var err error

	if needEAB {
		account, err = core.Accounts.NewEAB(accMsg, eabKid, eabHmacKey)
	} else {
		account, err = core.Accounts.New(accMsg)
	}
	if err != nil {
		return nil, fmt.Errorf("注册 ACME 账号失败: %w", err)
	}

	return &registration.Resource{
		URI:  account.Location,
		Body: account.Account,
	}, nil
}

// getDNSCredentials 获取 DNS 验证凭据
func (m *Manager) getDNSCredentials(cert *model.DomainCert) (accessID, accessSecret, provider string, err error) {
	if cert.DomainAccountID > 0 {
		var account model.DomainAccount
		if dbErr := m.db.First(&account, cert.DomainAccountID).Error; dbErr == nil {
			return account.AccessID, account.AccessSecret, account.Provider, nil
		}
	}
	return "", "", "", fmt.Errorf("未配置 DNS 账号，请关联域名账号")
}

// setDNSTxtRecord 使用 DDNS provider 设置 DNS TXT 记录
func (m *Manager) setDNSTxtRecord(providerName, accessID, accessSecret, domain, value string) error {
	provider := ddns.NewProvider(providerName, accessID, accessSecret)
	if provider == nil {
		return fmt.Errorf("不支持的 DNS 服务商: %s", providerName)
	}

	// 使用 DDNS provider 的 UpdateRecord 方法设置 TXT 记录
	// _acme-challenge.domain -> TXT -> value
	acmeDomain := fmt.Sprintf("_acme-challenge.%s", domain)
	return provider.UpdateRecord(acmeDomain, "TXT", value, "60")
}

// cleanupDNSRecords 清理 ACME DNS 验证记录（真正删除 TXT 记录）
func (m *Manager) cleanupDNSRecords(cert *model.DomainCert, flowData *acmeFlowData) {
	// 手动 DNS 模式下不自动清理（用户自行管理）
	if cert.DnsMode == "manual" {
		m.log.Infof("[证书][%s] 手动 DNS 模式：跳过自动清理 DNS 记录", cert.Name)
		return
	}

	accessID, accessSecret, providerName, err := m.getDNSCredentials(cert)
	if err != nil {
		m.log.Warnf("[证书][%s] 清理 DNS 记录失败（无法获取凭据）: %v", cert.Name, err)
		return
	}

	provider := ddns.NewProvider(providerName, accessID, accessSecret)
	if provider == nil {
		m.log.Warnf("[证书][%s] 清理 DNS 记录失败（不支持的服务商: %s）", cert.Name, providerName)
		return
	}

	for domain := range flowData.Challenges {
		acmeDomain := fmt.Sprintf("_acme-challenge.%s", domain)
		m.log.Infof("[证书][%s] 删除 DNS TXT 记录: %s", cert.Name, acmeDomain)
		if err := provider.DeleteRecord(acmeDomain, "TXT"); err != nil {
			m.log.Warnf("[证书][%s] 删除 DNS TXT 记录失败 (%s): %v", cert.Name, acmeDomain, err)
		}
	}
}

// finalizeOrder 使用 core API 完成订单并获取证书
func (m *Manager) finalizeOrder(core *api.Core, order acme.ExtendedOrder, domains []string) (*certificate.Resource, error) {
	// 生成证书私钥（RSA 2048）
	certKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成证书私钥失败: %w", err)
	}

	// 创建 CSR
	csr, err := generateCSR(certKey, domains)
	if err != nil {
		return nil, fmt.Errorf("生成 CSR 失败: %w", err)
	}

	// Finalize 订单
	orderResp, err := core.Orders.UpdateForCSR(order.Finalize, csr)
	if err != nil {
		return nil, fmt.Errorf("finalize 订单失败: %w", err)
	}

	// 等待订单完成
	for i := 0; i < 10; i++ {
		if orderResp.Status == "valid" {
			break
		}
		if orderResp.Status == "invalid" {
			return nil, fmt.Errorf("订单状态无效: %s", orderResp.Status)
		}
		time.Sleep(3 * time.Second)
		orderResp, err = core.Orders.Get(order.Location)
		if err != nil {
			return nil, fmt.Errorf("获取订单状态失败: %w", err)
		}
	}

	if orderResp.Certificate == "" {
		return nil, fmt.Errorf("订单完成但未返回证书 URL")
	}

	// 下载证书（返回值：证书PEM、issuer PEM、error）
	certData, _, err := core.Certificates.Get(orderResp.Certificate, true)
	if err != nil {
		return nil, fmt.Errorf("下载证书失败: %w", err)
	}

	// 编码私钥为 PEM
	keyDER, err := x509.MarshalECPrivateKey(certKey)
	if err != nil {
		return nil, fmt.Errorf("编码私钥失败: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &certificate.Resource{
		Domain:      domains[0],
		Certificate: certData,
		PrivateKey:  keyPEM,
	}, nil
}

// generateCSR 生成证书签名请求
func generateCSR(privateKey *ecdsa.PrivateKey, domains []string) ([]byte, error) {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: domains[0],
		},
	}
	if len(domains) > 1 {
		template.DNSNames = domains
	} else {
		template.DNSNames = domains
	}
	return x509.CreateCertificateRequest(rand.Reader, template, privateKey)
}

// updateCert 更新证书记录
func (m *Manager) updateCert(id uint, updates map[string]interface{}) {
	m.db.Model(&model.DomainCert{}).Where("id = ?", id).Updates(updates)
}

// setError 设置错误状态并返回错误
func (m *Manager) setError(id uint, err error) error {
	m.updateCert(id, map[string]interface{}{
		"status":           "error",
		"last_error":       err.Error(),
		"acme_next_action": nil,
	})
	return err
}

// ===== 工具函数 =====

// dns01KeyAuthDigest 计算 DNS-01 验证的 TXT 记录值
// 参考 RFC 8555: base64url(SHA-256(keyAuthorization))
func dns01KeyAuthDigest(keyAuth string) string {
	hash := sha256.Sum256([]byte(keyAuth))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

// encodeECPrivateKey 将 ECDSA 私钥编码为 PEM 字符串
func encodeECPrivateKey(key *ecdsa.PrivateKey) string {
	derBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return ""
	}
	block := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: derBytes,
	}
	return string(pem.EncodeToMemory(block))
}

// decodeECPrivateKey 从 PEM 字符串解码 ECDSA 私钥
func decodeECPrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("无法解析 PEM 数据")
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// parseCertExpiry 从 PEM 证书中解析到期时间
func parseCertExpiry(certPEM []byte) (*time.Time, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("无法解析 PEM 数据")
	}
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("解析 X.509 证书失败: %w", err)
	}
	expiry := x509Cert.NotAfter
	return &expiry, nil
}

// noopDNSProvider 空 DNS provider（DNS 记录已手动设置时使用）
type noopDNSProvider struct{}

func (p *noopDNSProvider) Present(domain, token, keyAuth string) error { return nil }
func (p *noopDNSProvider) CleanUp(domain, token, keyAuth string) error { return nil }
func (p *noopDNSProvider) Timeout() (timeout, interval time.Duration)  { return 1 * time.Second, 1 * time.Second }