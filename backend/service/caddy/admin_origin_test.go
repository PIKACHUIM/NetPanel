package caddy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminRequestIncludesAllowedOrigin(t *testing.T) {
	var gotOrigin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOrigin = r.Header.Get("Origin")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	manager := &Manager{
		admin:     adminEndpoint{baseURL: server.URL},
		adminHTTP: server.Client(),
	}
	if err := manager.adminRequest(http.MethodGet, "/config/", nil); err != nil {
		t.Fatalf("adminRequest returned error: %v", err)
	}
	if gotOrigin != server.URL {
		t.Fatalf("Origin = %q, want %q", gotOrigin, server.URL)
	}
}
