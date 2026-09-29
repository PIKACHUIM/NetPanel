package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
)

func TestGetCADirURL(t *testing.T) {
	cases := []struct {
		ca   string
		want string
	}{
		{"zerossl", "https://acme.zerossl.com/v2/DV90"},
		{"ZeroSSL", "https://acme.zerossl.com/v2/DV90"}, // 大小写不敏感
		{"buypass", "https://api.buypass.com/acme/directory"},
		{"google", "https://dv.acme-v02.api.pki.goog/directory"},
		{"letsencrypt", "https://acme-v02.api.letsencrypt.org/directory"},
		{"", "https://acme-v02.api.letsencrypt.org/directory"}, // 默认
		{"unknown-ca", "https://acme-v02.api.letsencrypt.org/directory"},
	}
	for _, c := range cases {
		if got := (&Manager{}).getCADirURL(c.ca); got != c.want {
			t.Errorf("getCADirURL(%q) = %q，期望 %q", c.ca, got, c.want)
		}
	}
}

func TestACMEUserGetters(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("生成测试密钥失败: %v", err)
	}
	u := &acmeUser{Email: "test@example.com", key: key}

	if u.GetEmail() != "test@example.com" {
		t.Errorf("GetEmail = %q", u.GetEmail())
	}
	if u.GetRegistration() != nil {
		t.Error("未注册时 GetRegistration 应为 nil")
	}
	if u.GetPrivateKey() == nil {
		t.Error("GetPrivateKey 不应为 nil")
	}
}
