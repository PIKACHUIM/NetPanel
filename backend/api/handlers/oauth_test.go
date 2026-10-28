package handlers

import "testing"

func TestSafeOAuthRedirect(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "site path with query", input: "/protected/page?mode=full", want: "/protected/page?mode=full"},
		{name: "absolute external URL", input: "https://attacker.example/path", want: ""},
		{name: "scheme relative URL", input: "//attacker.example/path", want: ""},
		{name: "multiple leading slashes", input: "///attacker.example/path", want: ""},
		{name: "backslash authority", input: `/\attacker.example/path`, want: ""},
		{name: "relative path", input: "dashboard", want: ""},
		{name: "javascript URL", input: "javascript:alert(1)", want: ""},
		{name: "empty path", input: "", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := safeOAuthRedirect(test.input); got != test.want {
				t.Errorf("safeOAuthRedirect(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestOAuthStateCookieNameIsPerFlow(t *testing.T) {
	first := "0123456789abcdef0123456789abcdef.ey9wYXRo"
	second := "fedcba9876543210fedcba9876543210.ey9wYXRo"
	if oauthStateCookieName(first) == oauthStateCookieName(second) {
		t.Fatal("concurrent OAuth states share a cookie name")
	}
	if got := oauthStateCookieName("invalid.state"); got != "oauth_state_invalid" {
		t.Fatalf("invalid state cookie name = %q, want fallback", got)
	}
}
