package handlers

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/model"
	"gorm.io/gorm"
)

func TestValidateLoginRedirect(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "redirect.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.CaddySite{}, &model.AccessRule{}, &model.OAuthProviderConfig{}); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	site := model.CaddySite{Name: "Protected", Domain: "app.example.com", Port: 443, Enable: true}
	if err := db.Create(&site).Error; err != nil {
		t.Fatalf("create test site: %v", err)
	}
	rule := model.AccessRule{Name: "Login", Enable: true, AuthMode: "page_login", BindSiteIDs: fmt.Sprintf("[%d]", site.ID)}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("create access rule: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
		valid bool
	}{
		{name: "local path", input: "/dashboard?tab=overview", want: "/dashboard?tab=overview", valid: true},
		{name: "backslash authority", input: `/\attacker.example/path`},
		{name: "oversized local path", input: "/" + string(make([]byte, 2050))},
		{name: "configured protected site", input: "https://app.example.com/private?view=1", want: "https://app.example.com/private?view=1", valid: true},
		{name: "unconfigured external host", input: "https://attacker.example/path"},
		{name: "unconfigured port", input: "https://app.example.com:444/path"},
		{name: "userinfo", input: "https://user@app.example.com/path"},
		{name: "ip literal", input: "https://127.0.0.1/path"},
		{name: "unsupported scheme", input: "javascript:alert(1)"},
		{name: "oversized target", input: "https://app.example.com/" + string(make([]byte, 2050))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := validateLoginRedirect(db, test.input)
			if valid != test.valid || got != test.want {
				t.Errorf("validateLoginRedirect(%q) = (%q, %v), want (%q, %v)", test.input, got, valid, test.want, test.valid)
			}
		})
	}

	if err := db.Model(&rule).Update("auth_mode", "oidc").Error; err != nil {
		t.Fatalf("change rule to OIDC: %v", err)
	}
	if got, valid := validateLoginRedirect(db, "https://app.example.com/private"); !valid || got != "https://app.example.com/private" {
		t.Fatalf("redirect for OIDC fallback site = (%q, %v), want configured URL and true", got, valid)
	}
	provider := model.OAuthProviderConfig{Name: "corp", Type: "oidc", Enable: false}
	if err := db.Create(&provider).Error; err != nil {
		t.Fatalf("create OAuth provider: %v", err)
	}
	if err := db.Model(&provider).Update("enable", false).Error; err != nil {
		t.Fatalf("disable OAuth provider: %v", err)
	}
	if err := db.Model(&rule).Update("oidc_provider_id", provider.ID).Error; err != nil {
		t.Fatalf("bind disabled OAuth provider: %v", err)
	}
	if got, valid := validateLoginRedirect(db, "https://app.example.com/private"); valid || got != "" {
		t.Fatalf("redirect to site with disabled OIDC provider = (%q, %v), want empty and false", got, valid)
	}
	if err := db.Model(&provider).Update("enable", true).Error; err != nil {
		t.Fatalf("enable OAuth provider: %v", err)
	}
	if got, valid := validateLoginRedirect(db, "https://app.example.com/private"); !valid || got != "https://app.example.com/private" {
		t.Fatalf("redirect to site with enabled OIDC provider = (%q, %v), want configured URL and true", got, valid)
	}
	if err := db.Model(&site).Update("enable", false).Error; err != nil {
		t.Fatalf("disable protected site: %v", err)
	}
	if got, valid := validateLoginRedirect(db, "https://app.example.com/private"); valid || got != "" {
		t.Fatalf("redirect to disabled site = (%q, %v), want empty and false", got, valid)
	}
	if err := db.Model(&site).Updates(map[string]any{"enable": true, "domain": "*.example.com"}).Error; err != nil {
		t.Fatalf("configure wildcard protected site: %v", err)
	}
	if got, valid := validateLoginRedirect(db, "https://api.example.com/private"); !valid || got != "https://api.example.com/private" {
		t.Fatalf("single-label wildcard redirect = (%q, %v), want configured URL and true", got, valid)
	}
	if got, valid := validateLoginRedirect(db, "https://a.b.example.com/private"); valid || got != "" {
		t.Fatalf("multi-label wildcard redirect = (%q, %v), want empty and false", got, valid)
	}
}
