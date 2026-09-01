package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Registry 全局Prometheus注册表
var Registry *prometheus.Registry

func init() {
	Registry = prometheus.NewRegistry()
	// 注册Go运行时指标
	_ = Registry.Register(collectors.NewGoCollector())
	// 注册进程指标
	_ = Registry.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
}

// GetRegistry 获取注册表
func GetRegistry() *prometheus.Registry {
	return Registry
}
