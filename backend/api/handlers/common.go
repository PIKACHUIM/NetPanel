// Package handlers 公共辅助函数：统一请求参数解析与错误响应，
// 避免各 handler 重复实现且忽略解析错误。
package handlers

import (
	"net/http"
	"strconv"

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
