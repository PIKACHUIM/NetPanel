package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/netpanel/netpanel/pkg/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&SystemConfig{}, &User{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

func TestMigrateLegacyAdminPassword(t *testing.T) {
	hash, err := utils.HashPassword("legacy-pass-123")
	if err != nil {
		t.Fatal(err)
	}
	db := newMigrationTestDB(t)
	db.Create(&SystemConfig{Key: "admin_password", Value: hash})

	migrateLegacyAdminPassword(db)

	var user User
	if err := db.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("应创建 admin 用户记录: %v", err)
	}
	if !user.IsAdmin || !user.Enable {
		t.Errorf("迁移用户应为启用的管理员: %+v", user)
	}
	if !utils.CheckPassword("legacy-pass-123", user.Password) {
		t.Error("迁移后口令应可正常登录")
	}
	var count int64
	db.Model(&SystemConfig{}).Where("key = ?", "admin_password").Count(&count)
	if count != 0 {
		t.Error("遗留 admin_password 键应被删除")
	}
}

func TestMigrateLegacyAdminPasswordPlaintext(t *testing.T) {
	db := newMigrationTestDB(t)
	db.Create(&SystemConfig{Key: "admin_password", Value: "old-plain-password"})

	migrateLegacyAdminPassword(db)

	var user User
	if err := db.Where("username = ?", "admin").First(&user).Error; err != nil {
		t.Fatalf("应创建 admin 用户记录: %v", err)
	}
	if !utils.CheckPassword("old-plain-password", user.Password) {
		t.Error("明文历史口令应现场哈希并保留语义")
	}
}

func TestMigrateLegacyAdminPasswordNoop(t *testing.T) {
	db := newMigrationTestDB(t)
	// 无遗留键：不应创建任何用户
	migrateLegacyAdminPassword(db)
	var count int64
	db.Model(&User{}).Count(&count)
	if count != 0 {
		t.Errorf("无遗留键时不应创建用户, got %d", count)
	}

	// 已有 admin 用户（正常升级路径）：只删键、不动用户
	hash, _ := utils.HashPassword("current-pass")
	db.Create(&User{Username: "admin", Password: hash, Enable: true, IsAdmin: true})
	db.Create(&SystemConfig{Key: "admin_password", Value: "stale-hash"})
	migrateLegacyAdminPassword(db)
	var users int64
	db.Model(&User{}).Count(&users)
	if users != 1 {
		t.Errorf("已有 admin 用户时不应重复创建, got %d", users)
	}
	var cfgCount int64
	db.Model(&SystemConfig{}).Where("key = ?", "admin_password").Count(&cfgCount)
	if cfgCount != 0 {
		t.Error("残留键仍应被清理")
	}
}
