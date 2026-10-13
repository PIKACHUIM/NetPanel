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

	// 历史遗留表数据迁移
	migrateLegacyCloudflareTunnels(db)
	migrateLegacyUserRoles(db)

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
		&CftunnelConfig{},
		&TunService{},
		&ProbeHistory{},
		&NpsTunnel{},
		&DDNSTask{},
		&DDNSHistory{},
		&CaddySite{},
		&WolDevice{},
		&DomainAccount{},
		&DomainInfo{},
		&CertAccount{},
		&DomainCert{},
		&DomainRecord{},
		&DnsmasqConfig{},
		&DnsmasqRecord{},
		&CronTask{},
		&StorageConfig{},
		&IPDBEntry{},
		&AccessRule{},
		&WafConfig{},
		&WafLog{},
		&WafBan{},
		&FirewallRule{},
		&CallbackAccount{},
		&CallbackTask{},
		&SystemLog{},
		&User{},
		&OAuthProviderConfig{},
		&IPDBSubscription{},
		&WireguardConfig{},
		&WireguardPeer{},
		&MeshNode{},
		&MeshNodeEvent{},
		// AI 管理模块
		&AiProvider{},
		&AiConversation{},
		&AiMessage{},
		&AiAssistant{},
		&AiCronTask{},
		&AiCronLog{},
		&AiPlugin{},
		// 服务监控模块
		&MonitorServer{},
		&MonitorMetric{},
		&MonitorProbe{},
		&MonitorProbeResult{},
		&MonitorTask{},
		&MonitorTaskLog{},
		&MonitorAlert{},
		&MonitorAlertRecord{},
		&MonitorNotificationChannel{},
		&MonitorDDNSBinding{},
		&MonitorTunnelBinding{},
	)
}

// migrateLegacyCloudflareTunnels 将历史 cloudflare_tunnels 表的数据迁移到
// cftunnel_configs（统一模型），完成后删除旧表。
//
// 背景：PR #40 曾并行引入 CloudflareTunnel 模型，与既有 CftunnelConfig 形成
// 两张表，且 handler 与 Manager 分别读写不同表，导致按 ID 启动时取到错误配置。
// 现统一到 CftunnelConfig，此处负责搬运存量数据，避免用户配置丢失。
func migrateLegacyCloudflareTunnels(db *gorm.DB) {
	if !db.Migrator().HasTable("cloudflare_tunnels") {
		return
	}

	// 旧表结构（字段名与 CftunnelConfig 不同：type→mode、public_url→quick_url）
	type legacyTunnel struct {
		ID         uint
		Name       string
		Type       string
		Enable     bool
		LocalURL   string `gorm:"column:local_url"`
		Token      string
		TunnelName string `gorm:"column:tunnel_name"`
		Hostname   string
		Status     string
		PublicURL  string `gorm:"column:public_url"`
		LastError  string `gorm:"column:last_error"`
		Remark     string
		CreatedAt  time.Time
	}

	var legacy []legacyTunnel
	if err := db.Table("cloudflare_tunnels").Find(&legacy).Error; err != nil {
		// 读取失败时保留旧表，避免误删数据
		return
	}

	for _, l := range legacy {
		mode := l.Type
		switch mode {
		case "quick", "named", "token":
		case "":
			mode = "quick"
		default:
			mode = "quick"
		}
		// named 模式若仅提供了 token，实际应按 token 模式运行
		if mode == "named" && l.Token != "" && l.TunnelName == "" {
			mode = "token"
		}

		remark := l.Remark
		if l.Hostname != "" {
			// Hostname 在统一模型中无对应字段，并入备注避免信息丢失
			if remark != "" {
				remark += " | "
			}
			remark += "hostname: " + l.Hostname
		}

		// 以名称去重，避免重复执行迁移时产生重复记录
		var exists int64
		db.Model(&CftunnelConfig{}).Where("name = ?", l.Name).Count(&exists)
		if exists > 0 {
			continue
		}

		cfg := CftunnelConfig{
			Name:       l.Name,
			Mode:       mode,
			Enable:     l.Enable,
			LocalURL:   l.LocalURL,
			TunnelName: l.TunnelName,
			Token:      l.Token,
			Protocol:   "http",
			QuickURL:   l.PublicURL,
			Status:     "stopped",
			LastError:  l.LastError,
			Remark:     remark,
		}
		db.Create(&cfg)
	}

	// 数据已搬运完成，删除旧表
	if err := db.Migrator().DropTable("cloudflare_tunnels"); err == nil {
		fmt.Printf("已迁移 %d 条 CF 隧道配置至 cftunnel_configs 并移除旧表\n", len(legacy))
	}

	// 测速弹窗开关：确保键存在（默认开启；设为 false 时 Speedtest API 返回 403）
	db.Where(&SystemConfig{Key: "speedtest_popup_enabled"}).
		FirstOrCreate(&SystemConfig{Key: "speedtest_popup_enabled", Value: "true"})
}

// initDefaultData 初始化默认配置数据。
//
// 安全说明：此处**不再创建任何默认管理员账号或默认密码**。
// 原实现会写入明文密码 "admin" 的 SystemConfig 与 admin 用户，导致：
//  1. 任何新部署实例都存在 admin/admin 弱口令后门；
//  2. 首次初始化向导（/api/v1/init/status）因检测到已存在用户而被跳过，
//     强制设置管理员密码的机制形同虚设。
//
// 现改为仅初始化与安全无关的界面偏好项，管理员账号一律由初始化向导创建。
func initDefaultData(db *gorm.DB) {
	ensureConfig := func(key, value string) {
		var count int64
		db.Model(&SystemConfig{}).Where("key = ?", key).Count(&count)
		if count == 0 {
			db.Create(&SystemConfig{Key: key, Value: value})
		}
	}
	ensureConfig("language", "zh")
	ensureConfig("theme", "light")
}

// migrateLegacyUserRoles 把旧 IsAdmin 字段语义迁移到新的 Roles 列。
// 首次升级：roles 列已存在但全为空时，根据 is_admin 回填 "admin" 或 "viewer"。
// 幂等：roles 非空时跳过。
func migrateLegacyUserRoles(db *gorm.DB) {
	var changed int64
	result := db.Model(&User{}).Where("roles = '' OR roles IS NULL").Updates(map[string]interface{}{
		"roles": gorm.Expr("CASE WHEN is_admin THEN ? ELSE ? END", RoleAdmin, RoleViewer),
	})
	if result.Error != nil {
		return
	}
	changed = result.RowsAffected
	if changed == 0 {
		return
	}
	db.Exec("UPDATE users SET is_admin = roles LIKE '%admin%'")
}
