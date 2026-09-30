// Package handlers 公共辅助函数：统一请求参数解析与错误响应，
// 避免各 handler 重复实现且忽略解析错误。
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseUintParam 解析路径参数为 uint。
// 解析失败或为 0 时直接写入 400 响应并返回 ok=false，调用方应立即 return。
//
// 说明：此前各 handler 普遍使用 `id, _ := strconv.ParseUint(...)` 忽略错误，
// 非法 id 会被静默转为 0，导致 Delete(&Model{}, 0)、Stop(0) 等语义不明的操作。
func parseUintParam(c *gin.Context, name string) (uint, bool) {
	raw := c.Param(name)
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "参数 " + name + " 非法: " + raw,
		})
		return 0, false
	}
	return uint(v), true
}

// diffJSON 将 before/after 转为 JSON 并做基础脱敏（替换 token/secret 类字段）。
func diffJSON(before, after interface{}) string {
	if before == nil && after == nil {
		return ""
	}
	sanitized := func(v interface{}) string {
		if v == nil {
			return "{}"
		}
		b, err := json.Marshal(v)
		if err != nil {
			return "{}"
		}
		s := string(b)
		s = strings.NewReplacer(
			`"token":"`, `"token":"[REDACTED]"`,
			`"secret":"`, `"secret":"[REDACTED]"`,
			`"password":"`, `"password":"[REDACTED]"`,
			`"cert":"`, `"cert":"[REDACTED]"`,
			`"key":"`, `"key":"[REDACTED]"`,
			`"ClientSecret":"`, `"ClientSecret":"[REDACTED]"`,
			`"PrivateKey":"`, `"PrivateKey":"[REDACTED]"`,
		).Replace(s)
		return s
	}
	beforeJSON := sanitized(before)
	afterJSON := sanitized(after)
	if beforeJSON == afterJSON {
		return ""
	}
	return fmt.Sprintf("before=%s|after=%s", beforeJSON, afterJSON)
}
