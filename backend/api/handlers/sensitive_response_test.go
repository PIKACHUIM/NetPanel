package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
)

func TestMaskSensitiveResponseRedactsNestedCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("is_admin", false)
	input := map[string]any{
		"servers": []any{map[string]any{
			"agent_token": "agent-secret",
			"name":        "edge-1",
		}},
		"key_content":     "private-key",
		"key_file":        "/srv/netpanel/private/key.pem",
		"cert_file":       "/srv/netpanel/certs/site.pem",
		"tls_key_file":    "/srv/netpanel/certs/site.key",
		"tls_cert_file":   "/srv/netpanel/certs/site.pem",
		"request_headers": "Authorization: bearer-secret",
	}

	got := maskSensitiveResponse(ctx, input)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal redacted response: %v", err)
	}
	var response map[string]any
	if err := json.Unmarshal(encoded, &response); err != nil {
		t.Fatalf("decode redacted response: %v", err)
	}
	servers := response["servers"].([]any)
	server := servers[0].(map[string]any)
	if server["agent_token"] != responseSecretMask || response["key_content"] != responseSecretMask || response["key_file"] != responseSecretMask || response["cert_file"] != responseSecretMask || response["tls_key_file"] != responseSecretMask || response["tls_cert_file"] != responseSecretMask || response["request_headers"] != responseSecretMask {
		t.Fatalf("sensitive response fields were not redacted: %s", encoded)
	}
	if server["name"] != "edge-1" {
		t.Fatalf("non-sensitive response field changed: %s", encoded)
	}
}

func TestMaskSensitiveResponsePreservesAdminConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("is_admin", true)
	input := model.MonitorServer{AgentToken: "admin-visible-token"}

	got := maskSensitiveResponse(ctx, input)
	server, ok := got.(model.MonitorServer)
	if !ok || server.AgentToken != input.AgentToken {
		t.Fatalf("admin configuration was unexpectedly changed: %#v", got)
	}
}
