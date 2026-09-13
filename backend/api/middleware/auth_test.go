package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/pkg/secret"
)

func initSecret(t *testing.T) {
	t.Helper()
	secret.InitForTest("unit-test-master-key-0123456789")
}

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	initSecret(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	auth := r.Group("")
	auth.Use(JWTAuth())
	admin := r.Group("")
	admin.Use(JWTAuth(), AdminOnly())
	auth.GET("/open", func(c *gin.Context) { c.String(200, "ok") })
	admin.GET("/admin-only", func(c *gin.Context) { c.String(200, "ok") })
	return r
}

func tokenFor(t *testing.T, username string, userID uint, admin bool) string {
	t.Helper()
	tok, err := GenerateToken(username, userID, admin)
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}
	return tok
}

func doReq(r *gin.Engine, token string, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestJWTAuthAndAdminOnly(t *testing.T) {
	r := newTestRouter(t)
	adminTok := tokenFor(t, "admin", 1, true)
	userTok := tokenFor(t, "alice", 2, false)

	// 无 token -> 401
	if w := doReq(r, "", "/admin-only"); w.Code != http.StatusUnauthorized {
		t.Errorf("无 token 应 401, got %d", w.Code)
	}
	// 非 admin 访问 admin 路由 -> 403
	if w := doReq(r, userTok, "/admin-only"); w.Code != http.StatusForbidden {
		t.Errorf("非管理员应 403, got %d", w.Code)
	}
	// admin 访问 admin 路由 -> 200
	if w := doReq(r, adminTok, "/admin-only"); w.Code != http.StatusOK {
		t.Errorf("管理员应 200, got %d", w.Code)
	}
	// 非管理员访问普通路由 -> 200
	if w := doReq(r, userTok, "/open"); w.Code != http.StatusOK {
		t.Errorf("普通登录用户应 200, got %d", w.Code)
	}
	// 伪造 token -> 401
	if w := doReq(r, adminTok+"x", "/admin-only"); w.Code != http.StatusUnauthorized {
		t.Errorf("非法 token 应 401, got %d", w.Code)
	}
}

func TestParseTokenClaims(t *testing.T) {
	initSecret(t)
	claims, err := ParseToken(mustToken(t, "bob", 7, false))
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if claims.Username != "bob" || claims.UserID != 7 || claims.IsAdmin {
		t.Errorf("claims 不符: %+v", claims)
	}
}

func mustToken(t *testing.T, u string, id uint, admin bool) string {
	t.Helper()
	return tokenFor(t, u, id, admin)
}
