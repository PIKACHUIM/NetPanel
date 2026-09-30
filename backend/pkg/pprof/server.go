// Package pprof 提供可选的 Go 运行时性能剖析端点。
//
// 仅当环境变量 NETPANEL_PPROF=1 时启用，且**只监听 127.0.0.1**，
// 与面板主 HTTP 服务（默认监听 0.0.0.0）完全分离，避免把堆栈、
// goroutine、命令行参数等敏感信息暴露到公网。
package pprof

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

// DefaultAddr 是 pprof 端点的默认回环监听地址。
const DefaultAddr = "127.0.0.1:6060"

// Server 是运行在独立回环端口上的 pprof HTTP 服务。
type Server struct {
	srv *http.Server
	log *logrus.Logger
}

// Enabled 报告 NETPANEL_PPROF 是否开启。
func Enabled() bool {
	return os.Getenv("NETPANEL_PPROF") == "1"
}

// Handler 返回挂载了全部标准 pprof 端点的 mux。
//
// 注意：这里不注册 /debug/pprof/dump 的自定义实现，避免拼装不完整的
// goroutine dump；完整 dump 用 /debug/pprof/goroutine?debug=2 即可。
func Handler() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
	mux.Handle("/debug/pprof/block", pprof.Handler("block"))
	mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
	mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
	mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
	mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
	return mux
}

// Start 在 addr 上启动 pprof 服务并立即返回。addr 为空时使用 DefaultAddr。
//
// 强制回环约束：若解析后的 host 不是回环地址，Start 会拒绝启动并返回错误。
func Start(addr string, log *logrus.Logger) (*Server, error) {
	if addr == "" {
		addr = os.Getenv("NETPANEL_PPROF_ADDR")
	}
	if addr == "" {
		addr = DefaultAddr
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return nil, fmt.Errorf("pprof: 拒绝在非回环地址 %s 上启动（仅允许 127.0.0.1/::1）", addr)
	}

	s := &Server{
		log: log,
		srv: &http.Server{
			Addr:              addr,
			Handler:           Handler(),
			ReadHeaderTimeout: 5 * time.Second,
		},
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	go func() {
		log.Infof("[pprof] 性能剖析端点已启用（NETPANEL_PPROF=1），仅回环可访问： http://%s/debug/pprof/", addr)
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Errorf("[pprof] 服务异常退出: %v", err)
		}
	}()

	return s, nil
}

// Shutdown 优雅关闭 pprof 服务。
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}
