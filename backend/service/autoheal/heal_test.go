package autoheal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func TestCheckCertWarningsHandlesMissingExpiry(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "autoheal.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get test database connection: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := db.AutoMigrate(&model.DomainCert{}); err != nil {
		t.Fatalf("migrate certificate model: %v", err)
	}

	expired := time.Now().Add(-24 * time.Hour)
	certs := []model.DomainCert{
		{Name: "manual-without-expiry", Domains: "manual.example"},
		{Name: "expired", Domains: "expired.example", ExpireAt: &expired},
	}
	if err := db.Create(&certs).Error; err != nil {
		t.Fatalf("create certificate fixtures: %v", err)
	}

	manager := &Manager{
		db:  db,
		log: logrus.New(),
		cfg: Config{CertWarnDays: 7},
	}
	manager.checkCertWarnings()
}
