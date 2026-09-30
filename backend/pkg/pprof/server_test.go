package pprof

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func testLog() *logrus.Logger {
	l := logrus.New()
	l.SetOutput(&strings.Builder{})
	return l
}

func TestEnabledFollowsEnv(t *testing.T) {
	t.Setenv("NETPANEL_PPROF", "1")
	if !Enabled() {
		t.Fatal("NETPANEL_PPROF=1 时 Enabled 应为 true")
	}
	t.Setenv("NETPANEL_PPROF", "0")
	if Enabled() {
		t.Fatal("NETPANEL_PPROF=0 时 Enabled 应为 false")
	}
	t.Setenv("NETPANEL_PPROF", "")
	if Enabled() {
		t.Fatal("NETPANEL_PPROF 为空时 Enabled 应为 false")
	}
}

func TestStartRejectsNonLoopback(t *testing.T) {
	for _, addr := range []string{"0.0.0.0:6060", ":6060", "192.168.1.10:6060", "example.com:6060"} {
		srv, err := Start(addr, testLog())
		if err == nil {
			_ = srv.Shutdown(context.Background())
			t.Fatalf("地址 %s 应被拒绝（非回环）", addr)
		}
	}
}

func TestHandlerServesAllProfiles(t *testing.T) {
	mux := Handler()
	paths := []string{
		"/debug/pprof/",
		"/debug/pprof/cmdline",
		"/debug/pprof/heap",
		"/debug/pprof/goroutine",
		"/debug/pprof/allocs",
		"/debug/pprof/block",
		"/debug/pprof/mutex",
		"/debug/pprof/threadcreate",
	}
	for _, p := range paths {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s 期望 200，实际 %d", p, rec.Code)
		}
	}
}

func TestStartAndShutdownLoopback(t *testing.T) {
	srv, err := Start("127.0.0.1:0", testLog())
	if err != nil {
		t.Fatalf("回环启动失败: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Fatalf("关闭失败: %v", err)
	}
}
