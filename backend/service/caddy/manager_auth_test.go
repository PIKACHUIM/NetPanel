package caddy

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func newAuthRouteTestManager(t *testing.T) *Manager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "caddy-auth.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.OAuthProviderConfig{}, &model.User{}, &model.AccessRule{}, &model.WafConfig{}, &model.IPDBEntry{}); err != nil {
		t.Fatalf("migrate auth models: %v", err)
	}
	return &Manager{db: db, log: logrus.New(), panelPort: 1086}
}

func TestBuildOIDCRoutesFailsClosedWhenProviderUnavailable(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	routes := manager.buildOIDCRoutes(nil, []interface{}{map[string]interface{}{"handler": "reverse_proxy"}}, model.AccessRule{OidcProviderID: 999})
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want one fail-closed route", len(routes))
	}
	route := routes[0].(map[string]interface{})
	handlers := route["handle"].([]interface{})
	response := handlers[0].(map[string]interface{})
	if response["handler"] != "static_response" || response["status_code"] != "503" {
		t.Fatalf("missing provider route = %#v, want static 503", response)
	}
}

func TestBuildOIDCRoutesAlwaysValidatesSession(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	provider := model.OAuthProviderConfig{Name: "corp", Type: "oidc", Enable: true}
	if err := manager.db.Create(&provider).Error; err != nil {
		t.Fatalf("create OAuth provider: %v", err)
	}
	rule := model.AccessRule{OidcProviderID: provider.ID, AllowedUserIDs: "[2,5]"}
	routes := manager.buildOIDCRoutes([]interface{}{map[string]interface{}{"host": []string{"app.example.com"}}}, []interface{}{map[string]interface{}{"handler": "reverse_proxy"}}, rule)
	if len(routes) != 1 {
		t.Fatalf("routes = %d, want one authenticated route", len(routes))
	}
	route := routes[0].(map[string]interface{})
	handlers := route["handle"].([]interface{})
	auth := handlers[0].(map[string]interface{})
	if auth["handler"] != "netpanel_session_auth" || auth["oauth_provider"] != "corp" {
		t.Fatalf("first handler = %#v, want signed-session auth with provider redirect", auth)
	}
	if got := auth["allowed_user_ids"].([]uint); len(got) != 2 || got[0] != 2 || got[1] != 5 {
		t.Fatalf("allowed_user_ids = %#v, want [2 5]", auth["allowed_user_ids"])
	}
	if _, ok := route["match"]; !ok {
		t.Fatal("host matcher was not retained")
	}
}

func TestLoadRouteDepsFailsClosedForDuplicateSecurityRules(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	sites := []model.CaddySite{{BaseModel: model.BaseModel{ID: 7}, Domain: "app.example.com", Enable: true}}
	for _, rule := range []model.AccessRule{
		{Name: "first", Enable: true, AuthMode: "page_login", BindSiteIDs: "[7]"},
		{Name: "second", Enable: true, AuthMode: "basic_auth", BindSiteIDs: "[7]"},
	} {
		if err := manager.db.Create(&rule).Error; err != nil {
			t.Fatalf("create access rule: %v", err)
		}
	}
	deps := manager.loadRouteDeps(sites)
	if !deps.invalid[7] {
		t.Fatal("duplicate auth rules were not marked invalid")
	}
	routes, err := manager.buildRoutes(&sites[0], deps)
	if err != nil {
		t.Fatalf("build routes: %v", err)
	}
	response := routes[0].(map[string]interface{})["handle"].([]interface{})[0].(map[string]interface{})
	if response["handler"] != "static_response" || response["status_code"] != "503" {
		t.Fatalf("duplicate auth rules route = %#v, want static 503", response)
	}
}

func TestLoadRouteDepsFailsClosedForUnboundOIDCRule(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	sites := []model.CaddySite{{BaseModel: model.BaseModel{ID: 13}, Domain: "app.example.com", Enable: true}}
	rule := model.AccessRule{Name: "unbound-oidc", Enable: true, AuthMode: "oidc"}
	if err := manager.db.Create(&rule).Error; err != nil {
		t.Fatalf("create unbound OIDC rule: %v", err)
	}
	deps := manager.loadRouteDeps(sites)
	if !deps.invalid[13] {
		t.Fatal("unbound OIDC rule was not marked invalid for configured sites")
	}
}

func TestLoadRouteDepsFailsClosedForEmptyWhitelist(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	sites := []model.CaddySite{{BaseModel: model.BaseModel{ID: 17}, Domain: "app.example.com", Enable: true}}
	rule := model.AccessRule{Name: "empty-whitelist", Enable: true, Mode: "whitelist", BindSiteIDs: "[17]"}
	if err := manager.db.Create(&rule).Error; err != nil {
		t.Fatalf("create empty whitelist: %v", err)
	}
	deps := manager.loadRouteDeps(sites)
	if !deps.invalid[17] {
		t.Fatal("empty whitelist was not marked invalid")
	}
}

func TestLoadRouteDepsFailsClosedForDuplicateIPRules(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	sites := []model.CaddySite{{BaseModel: model.BaseModel{ID: 11}, Domain: "app.example.com", Enable: true}}
	for _, rule := range []model.AccessRule{
		{Name: "first-ip-rule", Enable: true, Mode: "blacklist", IPList: `["192.0.2.10"]`, BindSiteIDs: "[11]"},
		{Name: "second-ip-rule", Enable: true, Mode: "whitelist", IPList: `["192.0.2.20"]`, BindSiteIDs: "[11]"},
	} {
		if err := manager.db.Create(&rule).Error; err != nil {
			t.Fatalf("create IP access rule: %v", err)
		}
	}
	deps := manager.loadRouteDeps(sites)
	if !deps.invalid[11] {
		t.Fatal("duplicate IP rules were not marked invalid")
	}
}

func TestLoadRouteDepsFailsClosedWhenRuleQueryFails(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	if err := manager.db.Migrator().DropTable(&model.AccessRule{}); err != nil {
		t.Fatalf("drop test access-rule table: %v", err)
	}
	sites := []model.CaddySite{{BaseModel: model.BaseModel{ID: 9}, Domain: "app.example.com", Enable: true}}
	deps := manager.loadRouteDeps(sites)
	if !deps.invalid[9] {
		t.Fatal("access-rule query failure was not marked invalid")
	}
}

func TestHostPatternsExtractsCaddyHostMatchers(t *testing.T) {
	got := hostPatterns([]interface{}{map[string]interface{}{"host": []string{"app.example.com", "*.example.net"}}})
	if len(got) != 2 || got[0] != "app.example.com" || got[1] != "*.example.net" {
		t.Fatalf("hostPatterns = %v, want configured exact and wildcard hosts", got)
	}
}

func TestBuildBasicAuthFailsClosedWithoutEnabledAccounts(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	handler := manager.buildBasicAuthHandler(model.AccessRule{})
	if handler["handler"] != "static_response" || handler["status_code"] != "503" {
		t.Fatalf("empty Basic Auth handler = %#v, want static 503", handler)
	}
}

func TestBuildBasicAuthFailsClosedWhenUserQueryFails(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	if err := manager.db.Migrator().DropTable(&model.User{}); err != nil {
		t.Fatalf("drop user table: %v", err)
	}
	handler := manager.buildBasicAuthHandler(model.AccessRule{})
	if handler["handler"] != "static_response" || handler["status_code"] != "503" {
		t.Fatalf("Basic Auth handler after query error = %#v, want static 503", handler)
	}
}

func TestBuildOIDCRoutesDisabledProviderFailsClosed(t *testing.T) {
	manager := newAuthRouteTestManager(t)
	provider := model.OAuthProviderConfig{Name: "corp", Type: "oidc", Enable: false}
	if err := manager.db.Create(&provider).Error; err != nil {
		t.Fatalf("create disabled OAuth provider: %v", err)
	}
	if err := manager.db.Model(&provider).Update("enable", false).Error; err != nil {
		t.Fatalf("disable OAuth provider: %v", err)
	}
	routes := manager.buildOIDCRoutes(nil, []interface{}{map[string]interface{}{"handler": "reverse_proxy"}}, model.AccessRule{OidcProviderID: provider.ID})
	response := routes[0].(map[string]interface{})["handle"].([]interface{})[0].(map[string]interface{})
	if response["handler"] != "static_response" || response["status_code"] != "503" {
		t.Fatalf("disabled provider route = %#v, want static 503", response)
	}
}
