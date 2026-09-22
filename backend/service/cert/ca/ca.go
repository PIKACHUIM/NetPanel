package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Signer CA 签发器
type Signer struct {
	db      *gorm.DB
	log     *logrus.Logger
	caCert  *x509.Certificate
	caKey   *ecdsa.PrivateKey
	caChain []byte
}

// NewSigner 创建 CA 签发器
func NewSigner(db *gorm.DB, log *logrus.Logger) *Signer {
	return &Signer{db: db, log: log}
}

// InitOrLoad 初始化或从数据库加载 CA
func (s *Signer) InitOrLoad() error {
	var ca model.CACert
	if err := s.db.First(&ca).Error; err != nil {
		return s.generateNewCA()
	}
	return s.loadFromDB(&ca)
}

// generateNewCA 生成新的 CA 密钥对和证书
func (s *Signer) generateNewCA() error {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	s.caKey = caKey

	caCert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName:   "NetPanel CA",
			Organization: []string{"NetPanel"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caCert, caCert, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}

	certParsed, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return err
	}
	s.caCert = certParsed

	ca := model.CACert{
		CACertPEM: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})),
		CAKeyPEM:  model.Secret(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: toPKCS8PrivKey(caKey)})),
		ExpiresAt: caCert.NotAfter,
		Status:    "active",
	}
	return s.db.Create(&ca).Error
}

// loadFromDB 从数据库加载 CA
func (s *Signer) loadFromDB(ca *model.CACert) error {
	certBlock, _ := pem.Decode([]byte(ca.CACertPEM))
	if certBlock == nil {
		return fmt.Errorf("failed to decode CA certificate")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return err
	}
	s.caCert = cert

	keyBlock, _ := pem.Decode([]byte(ca.CAKeyPEM))
	if keyBlock == nil {
		return fmt.Errorf("failed to decode CA private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}
	caKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return fmt.Errorf("CA key is not ECDSA")
	}
	s.caKey = caKey

	s.caChain = []byte(ca.CACertPEM)
	return nil
}

// SignNodeCert 为节点签发证书
func (s *Signer) SignNodeCert(nodeID string, dnsNames []string, ipAddresses []net.IP) (certPEM, keyPEM []byte, err error) {
	nodeKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	nodeCert := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			CommonName:   nodeID,
			Organization: []string{"NetPanel Nodes"},
		},
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, nodeCert, s.caCert, &nodeKey.PublicKey, s.caKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: toPKCS8PrivKey(nodeKey)})
	return
}

// GetCACert 返回 CA 证书 PEM
func (s *Signer) GetCACert() []byte {
	return s.caChain
}

// toPKCS8PrivKey 转换 ECDSA 私钥为 PKCS#8 DER 格式。
// 原实现手工拼接 DER：补了公钥位串却把私钥写成裸整数，且长度字节写死，
// 生成的密钥无法被 x509.ParsePKCS8PrivateKey 解析（CA 完全不可用）。
func toPKCS8PrivKey(key *ecdsa.PrivateKey) []byte {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil
	}
	return der
}
