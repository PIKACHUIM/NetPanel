package cert

import (
	"fmt"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager 域名证书管理器
type Manager struct {
	db  *gorm.DB
	log *logrus.Logger
	mu  sync.Mutex
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// StartAll 启动自动续期检查
func (m *Manager) StartAll() {
	go m.autoRenewLoop()
}

// autoRenewLoop 定期检查证书到期情况
func (m *Manager) autoRenewLoop() {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()

	// 启动时检查一次
	m.checkAndRenew()

	for range ticker.C {
		m.checkAndRenew()
	}
}

func (m *Manager) checkAndRenew() {
	var certs []model.DomainCert
	m.db.Where("auto_renew = ? AND enable = ?", true, true).Find(&certs)

	for _, c := range certs {
		if c.ExpireAt.IsZero() {
			continue
		}
		// 提前 30 天续期
		if time.Until(c.ExpireAt) < 30*24*time.Hour {
			m.log.Infof("[证书] 证书 [%s] 即将到期，开始自动续期", c.Name)
			if err := m.Apply(c.ID); err != nil {
				m.log.Errorf("[证书] 自动续期失败: %v", err)
			}
		}
	}
}

// Apply 申请/续期证书（ACME）
func (m *Manager) Apply(id uint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var cert model.DomainCert
	if err := m.db.First(&cert, id).Error; err != nil {
		return fmt.Errorf("证书配置不存在: %w", err)
	}

	m.log.Infof("[证书] 开始申请证书: %s", cert.Name)

	// TODO: 集成 lego/acme 库实现 ACME 证书申请
	// 1. 创建 ACME 客户端
	// 2. 根据 challengeType 选择 DNS-01 或 HTTP-01 验证
	// 3. 申请证书
	// 4. 保存证书文件路径和到期时间

	m.db.Model(&model.DomainCert{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "pending",
		"last_error": "ACME 证书申请待实现",
	})

	return fmt.Errorf("ACME 证书申请待实现")
}

// GetStatus 获取证书状态
func (m *Manager) GetStatus(id uint) string {
	var cert model.DomainCert
	if err := m.db.First(&cert, id).Error; err != nil {
		return "unknown"
	}
	if cert.ExpireAt.IsZero() {
		return "not_issued"
	}
	if time.Until(cert.ExpireAt) < 0 {
		return "expired"
	}
	if time.Until(cert.ExpireAt) < 30*24*time.Hour {
		return "expiring_soon"
	}
	return "valid"
}
