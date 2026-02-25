package cert

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/dns/alidns"
	"github.com/go-acme/lego/v4/providers/dns/cloudflare"
	"github.com/go-acme/lego/v4/providers/dns/dnspod"
	"github.com/go-acme/lego/v4/registration"
	"github.com/netpanel/netpanel/model"
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

// StartAll 启动自动续期检查
func (m *Manager) StartAll() {
	go m.autoRenewLoop()
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

// checkAndRenew 检查并自动续期即将到期的证书
func (m *Manager) checkAndRenew() {
	var certs []model.DomainCert
	m.db.Where("auto_renew = ? AND status = ?", true, "valid").Find(&certs)

	for _, c := range certs {
		if c.ExpireAt == nil || c.ExpireAt.IsZero() {
			continue
		}
		// 提前 30 天续期
		if time.Until(*c.ExpireAt) < 30*24*time.Hour {
			m.log.Infof("[证书][%s] 即将到期（%s），开始自动续期", c.Name, c.ExpireAt.Format("2006-01-02"))
			if err := m.Apply(c.ID); err != nil {
				m.log.Errorf("[证书][%s] 自动续期失败: %v", c.Name, err)
			}
		}
	}
}

// Apply 申请/续期证书（ACME DNS-01）
func (m *Manager) Apply(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cert model.DomainCert
	if err := m.db.Preload("DomainAccount").First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}

	// 更新状态为申请中
	m.db.Model(&model.DomainCert{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "applying",
		"last_error": "",
	})

	m.log.Infof("[证书][%s] 开始申请证书，CA: %s，验证方式: %s", cert.Name, cert.CA, cert.ChallengeType)

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

	email := cert.Email
	if email == "" {
		email = "admin@netpanel.local"
	}

	user := &acmeUser{
		Email: email,
		key:   privateKey,
	}

	// 配置 lego 客户端
	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.RSA2048
	config.HTTPClient = &http.Client{Timeout: 30 * time.Second}

	// 选择 CA
	switch strings.ToLower(cert.CA) {
	case "zerossl":
		config.CADirURL = "https://acme.zerossl.com/v2/DV90"
	case "buypass":
		config.CADirURL = "https://api.buypass.com/acme/directory"
	default: // letsencrypt
		config.CADirURL = lego.LEDirectoryProduction
	}

	client, err := lego.NewClient(config)
	if err != nil {
		return m.setError(id, fmt.Errorf("创建 ACME 客户端失败: %w", err))
	}

	// 配置 DNS 验证
	if cert.ChallengeType == "dns" || cert.ChallengeType == "" {
		// 获取 DNS 账号信息
		accessID, accessSecret, provider, err := m.getDNSCredentials(&cert)
		if err != nil {
			return m.setError(id, err)
		}

		dnsProvider, err := m.createDNSProvider(provider, accessID, accessSecret)
		if err != nil {
			return m.setError(id, err)
		}

		if err := client.Challenge.SetDNS01Provider(dnsProvider,
			dns01.AddRecursiveNameservers([]string{"8.8.8.8:53", "1.1.1.1:53"}),
		); err != nil {
			return m.setError(id, fmt.Errorf("设置 DNS 验证失败: %w", err))
		}
	} else {
		// HTTP-01 验证（需要 80 端口）
		httpProvider := http01.NewProviderServer("", "80")
		if err := client.Challenge.SetHTTP01Provider(httpProvider); err != nil {
			return m.setError(id, fmt.Errorf("设置 HTTP 验证失败: %w", err))
		}
	}

	// 注册账号
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		return m.setError(id, fmt.Errorf("ACME 账号注册失败: %w", err))
	}
	user.Registration = reg

	// 申请证书
	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		return m.setError(id, fmt.Errorf("证书申请失败: %w", err))
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
	expireAt, err := parseCertExpiry(certificates.Certificate)
	if err != nil {
		m.log.Warnf("[证书][%s] 解析到期时间失败: %v", cert.Name, err)
	}

	// 更新数据库
	updates := map[string]interface{}{
		"cert_file":  certFile,
		"key_file":   keyFile,
		"status":     "valid",
		"last_error": "",
	}
	if expireAt != nil {
		updates["expire_at"] = expireAt
	}
	m.db.Model(&model.DomainCert{}).Where("id = ?", id).Updates(updates)

	m.log.Infof("[证书][%s] 证书申请成功，到期时间: %v", cert.Name, expireAt)
	return nil
}

// GetStatus 获取证书状态
func (m *Manager) GetStatus(id uint) string {
	var cert model.DomainCert
	if err := m.db.First(&cert, id).Error; err != nil {
		return "unknown"
	}
	if cert.Status == "applying" {
		return "applying"
	}
	if cert.ExpireAt == nil || cert.ExpireAt.IsZero() {
		return "not_issued"
	}
	if time.Until(*cert.ExpireAt) < 0 {
		return "expired"
	}
	if time.Until(*cert.ExpireAt) < 30*24*time.Hour {
		return "expiring_soon"
	}
	return "valid"
}

// setError 设置错误状态并返回错误
func (m *Manager) setError(id uint, err error) error {
	m.db.Model(&model.DomainCert{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "error",
		"last_error": err.Error(),
	})
	return err
}

// getDNSCredentials 获取 DNS 验证凭据
func (m *Manager) getDNSCredentials(cert *model.DomainCert) (accessID, accessSecret, provider string, err error) {
	// 优先使用关联的域名账号
	if cert.DomainAccountID > 0 {
		var account model.DomainAccount
		if dbErr := m.db.First(&account, cert.DomainAccountID).Error; dbErr == nil {
			return account.AccessID, account.AccessSecret, account.Provider, nil
		}
	}
	return "", "", "", fmt.Errorf("未配置 DNS 账号，请关联域名账号")
}

// dnsProvider DNS 验证 provider 接口
type dnsProvider interface {
	Present(domain, token, keyAuth string) error
	CleanUp(domain, token, keyAuth string) error
}

// createDNSProvider 根据服务商名称创建 lego DNS provider
func (m *Manager) createDNSProvider(providerName, accessID, accessSecret string) (dnsProvider, error) {
	switch strings.ToLower(providerName) {
	case "alidns", "aliyun":
		os.Setenv("ALICLOUD_ACCESS_KEY", accessID)
		os.Setenv("ALICLOUD_SECRET_KEY", accessSecret)
		p, err := alidns.NewDNSProvider()
		if err != nil {
			return nil, fmt.Errorf("创建阿里云 DNS provider 失败: %w", err)
		}
		return p, nil

	case "cloudflare", "cf":
		os.Setenv("CF_DNS_API_TOKEN", accessSecret)
		p, err := cloudflare.NewDNSProvider()
		if err != nil {
			return nil, fmt.Errorf("创建 Cloudflare DNS provider 失败: %w", err)
		}
		return p, nil

	case "dnspod":
		os.Setenv("DNSPOD_API_KEY", accessID+","+accessSecret)
		p, err := dnspod.NewDNSProvider()
		if err != nil {
			return nil, fmt.Errorf("创建 DNSPod DNS provider 失败: %w", err)
		}
		return p, nil

	default:
		return nil, fmt.Errorf("不支持的 DNS 服务商: %s（支持: alidns/cloudflare/dnspod）", providerName)
	}
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