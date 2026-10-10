package enrollment

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/netpanel/netpanel/service/cert/ca"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Token 一次性enrollment token
type Token struct {
	ID          string    `json:"id"`
	NodeName    string    `json:"node_name"`
	DNSNames    []string  `json:"dns_names,omitempty"`    // 签发时锁定的 SAN，兑换时不可更改
	IPAddresses []net.IP  `json:"ip_addresses,omitempty"` // 同上
	ExpiresAt   time.Time `json:"expires_at"`
	Used        bool      `json:"used"`
	UsedAt      time.Time `json:"used_at,omitempty"`
}

// Manager enrollment管理器
type Manager struct {
	db     *gorm.DB
	log    *logrus.Logger
	ca     *ca.Signer
	tokens map[string]*Token // token字符串 -> Token
	mu     sync.RWMutex
}

// NewManager 创建enrollment管理器
func NewManager(db *gorm.DB, log *logrus.Logger, caSigner *ca.Signer) *Manager {
	return &Manager{
		db:     db,
		log:    log,
		ca:     caSigner,
		tokens: make(map[string]*Token),
	}
}

// GenerateToken 生成一次性enrollment token
func (m *Manager) GenerateToken(nodeName string, dnsNames []string, ipAddresses []net.IP) (*Token, error) {
	// 生成token ID
	tokenID := generateTokenID()

	token := &Token{
		ID:          tokenID,
		NodeName:    nodeName,
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	m.mu.Lock()
	m.tokens[tokenID] = token
	m.mu.Unlock()

	m.log.Infof("[enrollment] 生成token: %s (node: %s)", tokenID[:8], nodeName)
	return token, nil
}

// ExchangeCertificate 用token换取证书
func (m *Manager) ExchangeCertificate(tokenID string, dnsNames []string, ipAddresses []net.IP) (certPEM, keyPEM []byte, err error) {
	m.mu.RLock()
	token, exists := m.tokens[tokenID]
	m.mu.RUnlock()

	if !exists {
		return nil, nil, fmt.Errorf("invalid token")
	}
	if token.Used {
		return nil, nil, fmt.Errorf("token already used")
	}
	if time.Now().After(token.ExpiresAt) {
		return nil, nil, fmt.Errorf("token expired")
	}

	// 标记为已使用
	m.mu.Lock()
	token.Used = true
	token.UsedAt = time.Now()
	m.mu.Unlock()

	// 签发证书：SAN 使用签发 token 时锁定的值，不采信兑换请求方提交的值（P0：防止持 token 者伪造任意 SAN）
	certPEM, keyPEM, err = m.ca.SignNodeCert(token.NodeName, token.DNSNames, token.IPAddresses)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign certificate: %w", err)
	}

	m.log.Infof("[enrollment] 节点 %s 完成enrollment", token.NodeName)
	return certPEM, keyPEM, nil
}

// RevokeToken 吊销token
func (m *Manager) RevokeToken(tokenID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, tokenID)
	return nil
}

// ListTokens 列出所有token
func (m *Manager) ListTokens() []*Token {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tokens := make([]*Token, 0, len(m.tokens))
	for _, t := range m.tokens {
		tokens = append(tokens, t)
	}
	return tokens
}

// generateTokenID 生成随机 token ID（crypto/rand，128 位熵；P0 修复：原实现为确定性伪随机可预测）
func generateTokenID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败属系统级异常，直接 panic 而非退化为可预测 token
		panic(fmt.Errorf("crypto/rand 不可用，无法生成安全 token: %w", err))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// randomSerial 生成随机证书序列号（P1 修复：原实现用 time.Now().Unix()，可预测且同秒并发撞号）
func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}
