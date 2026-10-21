package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/service/docker"
)

// DockerHandler Docker 容器端口扫描
type DockerHandler struct{}

func NewDockerHandler() *DockerHandler { return &DockerHandler{} }

// ListContainers 列出运行中容器的端口映射（用于生成推荐穿透规则）
func (h *DockerHandler) ListContainers(c *gin.Context) {
	containers, err := docker.ListContainers()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": containers})
}
