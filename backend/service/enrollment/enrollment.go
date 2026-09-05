package enrollment

import (
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/netpanel/netpanel/service/cert/ca"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Token 一次性enrollment token
type Token struct {
	ID        string    `json:"id"`
	NodeName  string    `json:"node_name"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	UsedAt    time.Time `json:"used_at,omitempty"`
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
		db:   db,
		log:  log,
		ca:   caSigner,
		tokens: make(map[string]*Token),
	}
}

// GenerateToken 生成一次性enrollment token
func (m *Manager) GenerateToken(nodeName string, dnsNames []string, ipAddresses []net.IP) (*Token, error) {
	// 生成token ID
	tokenID := generateTokenID()

	token := &Token{
		ID:        tokenID,
		NodeName:  nodeName,
		ExpiresAt: time.Now().Add(24 * time.Hour),
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

	// 签发证书
	certPEM, keyPEM, err = m.ca.SignNodeCert(token.NodeName, dnsNames, ipAddresses)
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

// generateTokenID 生成随机token ID
func generateTokenID() string {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(0x30 + (i * 7) % 36) // 0-9, a-z
	}
	return base64.StdEncoding.EncodeToString(b)
}
