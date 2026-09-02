package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/frpmaster"
)

// newFrpMasterTestEnv 组装 Manager + Handler + 路由（管理面 + 节点控制面）。
func newFrpMasterTestEnv(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.FrpMasterNode{}, &model.SystemLog{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	h := NewFrpMasterHandler(db, nil, frpmaster.NewManager(db, nil))
	r := gin.New()
	r.POST("/api/v1/frpmaster/nodes", h.Create)
	r.GET("/api/v1/frpmaster/nodes", h.List)
	r.POST("/api/v1/frpmaster/agent/heartbeat", h.Heartbeat)
	r.POST("/api/v1/frpmaster/agent/status", h.ReportStatus)
	r.POST("/api/v1/frpmaster/agent/logs", h.ReportLogs)
	r.GET("/api/v1/frpmaster/agent/config", h.FetchConfig)
	return r
}

func fmRequest(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	r.ServeHTTP(w, req)
	return w
}

// TestFrpMasterAgentFlow 模拟远程节点完整接入流程：
// 管理面注册（拿一次性 token）→ 心跳（先 401 后 200）→ 拉取 frpc.toml → 状态上报 → 列表可见。
func TestFrpMasterAgentFlow(t *testing.T) {
	r := newFrpMasterTestEnv(t)

	// 1) 管理面注册节点，node_token 仅此一次返回
	w := fmRequest(r, http.MethodPost, "/api/v1/frpmaster/nodes",
		`{"name":"home","region":"cn-east","server_addr":"frp.example.com","server_port":7000,"frps_token":"tk"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("注册节点 code=%d body=%s", w.Code, w.Body.String())
	}
	var createResp struct {
		Code int `json:"code"`
		Data struct {
			ID        uint   `json:"id"`
			Name      string `json:"name"`
			NodeToken string `json:"node_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("解析注册响应失败: %v body=%s", err, w.Body.String())
	}
	if createResp.Data.ID == 0 || createResp.Data.NodeToken == "" {
		t.Fatalf("应返回节点 id 与一次性 token，得到 %+v", createResp.Data)
	}
	id := createResp.Data.ID
	token := createResp.Data.NodeToken

	// 2) 错误 token 心跳 → 401
	w = fmRequest(r, http.MethodPost, "/api/v1/frpmaster/agent/heartbeat",
		fmt.Sprintf(`{"node_id":%d,"token":"bad-token"}`, id))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("错误 token 心跳应 401，得到 code=%d", w.Code)
	}

	// 3) 正确 token 心跳 → 200
	w = fmRequest(r, http.MethodPost, "/api/v1/frpmaster/agent/heartbeat",
		fmt.Sprintf(`{"node_id":%d,"token":%q}`, id, token))
	if w.Code != http.StatusOK {
		t.Fatalf("正确 token 心跳应 200，得到 code=%d body=%s", w.Code, w.Body.String())
	}

	// 4) 拉取本节点 frpc.toml（查询参数认证）
	w = fmRequest(r, http.MethodGet,
		fmt.Sprintf("/api/v1/frpmaster/agent/config?node_id=%d&token=%s", id, token), "")
	if w.Code != http.StatusOK {
		t.Fatalf("拉取配置应 200，得到 code=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `serverAddr = "frp.example.com"`) ||
		!strings.Contains(w.Body.String(), `token = "tk"`) {
		t.Fatalf("配置内容不符:\n%s", w.Body.String())
	}

	// 5) 状态上报 → 200
	w = fmRequest(r, http.MethodPost, "/api/v1/frpmaster/agent/status",
		fmt.Sprintf(`{"node_id":%d,"token":%q,"tunnels":[{"name":"ssh","type":"tcp","status":"running"}]}`, id, token))
	if w.Code != http.StatusOK {
		t.Fatalf("状态上报应 200，得到 code=%d body=%s", w.Code, w.Body.String())
	}

	// 6) 管理面列表应看到节点（含状态）
	w = fmRequest(r, http.MethodGet, "/api/v1/frpmaster/nodes", "")
	if w.Code != http.StatusOK {
		t.Fatalf("节点列表应 200，得到 code=%d", w.Code)
	}
	var listResp struct {
		Code int `json:"code"`
		Data []struct {
			ID     uint   `json:"id"`
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("解析列表失败: %v body=%s", err, w.Body.String())
	}
	if len(listResp.Data) != 1 || listResp.Data[0].ID != id || listResp.Data[0].Status != frpmaster.StatusOnline {
		t.Fatalf("节点列表应包含心跳过的节点且状态 online，得到 %+v", listResp.Data)
	}
}

// TestFrpMasterAgentLogs 节点日志回传：错误 token 401；正确 token 返回 written。
func TestFrpMasterAgentLogs(t *testing.T) {
	r := newFrpMasterTestEnv(t)

	// 注册节点拿一次性 token。
	w := fmRequest(r, http.MethodPost, "/api/v1/frpmaster/nodes",
		`{"name":"home","server_addr":"1.2.3.4","server_port":7000}`)
	if w.Code != http.StatusOK {
		t.Fatalf("注册节点 code=%d body=%s", w.Code, w.Body.String())
	}
	var createResp struct {
		Data struct {
			ID        uint   `json:"id"`
			NodeToken string `json:"node_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("解析注册响应失败: %v", err)
	}
	id, token := createResp.Data.ID, createResp.Data.NodeToken

	// 错误 token → 401。
	w = fmRequest(r, http.MethodPost, "/api/v1/frpmaster/agent/logs",
		fmt.Sprintf(`{"node_id":%d,"token":"bad","logs":["x"]}`, id))
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("错误 token 应 401, code=%d", w.Code)
	}

	// 正确 token + 2 行 → 200 且 written=2（落库断言在 Manager 层测试覆盖）。
	w = fmRequest(r, http.MethodPost, "/api/v1/frpmaster/agent/logs",
		fmt.Sprintf(`{"node_id":%d,"token":%q,"logs":["line1","line2"]}`, id, token))
	if w.Code != http.StatusOK {
		t.Fatalf("日志回传应 200, code=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Written int `json:"written"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Written != 2 {
		t.Fatalf("written 应为 2, 得到 %d", resp.Written)
	}
}
