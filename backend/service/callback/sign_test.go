package callback

import (
	"net/http"
	"strings"
	"testing"
)

func TestTC3SignDeterministic(t *testing.T) {
	payload := []byte(`{"ZoneId":"zone-1"}`)
	const ts = int64(1700000000) // 2023-11-14 UTC

	sig1 := tc3Sign("test-id", "test-key", "teo.tencentcloudapi.com", "teo", ts, payload)
	sig2 := tc3Sign("test-id", "test-key", "teo.tencentcloudapi.com", "teo", ts, payload)
	if sig1 != sig2 {
		t.Fatalf("同参数签名应确定:\n%s\n%s", sig1, sig2)
	}

	wantPrefix := "TC3-HMAC-SHA256 Credential=test-id/2023-11-14/teo/tc3_request, SignedHeaders=content-type;host, Signature="
	if !strings.HasPrefix(sig1, wantPrefix) {
		t.Errorf("Authorization 前缀/格式错误: %s", sig1)
	}
	sig := strings.TrimPrefix(sig1, wantPrefix)
	if len(sig) != 64 {
		t.Errorf("签名应为 64 位 hex, got %d: %s", len(sig), sig)
	}

	// payload 变化签名必须变化
	other := tc3Sign("test-id", "test-key", "teo.tencentcloudapi.com", "teo", ts, []byte(`{"ZoneId":"zone-2"}`))
	if other == sig1 {
		t.Error("不同 payload 的签名不应相同")
	}
}

func TestACS3SignSetsHeaders(t *testing.T) {
	req, err := http.NewRequest("POST", "https://esa.aliyuncs.com/?SiteId=site-1&Origin=1.2.3.4:80", nil)
	if err != nil {
		t.Fatal(err)
	}
	acs3Sign(req, "test-ak", "test-sk", "UpdateOriginPool", "2024-09-10")

	for _, h := range []string{"x-acs-action", "x-acs-version", "x-acs-date", "x-acs-signature-nonce"} {
		if req.Header.Get(h) == "" {
			t.Errorf("缺少必需头 %s", h)
		}
	}
	if req.Header.Get("x-acs-action") != "UpdateOriginPool" ||
		req.Header.Get("x-acs-version") != "2024-09-10" {
		t.Errorf("action/version 头错误: %s / %s", req.Header.Get("x-acs-action"), req.Header.Get("x-acs-version"))
	}

	const wantPrefix = "ACS3-HMAC-SHA256 Credential=test-ak, SignedHeaders=host;x-acs-action;x-acs-date;x-acs-signature-nonce;x-acs-version, Signature="
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, wantPrefix) {
		t.Errorf("Authorization 格式错误: %s", auth)
	}
	sig := strings.TrimPrefix(auth, wantPrefix)
	if len(sig) != 64 {
		t.Errorf("签名应为 64 位 hex, got %d: %s", len(sig), sig)
	}
}
