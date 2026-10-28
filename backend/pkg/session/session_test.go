package session

import (
	"strings"
	"testing"

	"github.com/netpanel/netpanel/pkg/secret"
)

func TestSessionTokenVersionIsSigned(t *testing.T) {
	secret.InitForTest("session-test-key-with-sufficient-length")
	cookie := Issue("operator", 7)
	data, ok := ValidateData(cookie)
	if !ok {
		t.Fatal("valid session cookie was rejected")
	}
	if data.Username != "operator" || data.TokenVersion != 7 {
		t.Fatalf("session data = %#v, want operator at token version 7", data)
	}

	payload, signature, found := strings.Cut(cookie, ".")
	if !found {
		t.Fatal("issued cookie does not contain a signature")
	}
	tampered := payload + "0." + signature
	if _, ok := ValidateData(tampered); ok {
		t.Fatal("tampered cookie unexpectedly validated")
	}
}
