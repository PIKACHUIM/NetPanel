package autoheal

import (
	"fmt"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/netpanel/netpanel/service/syslog"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ProbeResult 子进程探测结果
type ProbeResult struct {
	ID      uint
	Service string
	Enable  bool
	Running bool
}

// Config 自愈配置
type Config struct {
	CheckIntervalSec  int
	RestartMaxRetries int
	CertWarnDays      int
	ProbeFunc         func() []ProbeResult
	RestartFn         func(id uint, service string) error
}

// Manager 自愈管理器
type Manager struct {
	mu       sync.Mutex
	db       *gorm.DB
	log      *logrus.Logger
	syslog   *syslog.Manager
	callback *callback.Manager
	cfg      Config
	ticker   *time.Ticker
	stopCh   chan struct{}
	running  bool
}

// NewManager 创建自愈管理器
func NewManager(db *gorm.DB, log *logrus.Logger, syslogMgr *syslog.Manager, cbMgr *callback.Manager, cfg Config) *Manager {
	if cfg.CheckIntervalSec <= 0 {
		cfg.CheckIntervalSec = 30
	}
	if cfg.CertWarnDays <= 0 {
		cfg.CertWarnDays = 7
	}
	return &Manager{
		db:       db,
		log:      log,
		syslog:   syslogMgr,
		callback: cbMgr,
		cfg:      cfg,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动周期性自愈检查
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return
	}
	m.ticker = time.NewTicker(time.Duration(m.cfg.CheckIntervalSec) * time.Second)
	m.running = true
	go m.run()
	m.log.Infof("[autoheal] 自愈守护已启动（间隔 %ds）", m.cfg.CheckIntervalSec)
}

// Stop 停止自愈守护
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.stopCh)
	m.log.Infof("[autoheal] 自愈守护已停止")
}

func (m *Manager) run() {
	for {
		select {
		case <-m.ticker.C:
			m.tick()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) tick() {
	for _, r := range m.cfg.ProbeFunc() {
		m.probeOne(r)
	}
	m.checkCertWarnings()
}

func (m *Manager) probeOne(r ProbeResult) {
	if !r.Enable {
		return
	}
	if r.Running {
		return
	}
	m.log.Warnf("[autoheal] %s#%d 未运行，尝试重启", r.Service, r.ID)
	if m.cfg.RestartFn != nil {
		if err := m.cfg.RestartFn(r.ID, r.Service); err != nil {
			m.log.Errorf("[autoheal] %s#%d 重启失败: %v", r.Service, r.ID, err)
			m.logAudit(r.Service, r.ID, "HEAL_FAIL", fmt.Sprintf("重启失败: %v", err))
			return
		}
	}
	m.logAudit(r.Service, r.ID, "HEAL_OK", "已自动重启")
}

func (m *Manager) checkCertWarnings() {
	var certs []model.DomainCert
	if err := m.db.Find(&certs).Error; err != nil {
		return
	}
	for _, c := range certs {
		if c.ExpireAt.IsZero() {
			continue
		}
		days := int(time.Until(*c.ExpireAt).Hours() / 24)
		switch {
		case days <= 0:
			m.alertCert("expired", c.Name, c.Domains)
		case days <= m.cfg.CertWarnDays:
			m.alertCert(fmt.Sprintf("expiring_in_%d_days", days), c.Name, c.Domains)
		}
	}
}

func (m *Manager) alertCert(reason, name, domains string) {
	msg := fmt.Sprintf("证书告警 [%s] %s (%s)", reason, name, domains)
	m.log.Warnf("[autoheal] %s", msg)
	m.syslogWrite("warn", "autoheal", msg)
	if m.callback != nil {
		m.callback.Trigger(callback.TriggerEvent{
			Type: "cert_" + reason,
		})
	}
}

func (m *Manager) syslogWrite(level, service, msg string) {
	if m.syslog != nil {
		m.syslog.Write(level, service, msg)
	}
}

func (m *Manager) logAudit(service string, id uint, action, detail string) {
	if m.syslog != nil {
		msg := fmt.Sprintf("[autoheal] [%s] %s#%d %s", action, service, id, detail)
		m.syslog.Write("audit", "autoheal", msg)
	}
}

// === 接口类型定义 ===
type FrpClientProber interface{ GetClientStatus(id uint) string }
type CftunnelProber interface{ GetStatus(id uint) (map[string]interface{}, error) }
type PortforwardProber interface{ GetStatus(id uint) string }

// BuildProbeFunc 构造综合探活函数
func BuildProbeFunc(
	db *gorm.DB,
	frp FrpClientProber,
	cft CftunnelProber,
	pf PortforwardProber,
) func() []ProbeResult {
	return func() []ProbeResult {
		var out []ProbeResult

		var frpClients []model.FrpcConfig
		db.Where("enable = ?", true).Find(&frpClients)
		for _, c := range frpClients {
			running := false
			if frp != nil {
				running = frp.GetClientStatus(c.ID) == "running"
			}
			out = append(out, ProbeResult{ID: c.ID, Service: "frp_client", Enable: c.Enable, Running: running})
		}

		var tunnels []model.CftunnelConfig
		db.Where("enable = ?", true).Find(&tunnels)
		for _, t := range tunnels {
			running := false
			if cft != nil {
				if st, err := cft.GetStatus(t.ID); err == nil {
					if v, ok := st["running"].(bool); ok {
						running = v
					}
				}
			}
			out = append(out, ProbeResult{ID: t.ID, Service: "cftunnel", Enable: t.Enable, Running: running})
		}

		var rules []model.PortForwardRule
		db.Where("enable = ?", true).Find(&rules)
		for _, r := range rules {
			st := "stopped"
			if pf != nil {
				st = pf.GetStatus(r.ID)
			}
			out = append(out, ProbeResult{ID: r.ID, Service: "portforward", Enable: r.Enable, Running: st == "running"})
		}

		return out
	}
}
