package model

import (
	"os"
	"testing"

	"github.com/netpanel/netpanel/pkg/crypto"
)

// TestSecretValueScanEncrypted 验证 Value/Scan 真的走了加解密：
// 写入数据库的值必须是密文，读回来必须还原为原文。
func TestSecretValueScanEncrypted(t *testing.T) {
	if err := crypto.InitKey(); err != nil {
		t.Setenv(crypto.EnvSecretKey, "0123456789abcdef0123456789abcdef")
		if err := crypto.InitKey(); err != nil {
			t.Fatalf("InitKey: %v", err)
		}
	}
	_ = os.Getenv(crypto.EnvSecretKey)

	const plain = "s3cr3t-password"
	enc, err := Secret(plain).Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	stored, ok := enc.(string)
	if !ok {
		t.Fatalf("Value 返回类型 = %T，期望 string", enc)
	}
	if stored == plain {
		t.Fatal("Value 未加密：明文直接落库，加密逻辑未生效")
	}

	var got Secret
	if err := got.Scan(stored); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.String() != plain {
		t.Fatalf("Scan 往返 = %q，期望 %q", got.String(), plain)
	}
}

// TestSecretScanLegacyPlaintext 兼容启用加密前的历史明文数据。
func TestSecretScanLegacyPlaintext(t *testing.T) {
	var s Secret
	if err := s.Scan("legacy-plain-value"); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if s.String() != "legacy-plain-value" {
		t.Fatalf("历史明文读取 = %q，期望原样返回", s.String())
	}
}

// TestSecretEmptyValue 空值不应产生密文。
func TestSecretEmptyValue(t *testing.T) {
	v, err := Secret("").Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if v != "" {
		t.Fatalf("空 Secret 落库 = %v，期望空字符串", v)
	}
}
