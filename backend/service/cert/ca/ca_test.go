package ca

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.CACert{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestCAKeyPKCS8RoundTrip 是本 PR 两个 P0 的回归：
// 1) toPKCS8PrivKey 生成的 DER 必须能被 x509.ParsePKCS8PrivateKey 解析（原手工拼接实现解析失败）
// 2) CA 私钥经数据库存取后仍能用于签发节点证书（CAKeyPEM 改 Secret 加密存储后不能破坏可用性）
func TestCAKeyPKCS8RoundTrip(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	der := toPKCS8PrivKey(key)
	if der == nil {
		t.Fatal("toPKCS8PrivKey 返回 nil，MarshalPKCS8PrivateKey 失败")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		t.Fatalf("PKCS#8 解析失败（原 P0 的症状）: %v", err)
	}
	got, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		t.Fatalf("解析结果类型 = %T，期望 *ecdsa.PrivateKey", parsed)
	}
	if got.D.Cmp(key.D) != 0 {
		t.Fatal("往返后私钥 D 不一致")
	}

	// 落库 → 重新加载 → 仍可签发
	db := newTestDB(t)
	log := logrus.New()
	s := NewSigner(db, log)
	if err := s.generateNewCA(); err != nil {
		t.Fatalf("generateNewCA: %v", err)
	}
	certPEM, _, err := s.SignNodeCert("node-1", []string{"node-1.local"}, nil)
	if err != nil {
		t.Fatalf("SignNodeCert(内存 CA): %v", err)
	}
	if block, _ := pem.Decode(certPEM); block == nil {
		t.Fatal("签发的节点证书不是合法 PEM")
	}

	// 新 Signer 从数据库加载（触发 Scan 解密路径）
	s2 := NewSigner(db, log)
	if err := s2.InitOrLoad(); err != nil {
		t.Fatalf("InitOrLoad(从库加载 CA): %v", err)
	}
	if _, _, err := s2.SignNodeCert("node-2", []string{"node-2.local"}, nil); err != nil {
		t.Fatalf("从库加载的 CA 无法签发证书: %v", err)
	}
}
