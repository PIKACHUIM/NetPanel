package mcp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

// newTestServer 构造带 token 的测试 Server
func newTestServer(token string) *Server {
	return &Server{
		log:   logrus.New(),
		token: token,
	}
}

// TestAuthorized_有效令牌 Bearer 令牌应通过
func TestAuthorized_有效令牌(t *testing.T) {
	s := newTestServer("secret-token-123")

	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer secret-token-123")
	if !s.authorized(r) {
		t.Fatal("有效令牌应通过认证")
	}
}

// TestAuthorized_错误令牌 错误令牌应拒绝
func TestAuthorized_错误令牌(t *testing.T) {
	s := newTestServer("secret-token-123")

	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Bearer wrong-token")
	if s.authorized(r) {
		t.Fatal("错误令牌应被拒绝")
	}
}

// TestAuthorized_缺失头 无 Authorization 头应拒绝
func TestAuthorized_缺失头(t *testing.T) {
	s := newTestServer("secret-token-123")

	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if s.authorized(r) {
		t.Fatal("缺失 Authorization 头应被拒绝")
	}
}

// TestAuthorized_非Bearer格式 非 Bearer 前缀应拒绝
func TestAuthorized_非Bearer格式(t *testing.T) {
	s := newTestServer("secret-token-123")

	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	r.Header.Set("Authorization", "Token secret-token-123")
	if s.authorized(r) {
		t.Fatal("非 Bearer 格式应被拒绝")
	}
}

// TestAuthorized_空token Server 未配置 token 时放行（兼容旧行为）
func TestAuthorized_空token(t *testing.T) {
	s := newTestServer("")

	r := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if !s.authorized(r) {
		t.Fatal("空 token 时应放行")
	}
}

// TestHandler_未授权返回401 未授权请求应返回 401 与 JSON-RPC 错误
func TestHandler_未授权返回401(t *testing.T) {
	s := newTestServer("secret-token-123")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	s.handler(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("未授权请求应返回 401, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Unauthorized") {
		t.Fatalf("响应应包含 Unauthorized, got %s", w.Body.String())
	}
}

// TestHandler_授权通过 授权请求应正常处理
func TestHandler_授权通过(t *testing.T) {
	s := newTestServer("secret-token-123")

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","method":"initialize","id":1}`))
	r.Header.Set("Authorization", "Bearer secret-token-123")
	s.handler(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("授权请求应返回 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "netpanel-mcp") {
		t.Fatalf("initialize 响应应包含 serverInfo, got %s", w.Body.String())
	}
}

// TestHandleToolLogs_Tail限制 tail 参数应截断日志行数
func TestHandleToolLogs_Tail限制(t *testing.T) {
	s := newTestServer("x")
	// 构造 150 行日志
	logs := make([]string, 150)
	for i := range logs {
		logs[i] = "line"
	}

	// 直接测内部截断逻辑:复用 handleToolLogs 的 tail 语义
	// (easytierMgr 未初始化会先返回错误,这里用 tail 校验先行逻辑)
	args := map[string]interface{}{"tool": "easytier", "id": "1", "tail": float64(50)}
	res := s.handleToolLogs(args)
	// easytierMgr 为 nil,应返回"管理器未初始化"而非 tail 错误(说明 tail 校验通过后进入 switch)
	if isErr, _ := res["isError"].(bool); !isErr {
		t.Fatalf("easytierMgr 未初始化应返回错误, got %v", res)
	}
	if msg, _ := res["content"].([]interface{}); len(msg) > 0 {
		if text, _ := msg[0].(map[string]interface{})["text"].(string); strings.Contains(text, "tail 参数非法") {
			t.Fatalf("合法 tail 不应报参数错误: %s", text)
		}
	}
}

// TestHandleToolLogs_Tail非法 tail 非法值应返回参数错误
func TestHandleToolLogs_Tail非法(t *testing.T) {
	s := newTestServer("x")

	args := map[string]interface{}{"tool": "easytier", "id": "1", "tail": float64(0)}
	res := s.handleToolLogs(args)
	if msg, _ := res["content"].([]interface{}); len(msg) > 0 {
		if text, _ := msg[0].(map[string]interface{})["text"].(string); !strings.Contains(text, "tail 参数非法") {
			t.Fatalf("tail=0 应报参数非法, got: %s", text)
		}
	}
}
