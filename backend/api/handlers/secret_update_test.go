package handlers

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/crypto"
	"gorm.io/gorm"
)

func TestPreserveEmptySecretFields(t *testing.T) {
	t.Setenv(crypto.EnvSecretKey, "0123456789abcdef0123456789abcdef")
	if err := crypto.InitKey(); err != nil {
		t.Fatalf("initialize test encryption key: %v", err)
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "secrets.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.AiProvider{}, &model.CertAccount{}); err != nil {
		t.Fatalf("migrate secret-bearing models: %v", err)
	}

	stored := model.AiProvider{Name: "provider", BaseURL: "https://example.invalid", ApiKey: model.Secret("old-secret")}
	if err := db.Create(&stored).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	update := model.AiProvider{BaseModel: model.BaseModel{ID: stored.ID}, Name: "provider-renamed"}
	if err := preserveEmptySecretFields(db, stored.ID, &update); err != nil {
		t.Fatalf("preserve masked secret: %v", err)
	}
	if update.ApiKey.String() != "old-secret" {
		t.Fatalf("preserved API key = %q, want old secret", update.ApiKey.String())
	}

	update.ApiKey = model.Secret("new-secret")
	if err := preserveEmptySecretFields(db, stored.ID, &update); err != nil {
		t.Fatalf("preserve explicit replacement: %v", err)
	}
	if update.ApiKey.String() != "new-secret" {
		t.Fatalf("explicit replacement changed to %q", update.ApiKey.String())
	}

	account := model.CertAccount{Name: "acme", Type: "zerossl", EabHmacKey: model.Secret("old-eab-secret")}
	if err := db.Create(&account).Error; err != nil {
		t.Fatalf("create cert account: %v", err)
	}
	accountUpdate := model.CertAccount{BaseModel: model.BaseModel{ID: account.ID}, Name: "acme-renamed", Type: "zerossl"}
	if err := preserveEmptySecretFields(db, account.ID, &accountUpdate); err != nil {
		t.Fatalf("preserve EAB secret: %v", err)
	}
	if accountUpdate.EabHmacKey.String() != "old-eab-secret" {
		t.Fatalf("preserved EAB key = %q, want old secret", accountUpdate.EabHmacKey.String())
	}
}
