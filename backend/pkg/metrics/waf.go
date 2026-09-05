package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// WAFMetrics WAF指标
type WAFMetrics struct {
	Blocks *prometheus.CounterVec // WAF拦截计数
}

// NewWAFMetrics 创建WAF指标
func NewWAFMetrics() *WAFMetrics {
	return &WAFMetrics{
		Blocks: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "netpanel_waf_blocks_total",
				Help: "WAF拦截请求总数",
			},
			[]string{"rule", "action", "domain"},
		),
	}
}

// Register 注册WAF指标
func (m *WAFMetrics) Register(registry *prometheus.Registry) error {
	return registry.Register(m.Blocks)
}
