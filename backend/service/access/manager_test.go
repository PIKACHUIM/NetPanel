package access

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func openAccessTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "access.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

func TestResolveIPsCheckedFailsOnMalformedInputAndDatabaseError(t *testing.T) {
	db := openAccessTestDB(t)
	if err := db.AutoMigrate(&model.IPDBEntry{}); err != nil {
		t.Fatalf("migrate IPDB table: %v", err)
	}
	for _, rule := range []model.AccessRule{
		{BaseModel: model.BaseModel{ID: 1}, IPList: "not-json"},
		{BaseModel: model.BaseModel{ID: 2}, IPList: `[{"ip":"192.0.2.1"}]`},
		{BaseModel: model.BaseModel{ID: 3}, IPList: `["not-an-ip"]`},
	} {
		if _, err := ResolveIPsChecked(db, rule); err == nil {
			t.Errorf("ResolveIPsChecked(%+v) unexpectedly succeeded", rule)
		}
	}
	if err := db.Create(&model.IPDBEntry{CIDR: ""}).Error; err != nil {
		t.Fatalf("create empty IPDB entry: %v", err)
	}
	for _, rule := range []model.AccessRule{
		{BaseModel: model.BaseModel{ID: 4}, BindIPDBIDs: "[1]"},
		{BaseModel: model.BaseModel{ID: 5}, BindIPDBIDs: "[999]"},
	} {
		if _, err := ResolveIPsChecked(db, rule); err == nil {
			t.Errorf("ResolveIPsChecked(%+v) unexpectedly accepted empty or missing IPDB entries", rule)
		}
	}
	if err := db.Migrator().DropTable(&model.IPDBEntry{}); err != nil {
		t.Fatalf("drop IPDB table: %v", err)
	}
	if _, err := ResolveIPsChecked(db, model.AccessRule{BaseModel: model.BaseModel{ID: 6}, BindIPDBIDs: "[1]"}); err == nil {
		t.Fatal("IPDB query failure unexpectedly succeeded")
	}
}

func TestAccessMiddlewareFailsClosedWhenRulesCannotLoad(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAccessTestDB(t)
	manager := &Manager{db: db, log: logrus.New()}
	manager.loadRules()

	engine := gin.New()
	engine.Use(manager.GinMiddleware())
	engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("response status = %d, want %d when rules cannot load", recorder.Code, http.StatusServiceUnavailable)
	}
}

func TestAccessMiddlewareFailsClosedForInvalidSiteBindings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name          string
		bind          string
		mode          string
		authMode      string
		emptyIPs      bool
		dropSiteTable bool
	}{
		{name: "malformed json", bind: "not-json", mode: "whitelist"},
		{name: "missing site", bind: "[999]", mode: "whitelist"},
		{name: "site query failure", bind: "[1]", mode: "whitelist", dropSiteTable: true},
		{name: "unknown ip policy", mode: "unexpected"},
		{name: "unknown policy without IPs", mode: "unexpected", emptyIPs: true},
		{name: "empty whitelist", mode: "whitelist", emptyIPs: true},
		{name: "unknown auth mode", authMode: "unexpected"},
		{name: "unbound OIDC", authMode: "oidc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			db := openAccessTestDB(t)
			if err := db.AutoMigrate(&model.AccessRule{}, &model.CaddySite{}, &model.IPDBEntry{}); err != nil {
				t.Fatalf("migrate access tables: %v", err)
			}
			if test.dropSiteTable {
				if err := db.Migrator().DropTable(&model.CaddySite{}); err != nil {
					t.Fatalf("drop Caddy site table: %v", err)
				}
			}
			ipList := `["192.0.2.1"]`
			if test.emptyIPs {
				ipList = ""
			}
			rule := model.AccessRule{Enable: true, Mode: test.mode, IPList: ipList, BindSiteIDs: test.bind, AuthMode: test.authMode}
			if err := db.Create(&rule).Error; err != nil {
				t.Fatalf("create malformed site rule: %v", err)
			}
			manager := &Manager{db: db, log: logrus.New()}
			manager.loadRules()

			engine := gin.New()
			engine.Use(manager.GinMiddleware())
			engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "http://app.example.com/protected", nil))
			if recorder.Code != http.StatusServiceUnavailable {
				t.Fatalf("response status = %d, want %d for invalid site binding", recorder.Code, http.StatusServiceUnavailable)
			}
		})
	}
}

func TestAccessMiddlewareFailsClosedOnInvalidRule(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openAccessTestDB(t)
	if err := db.AutoMigrate(&model.AccessRule{}, &model.IPDBEntry{}); err != nil {
		t.Fatalf("migrate access tables: %v", err)
	}
	rule := model.AccessRule{Enable: true, Mode: "whitelist", IPList: "not-json"}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("create malformed access rule: %v", err)
	}
	manager := &Manager{db: db, log: logrus.New()}
	manager.loadRules()

	engine := gin.New()
	engine.Use(manager.GinMiddleware())
	engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("response status = %d, want %d for invalid access rule", recorder.Code, http.StatusServiceUnavailable)
	}
}
