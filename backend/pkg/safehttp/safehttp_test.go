package safehttp

import "testing"

func TestValidateURLRejectsPrivateAndSpecialAddresses(t *testing.T) {
	for _, raw := range []string{
		"http://127.0.0.1/admin",
		"http://10.0.0.1/",
		"http://192.168.1.2/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"http://[fd00::1]/",
		"file:///etc/passwd",
	} {
		if _, err := ValidateURL(raw); err == nil {
			t.Errorf("ValidateURL(%q) unexpectedly succeeded", raw)
		}
	}
}

func TestValidateURLAcceptsPublicHTTPURL(t *testing.T) {
	if _, err := ValidateURL("https://example.com/path"); err != nil {
		t.Fatalf("ValidateURL(public URL): %v", err)
	}
}
