package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// CertMetrics 证书指标
type CertMetrics struct {
	DaysRemaining *prometheus.GaugeVec // 证书剩余天数
}

// NewCertMetrics 创建证书指标
func NewCertMetrics() *CertMetrics {
	return &CertMetrics{
		DaysRemaining: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "netpanel_cert_days_remaining",
				Help: "证书剩余有效天数",
			},
			[]string{"domain", "issuer"},
		),
	}
}

// Register 注册证书指标
func (m *CertMetrics) Register(registry *prometheus.Registry) error {
	return registry.Register(m.DaysRemaining)
}
