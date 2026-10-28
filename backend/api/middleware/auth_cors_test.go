package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCORSRejectsWildcardWithCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("NETPANEL_ALLOWED_ORIGINS", "*")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Request.Header.Set("Origin", "https://untrusted.example")

	CORS()(ctx)
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("wildcard origin emitted ACAO %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Fatalf("wildcard origin emitted credential permission %q", got)
	}
}

func TestCORSAllowsExactConfiguredOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("NETPANEL_ALLOWED_ORIGINS", "https://trusted.example")
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	ctx.Request.Header.Set("Origin", "https://trusted.example")

	CORS()(ctx)
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://trusted.example" {
		t.Fatalf("allowed origin = %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("credential header = %q", got)
	}
}
