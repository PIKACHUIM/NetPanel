package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/service/cftunnel"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/wireguard"
)

// newTestMonitorHandler 构造带内存数据库与各隧道 manager 的测试 Handler
func newTestMonitorHandler(t *testing.T) *MonitorHandler {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}

	log := logrus.New()
	dataDir := t.TempDir()

	return NewMonitorHandler(
		db,
		frp.NewManager(db, log),
		nps.NewManager(db, log, dataDir),
		easytier.NewManager(db, log, dataDir),
		cftunnel.NewManager(db, log, dataDir),
		wireguard.NewManager(db, log, dataDir),
	)
}

// TestQueryTunnelStatus_StoppedManager 未启动的隧道 manager 应映射为 disconnected
func TestQueryTunnelStatus_StoppedManager(t *testing.T) {
	h := newTestMonitorHandler(t)

	cases := []struct {
		name       string
		tunnelType string
		want       string
	}{
		{"frp", "frp", "disconnected"},
		{"nps", "nps", "disconnected"},
		{"easytier", "easytier", "disconnected"},
		{"cftunnel", "cftunnel", "disconnected"},
		{"wireguard", "wireguard", "disconnected"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.queryTunnelStatus(tc.tunnelType, 1)
			if got != tc.want {
				t.Fatalf("queryTunnelStatus(%q) = %q, want %q", tc.tunnelType, got, tc.want)
			}
		})
	}
}

// TestQueryTunnelStatus_UnknownType 未知隧道类型应返回 unknown
func TestQueryTunnelStatus_UnknownType(t *testing.T) {
	h := newTestMonitorHandler(t)

	if got := h.queryTunnelStatus("unknown-svc", 1); got != "unknown" {
		t.Fatalf("queryTunnelStatus(unknown-svc) = %q, want %q", got, "unknown")
	}
}

// TestSyncTunnelStatus_NotFound 不存在的绑定应返回 404
func TestSyncTunnelStatus_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := newTestMonitorHandler(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "99999"}}

	h.SyncTunnelStatus(c)

	if w.Code != http.StatusNotFound {
		t.Fatalf("SyncTunnelStatus 不存在绑定返回 %d, want %d", w.Code, http.StatusNotFound)
	}
}
