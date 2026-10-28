package caddy

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/secret"
	"github.com/netpanel/netpanel/pkg/session"
)

type authNextHandler struct{ called bool }

func (h *authNextHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) error {
	h.called = true
	w.WriteHeader(http.StatusOK)
	return nil
}

func TestSessionAuthMiddlewareRejectsForgedCookieAndBuildsOAuthURL(t *testing.T) {
	secret.InitForTest("session-test-key-with-sufficient-length")
	request := httptest.NewRequest(http.MethodGet, "https://app.example.com/private?view=full&sort=asc", nil)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: "forged"})
	next := &authNextHandler{}
	recorder := httptest.NewRecorder()
	middleware := &SessionAuthMiddleware{PanelPort: 1086, AllowedHosts: []string{"app.example.com"}, OAuthProvider: "corporate-login"}

	if err := middleware.ServeHTTP(recorder, request, caddyhttp.HandlerFunc(next.ServeHTTP)); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}
	if next.called {
		t.Fatal("forged cookie unexpectedly reached protected handler")
	}
	if recorder.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusFound)
	}
	location, err := url.Parse(recorder.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if location.Host != "app.example.com:1086" {
		t.Fatalf("redirect host = %q, want panel port on request host", location.Host)
	}
	if location.Path != "/api/v1/auth/oauth/corporate-login/authorize" {
		t.Fatalf("redirect path = %q, want API OAuth authorize endpoint", location.Path)
	}
	if got := location.Query().Get("redirect"); got != "https://app.example.com/private?view=full&sort=asc" {
		t.Fatalf("redirect query = %q, want original URL", got)
	}
	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", recorder.Header().Get("Cache-Control"))
	}
}

func TestSessionAuthMiddlewareRejectsUntrustedHostHeader(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://attacker.example/private", nil)
	next := &authNextHandler{}
	recorder := httptest.NewRecorder()
	middleware := &SessionAuthMiddleware{PanelPort: 1086, AllowedHosts: []string{"app.example.com"}}
	if err := middleware.ServeHTTP(recorder, request, caddyhttp.HandlerFunc(next.ServeHTTP)); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}
	if recorder.Code != http.StatusBadRequest || recorder.Header().Get("Location") != "" {
		t.Fatalf("untrusted Host response = status %d, Location %q; want 400 without redirect", recorder.Code, recorder.Header().Get("Location"))
	}
}

func TestSessionAuthMiddlewareAllowsConfiguredWildcardHost(t *testing.T) {
	middleware := &SessionAuthMiddleware{PanelPort: 1086, AllowedHosts: []string{"*.example.com"}}
	request := httptest.NewRequest(http.MethodGet, "http://node.example.com/private", nil)
	location, ok := middleware.loginURL(request)
	if !ok {
		t.Fatal("configured wildcard host was rejected")
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}
	if parsed.Host != "node.example.com:1086" {
		t.Fatalf("redirect host = %q, want trusted wildcard subdomain", parsed.Host)
	}
	if _, ok := middleware.loginURL(httptest.NewRequest(http.MethodGet, "http://a.b.example.com/private", nil)); ok {
		t.Fatal("multi-label wildcard host should be rejected")
	}
}

func TestSessionAuthMiddlewareRejectsRevokedSessionVersion(t *testing.T) {
	secret.InitForTest("session-test-key-with-sufficient-length")
	manager := newAuthRouteTestManager(t)
	user := model.User{Username: "operator", Enable: true, TokenVersion: 2}
	if err := manager.db.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}
	SetAuthDB(manager.db)
	t.Cleanup(func() { SetAuthDB(nil) })

	request := httptest.NewRequest(http.MethodGet, "https://app.example.com/private", nil)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: session.Issue(user.Username, 1)})
	next := &authNextHandler{}
	recorder := httptest.NewRecorder()
	middleware := &SessionAuthMiddleware{PanelPort: 1086, AllowedHosts: []string{"app.example.com"}}
	if err := middleware.ServeHTTP(recorder, request, caddyhttp.HandlerFunc(next.ServeHTTP)); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}
	if next.called || recorder.Code != http.StatusFound {
		t.Fatalf("revoked session reached handler=%v status=%d, want rejected redirect", next.called, recorder.Code)
	}
}

func TestSessionAuthMiddlewareReturnsForbiddenForAuthenticatedUserOutsideACL(t *testing.T) {
	secret.InitForTest("session-test-key-with-sufficient-length")
	manager := newAuthRouteTestManager(t)
	user := model.User{Username: "operator", Enable: true, TokenVersion: 3}
	if err := manager.db.Create(&user).Error; err != nil {
		t.Fatalf("create test user: %v", err)
	}
	SetAuthDB(manager.db)
	t.Cleanup(func() { SetAuthDB(nil) })

	request := httptest.NewRequest(http.MethodGet, "https://app.example.com/private", nil)
	request.AddCookie(&http.Cookie{Name: session.CookieName, Value: session.Issue(user.Username, user.TokenVersion)})
	next := &authNextHandler{}
	recorder := httptest.NewRecorder()
	middleware := &SessionAuthMiddleware{PanelPort: 1086, AllowedUserIDs: []uint{user.ID + 1}}
	if err := middleware.ServeHTTP(recorder, request, caddyhttp.HandlerFunc(next.ServeHTTP)); err != nil {
		t.Fatalf("ServeHTTP: %v", err)
	}
	if next.called || recorder.Code != http.StatusForbidden {
		t.Fatalf("unauthorized session reached handler=%v status=%d, want 403", next.called, recorder.Code)
	}
	if recorder.Header().Get("Location") != "" {
		t.Fatalf("unauthorized user was redirected to login: %q", recorder.Header().Get("Location"))
	}
}

func TestParseAllowedUserIDsFailsClosed(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []uint
		valid bool
	}{
		{name: "empty means any signed-in user", valid: true},
		{name: "explicit empty list", input: "[]", valid: true},
		{name: "specific users", input: "[2,5]", want: []uint{2, 5}, valid: true},
		{name: "invalid json", input: "[2,", valid: false},
		{name: "null is invalid", input: "null", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, valid := parseAllowedUserIDs(test.input)
			if valid != test.valid || len(got) != len(test.want) {
				t.Fatalf("parseAllowedUserIDs(%q) = (%v, %v), want (%v, %v)", test.input, got, valid, test.want, test.valid)
			}
			for i := range got {
				if got[i] != test.want[i] {
					t.Fatalf("parseAllowedUserIDs(%q) = %v, want %v", test.input, got, test.want)
				}
			}
		})
	}
}
