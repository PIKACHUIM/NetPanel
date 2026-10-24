package cert

// IP 证书申请（规划 #72）：Let's Encrypt IP 地址证书（RFC 8738）
//
// 与域名证书（DNS-01 四步异步流程）不同，IP 证书只能用 http-01/tls-alpn-01 验证
// 且必须使用 shortlived profile（约 6 天有效期），因此走同步申请流程：
// 内置 HTTP-01 挑战服务（监听 80 端口）+ lego 高层 Obtain（自动生成不带 CN 的
// IP SAN CSR——LE 拒绝 CommonName 含 IP 的 CSR）。
//
// ACME 账号密钥复用 acmeFlowData 存储：首次申请注册并保存，续期时直接恢复，
// 避免每次续期新建账号。

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-acme/lego/v4/acme/api"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/http01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"

	"github.com/netpanel/netpanel/model"
)

// ipShortlivedProfile Let's Encrypt IP 证书强制使用的 profile
const ipShortlivedProfile = "shortlived"

// ipCertRenewDays IP 证书有效期仅约 6 天，续期阈值固定按 2 天算
const ipCertRenewDays = 2

// http01ListenPort HTTP-01 挑战监听端口，ACME 规范要求 80；
// 可用 NETPANEL_HTTP01_PORT 覆盖（仅测试/反代场景）。
func http01ListenPort() string {
	if p := os.Getenv("NETPANEL_HTTP01_PORT"); p != "" {
		return p
	}
	return "80"
}

// ApplyIPCert 同步申请/续期一张 IP 证书。证书记录的 Domains 字段保存 IP 列表
// （JSON 数组），CertType 为 "acme-ip"，ChallengeType 为 "http"。
func (m *Manager) ApplyIPCert(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cert model.DomainCert
	if err := m.db.Preload("CertAccount").First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}
	if cert.CertType != "acme-ip" {
		return fmt.Errorf("证书 %s 不是 IP 证书类型", cert.Name)
	}
	if cert.Status == "applying" {
		return fmt.Errorf("证书正在申请中")
	}

	var ips []string
	if err := json.Unmarshal([]byte(cert.Domains), &ips); err != nil || len(ips) == 0 {
		return m.setError(id, fmt.Errorf("IP 列表解析失败: %w", err))
	}
	for _, ip := range ips {
		if net.ParseIP(ip) == nil {
			return m.setError(id, fmt.Errorf("%s 不是合法的 IP 地址", ip))
		}
	}

	m.updateCert(id, map[string]interface{}{"status": "applying", "last_error": ""})
	m.log.Infof("[证书][%s] 开始申请 IP 证书: %v", cert.Name, ips)

	// 恢复或新建 ACME 账号密钥
	email, eabKid, eabHmacKey := m.getAccountInfo(&cert)
	var flow acmeFlowData
	_ = json.Unmarshal([]byte(cert.AcmeData), &flow)

	var user *acmeUser
	if flow.PrivateKeyPEM != "" && flow.RegistrationURI != "" {
		key, err := decodeECPrivateKey(flow.PrivateKeyPEM)
		if err == nil {
			user = &acmeUser{Email: email, key: key, Registration: &registration.Resource{URI: flow.RegistrationURI}}
			m.log.Infof("[证书][%s] 复用已注册的 ACME 账号", cert.Name)
		}
	}
	if user == nil {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return m.setError(id, fmt.Errorf("生成账号密钥失败: %w", err))
		}
		user = &acmeUser{Email: email, key: key}
	}

	config := lego.NewConfig(user)
	config.Certificate.KeyType = certcrypto.RSA2048
	config.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	config.CADirURL = m.getCADirURL(cert.CA)

	// 首次申请：注册账号（EAB CA 需要 EAB 凭据）
	if user.Registration == nil || user.Registration.URI == "" {
		core, err := api.New(config.HTTPClient, config.UserAgent, config.CADirURL, "", user.key)
		if err != nil {
			return m.setError(id, fmt.Errorf("创建 ACME 客户端失败: %w", err))
		}
		reg, err := m.registerAccount(core, user, cert.CA, eabKid, eabHmacKey)
		if err != nil {
			return m.setError(id, err)
		}
		user.Registration = reg
	}

	client, err := lego.NewClient(config)
	if err != nil {
		return m.setError(id, fmt.Errorf("创建 ACME 客户端失败: %w", err))
	}

	// HTTP-01 挑战：standalone 服务监听 80 端口，Present 启动、CleanUp 关闭
	provider := http01.NewProviderServer("", http01ListenPort())
	if err := client.Challenge.SetHTTP01Provider(provider); err != nil {
		return m.setError(id, fmt.Errorf("配置 HTTP-01 挑战失败: %w", err))
	}

	req := certificate.ObtainRequest{
		Domains: ips,
		Profile: ipShortlivedProfile,
		Bundle:  true,
	}
	res, err := client.Certificate.Obtain(req)
	if err != nil {
		return m.setError(id, fmt.Errorf("IP 证书申请失败: %w", err))
	}

	// 保存证书文件（与域名证书同目录布局，便于 Caddy/面板直接消费）
	certDir := filepath.Join(m.dataDir, "certs", strconv.FormatUint(uint64(id), 10))
	if err := os.MkdirAll(certDir, 0700); err != nil {
		return m.setError(id, fmt.Errorf("创建证书目录失败: %w", err))
	}
	certFile := filepath.Join(certDir, "cert.pem")
	keyFile := filepath.Join(certDir, "key.pem")
	if err := os.WriteFile(certFile, res.Certificate, 0600); err != nil {
		return m.setError(id, fmt.Errorf("保存证书文件失败: %w", err))
	}
	if err := os.WriteFile(keyFile, res.PrivateKey, 0600); err != nil {
		return m.setError(id, fmt.Errorf("保存私钥文件失败: %w", err))
	}

	expireAt, _ := parseCertExpiry(res.Certificate)

	// 账号密钥与注册 URI 落盘，续期复用
	userKey, _ := user.key.(*ecdsa.PrivateKey)
	keyPEM := ""
	if userKey != nil {
		keyPEM = encodeECPrivateKey(userKey)
	}
	if keyPEM != "" {
		flow.PrivateKeyPEM = keyPEM
		flow.RegistrationURI = user.Registration.URI
		flowDataJSON, _ := json.Marshal(flow)
		m.updateCert(id, map[string]interface{}{"acme_data": string(flowDataJSON)})
	}

	m.updateCert(id, map[string]interface{}{
		"cert_file":        certFile,
		"key_file":         keyFile,
		"status":           "valid",
		"acme_step":        4,
		"acme_next_action": nil,
		"last_error":       "",
		"expire_at":        expireAt,
	})

	m.log.Infof("[证书][%s] IP 证书申请成功，到期时间: %v（shortlived，约 6 天后自动续期）", cert.Name, expireAt)
	return nil
}
