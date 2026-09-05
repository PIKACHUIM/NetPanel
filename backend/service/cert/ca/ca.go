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
		CAKeyPEM:  string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: toPKCS8PrivKey(caKey)})),
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

// toPKCS8PrivKey 转换 ECDSA 私钥为 PKCS#8 格式
func toPKCS8PrivKey(key *ecdsa.PrivateKey) []byte {
	keyBytes := elliptic.Marshal(key.Curve, key.X, key.Y)
	prefix := []byte{0x30, 0x82, 0x1, 0x2, 0x2, 0x1, 0x1, 0x0, 0xA0, 0x81, 0x0, 0x30, 0x13, 0x6, 0x7, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x2, 0x1, 0x6, 0x8, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x3, 0x1, 0x7, 0x3, 0x82, 0x1, 0x4, 0x0}
	content := append(prefix, keyBytes...)
	content = append(content, []byte{0x2, 0x1, 0x1}...)
	content = append(content, key.D.Bytes()...)
	return content
}
