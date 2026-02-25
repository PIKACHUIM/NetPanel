package model

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitDB 初始化数据库，自动迁移所有表
func InitDB(dataDir string) (*gorm.DB, error) {
	dbPath := filepath.Join(dataDir, "netpanel.db")

	// 确保目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // SQLite 单连接
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 启用 WAL 模式提升并发性能
	db.Exec("PRAGMA journal_mode=WAL")
	db.Exec("PRAGMA synchronous=NORMAL")
	db.Exec("PRAGMA foreign_keys=ON")

	// 自动迁移所有表
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}

	// 初始化默认数据
	initDefaultData(db)

	return db, nil
}

// autoMigrate 自动迁移所有模型
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&SystemConfig{},
		&PortForwardRule{},
		&StunRule{},
		&FrpcConfig{},
		&FrpcProxy{},
		&FrpsConfig{},
		&NpsServerConfig{},
		&NpsClientConfig{},
		&EasytierClient{},
		&EasytierServer{},
		&DDNSTask{},
		&CaddySite{},
		&WolDevice{},
		&DomainAccount{},
		&DomainCert{},
		&DomainRecord{},
		&DnsmasqConfig{},
		&DnsmasqRecord{},
		&CronTask{},
		&StorageConfig{},
		&IPDBEntry{},
		&AccessRule{},
		&CallbackAccount{},
		&CallbackTask{},
	)
}

// initDefaultData 初始化默认配置数据
func initDefaultData(db *gorm.DB) {
	var count int64
	db.Model(&SystemConfig{}).Count(&count)
	if count == 0 {
		db.Create(&SystemConfig{
			Key:   "admin_password",
			Value: "admin123", // 默认密码，首次登录后应修改
		})
		db.Create(&SystemConfig{
			Key:   "language",
			Value: "zh",
		})
		db.Create(&SystemConfig{
			Key:   "theme",
			Value: "light",
		})
	}
}
