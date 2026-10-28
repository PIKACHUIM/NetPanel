package handlers

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api/middleware"
)

const responseSecretMask = "********"

var sensitiveResponseFields = map[string]struct{}{
	"accesssecret": {}, "agenttoken": {}, "apikey": {}, "authkey": {},
	"authorization": {}, "basicauthpasswd": {}, "clientsecret": {}, "configservertoken": {},
	"actionconfig": {}, "certfile": {}, "eabhmackey": {}, "extraconfig": {}, "headers": {}, "httpheaders": {}, "httppassword": {},
	"keycontent": {}, "loadbalancergroupkey": {}, "password": {}, "presharedkey": {},
	"keyfile": {}, "privatekey": {}, "requestheaders": {}, "secretkey": {}, "sshpassword": {}, "token": {}, "proxyurl": {},
	"tlscertfile": {}, "tlskeyfile": {},
	"vkeyorid": {}, "webhookheaders": {}, "webpassword": {},
}

// maskSensitiveResponse 为非管理员生成脱敏响应投影。管理员保留原有管理界面所需字段；
// 写接口仍由 AdminOnly 保护，读接口不会把配置秘密交给普通登录用户。
func maskSensitiveResponse(c *gin.Context, value any) any {
	if middleware.IsCurrentUserAdmin(c) || value == nil {
		return value
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var projected any
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	if err := decoder.Decode(&projected); err != nil {
		return value
	}
	return redactSensitiveFields(projected)
}

func redactSensitiveFields(value any) any {
	switch current := value.(type) {
	case map[string]any:
		for key, field := range current {
			normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
			if _, sensitive := sensitiveResponseFields[normalized]; sensitive {
				if text, ok := field.(string); ok && text != "" {
					current[key] = responseSecretMask
				}
				continue
			}
			current[key] = redactSensitiveFields(field)
		}
		return current
	case []any:
		for i := range current {
			current[i] = redactSensitiveFields(current[i])
		}
		return current
	default:
		return value
	}
}
