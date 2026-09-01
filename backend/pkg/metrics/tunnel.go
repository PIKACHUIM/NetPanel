package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

// TunnelMetrics 穿透服务指标
type TunnelMetrics struct {
	ActiveTunnels *prometheus.GaugeVec      // 活跃隧道数
	Connections   *prometheus.CounterVec    // 连接计数
	Latency       *prometheus.HistogramVec  // 延迟分布
}

// NewTunnelMetrics 创建穿透服务指标
func NewTunnelMetrics() *TunnelMetrics {
	return &TunnelMetrics{
		ActiveTunnels: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "netpanel_tunnels_active",
				Help: "当前活跃的穿透隧道数量",
			},
			[]string{"tool", "name"},
		),
		Connections: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "netpanel_connections_total",
				Help: "累计连接数",
			},
			[]string{"tool", "direction"},
		),
		Latency: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "netpanel_tunnel_latency_seconds",
				Help:    "隧道延迟分布",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"tool", "line_id"},
		),
	}
}

// Register 注册所有指标
func (m *TunnelMetrics) Register(registry *prometheus.Registry) error {
	if err := registry.Register(m.ActiveTunnels); err != nil {
		return err
	}
	if err := registry.Register(m.Connections); err != nil {
		return err
	}
	if err := registry.Register(m.Latency); err != nil {
		return err
	}
	return nil
}
