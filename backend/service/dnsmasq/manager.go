package dnsmasq

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/miekg/dns"
	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager DNS 解析服务管理器（纯 Go 实现，不依赖 dnsmasq 进程）
type Manager struct {
	db     *gorm.DB
	log    *logrus.Logger
	server *dns.Server
	mu     sync.Mutex
	cancel context.CancelFunc
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

func (m *Manager) StartAll() {
	var cfg model.DnsmasqConfig
	if err := m.db.Order("id desc").First(&cfg).Error; err == nil && cfg.Enable {
		if err := m.Start(); err != nil {
			m.log.Errorf("DNS 服务启动失败: %v", err)
		}
	}
}

func (m *Manager) StopAll() {
	m.Stop()
}

func (m *Manager) Start() error {
	m.Stop()

	var cfg model.DnsmasqConfig
	if err := m.db.Order("id desc").First(&cfg).Error; err != nil {
		return fmt.Errorf("DNS 配置不存在: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", cfg.ListenAddr, cfg.ListenPort)

	mux := dns.NewServeMux()
	mux.HandleFunc(".", m.handleDNS)

	server := &dns.Server{
		Addr:    addr,
		Net:     "udp",
		Handler: mux,
	}

	m.mu.Lock()
	m.server = server
	m.mu.Unlock()

	go func() {
		m.log.Infof("[DNS] 服务已启动，监听 %s", addr)
		if err := server.ListenAndServe(); err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				m.log.Errorf("[DNS] 服务错误: %v", err)
			}
		}
		m.db.Model(&model.DnsmasqConfig{}).Where("id = ?", cfg.ID).Update("status", "stopped")
	}()

	m.db.Model(&model.DnsmasqConfig{}).Where("id = ?", cfg.ID).Update("status", "running")
	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		m.server.Shutdown()
		m.server = nil
	}
}

func (m *Manager) Reload() {
	// DNS 记录变化时，无需重启，下次查询时自动读取最新记录
	m.log.Info("[DNS] 配置已重载")
}

func (m *Manager) GetStatus() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server != nil {
		return "running"
	}
	return "stopped"
}

// handleDNS 处理 DNS 查询
func (m *Manager) handleDNS(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = false

	for _, q := range r.Question {
		domain := strings.TrimSuffix(q.Name, ".")

		// 查找自定义记录
		var record model.DnsmasqRecord
		if err := m.db.Where("domain = ? AND enable = ?", domain, true).First(&record).Error; err == nil {
			switch q.Qtype {
			case dns.TypeA:
				ip := net.ParseIP(record.IP)
				if ip != nil && ip.To4() != nil {
					msg.Answer = append(msg.Answer, &dns.A{
						Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: 60},
						A:   ip.To4(),
					})
					w.WriteMsg(msg)
					return
				}
			case dns.TypeAAAA:
				ip := net.ParseIP(record.IP)
				if ip != nil && ip.To4() == nil {
					msg.Answer = append(msg.Answer, &dns.AAAA{
						Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
						AAAA: ip,
					})
					w.WriteMsg(msg)
					return
				}
			}
		}

		// 转发到上游 DNS
		m.forwardToUpstream(w, r)
		return
	}

	w.WriteMsg(msg)
}

// forwardToUpstream 转发到上游 DNS 服务器
func (m *Manager) forwardToUpstream(w dns.ResponseWriter, r *dns.Msg) {
	var cfg model.DnsmasqConfig
	m.db.Order("id desc").First(&cfg)

	var upstreams []string
	if cfg.UpstreamDNS != "" {
		// 优先尝试解析 JSON 数组格式 ["8.8.8.8","114.114.114.114"]
		if err := json.Unmarshal([]byte(cfg.UpstreamDNS), &upstreams); err != nil {
			// 降级：逗号分隔格式
			for _, u := range strings.Split(cfg.UpstreamDNS, ",") {
				u = strings.TrimSpace(u)
				if u != "" {
					upstreams = append(upstreams, u)
				}
			}
		}
	}
	if len(upstreams) == 0 {
		upstreams = []string{"8.8.8.8:53", "114.114.114.114:53"}
	}

	c := new(dns.Client)
	for _, upstream := range upstreams {
		if !strings.Contains(upstream, ":") {
			upstream += ":53"
		}
		resp, _, err := c.Exchange(r, upstream)
		if err == nil {
			// DNS64:纯 IPv6 客户端请求 AAAA 但上游无 AAAA 记录时,
			// 用 A 记录 + NAT64 前缀合成 AAAA,让 IPv6 客户端可访问 IPv4 服务
			if cfg.EnableDNS64 && len(r.Question) > 0 && r.Question[0].Qtype == dns.TypeAAAA && !hasAAAA(resp.Answer) {
				if aResp, _, aErr := c.Exchange(aQueryFromAAAA(r), upstream); aErr == nil {
					resp.Answer = append(resp.Answer, synthesizeAAAA(aResp.Answer, cfg.NAT64Prefix)...)
				}
			}
			resp.SetReply(r)
			w.WriteMsg(resp)
			return
		}
	}

	// 所有上游失败，返回 SERVFAIL
	msg := new(dns.Msg)
	msg.SetRcode(r, dns.RcodeServerFailure)
	w.WriteMsg(msg)
}

// aQueryFromAAAA 由 AAAA 查询构造同域名的 A 查询
func aQueryFromAAAA(r *dns.Msg) *dns.Msg {
	q := &dns.Msg{}
	q.SetQuestion(r.Question[0].Name, dns.TypeA)
	return q
}

// hasAAAA 判断应答中是否已有 AAAA 记录
func hasAAAA(answers []dns.RR) bool {
	for _, rr := range answers {
		if _, ok := rr.(*dns.AAAA); ok {
			return true
		}
	}
	return false
}

// synthesizeAAAA 用 A 记录与 NAT64 前缀合成 AAAA 记录
// 前缀默认 64:ff9b::/96,合成规则:前缀前 96 位 + IPv4 后 32 位
func synthesizeAAAA(aAnswers []dns.RR, prefix string) []dns.RR {
	// 解析前缀
	_, prefixNet, err := net.ParseCIDR(prefix)
	if err != nil {
		return nil
	}
	prefixBytes := prefixNet.IP.To16()
	if prefixBytes == nil {
		return nil
	}

	var out []dns.RR
	for _, rr := range aAnswers {
		a, ok := rr.(*dns.A)
		if !ok || a.A == nil {
			continue
		}
		ipv6 := make(net.IP, 16)
		// 前缀前 12 字节(96 位)
		copy(ipv6[:12], prefixBytes[:12])
		// IPv4 后 4 字节(32 位)
		copy(ipv6[12:], a.A.To4())
		out = append(out, &dns.AAAA{
			Hdr:  dns.RR_Header{Name: a.Hdr.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: 60},
			AAAA: ipv6,
		})
	}
	return out
}
