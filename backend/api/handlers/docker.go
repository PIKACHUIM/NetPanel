package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/service/docker"
)

// DockerHandler Docker 容器管理
type DockerHandler struct{}

func NewDockerHandler() *DockerHandler { return &DockerHandler{} }

// ListContainers 列出容器：all=1 时包含已停止容器（本地容器管理页），
// 否则仅返回有端口映射的运行中容器（用于生成推荐穿透规则）。
func (h *DockerHandler) ListContainers(c *gin.Context) {
	var (
		containers []docker.ContainerInfo
		err        error
	)
	if c.Query("all") == "1" {
		containers, err = docker.ListAllContainers()
	} else {
		containers, err = docker.ListContainers()
	}
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 503, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": containers})
}

// Start 启动容器。
func (h *DockerHandler) Start(c *gin.Context) {
	h.action(c, "start")
}

// Stop 停止容器。
func (h *DockerHandler) Stop(c *gin.Context) {
	h.action(c, "stop")
}

// Restart 重启容器。
func (h *DockerHandler) Restart(c *gin.Context) {
	h.action(c, "restart")
}

func (h *DockerHandler) action(c *gin.Context, op string) {
	id := c.Param("id")
	if err := docker.Action(id, op); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}
