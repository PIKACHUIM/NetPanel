package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/meshnode"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== 组网节点管理 =====

// MeshNodeHandler 组网节点处理器
type MeshNodeHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *meshnode.Manager
}

func NewMeshNodeHandler(db *gorm.DB, log *logrus.Logger, mgr *meshnode.Manager) *MeshNodeHandler {
	return &MeshNodeHandler{db: db, log: log, mgr: mgr}
}

// ListNodes 获取节点列表
func (h *MeshNodeHandler) ListNodes(c *gin.Context) {
	var nodes []model.MeshNode
	if err := h.db.Order("id desc").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": nodes})
}

// CreateNode 创建节点
func (h *MeshNodeHandler) CreateNode(c *gin.Context) {
	var node model.MeshNode
	if err := c.ShouldBindJSON(&node); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	node.IsOnline = false
	node.Latency = -1
	if err := h.db.Create(&node).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.mgr.RecordEvent(node.ID, node.Name, "created", fmt.Sprintf("新增节点 %s (%s)", node.Name, node.URL))
	logger.WriteLog("info", "meshnode", fmt.Sprintf("创建组网节点 [%d] %s", node.ID, node.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": node, "message": "创建成功"})
}

// UpdateNode 更新节点
func (h *MeshNodeHandler) UpdateNode(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var existing model.MeshNode
	if err := h.db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
		return
	}

	var req model.MeshNode
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	req.ID = uint(id)
	// 保留只读字段
	req.IsOnline = existing.IsOnline
	req.NodeIP = existing.NodeIP
	req.Latency = existing.Latency
	req.LastHeartbeat = existing.LastHeartbeat
	req.PeerLatencies = existing.PeerLatencies
	// Secret 空值（掩码回显提交）= 不修改，回填数据库现值
	model.PreserveSecrets(&req, &existing)

	if err := h.db.Save(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.mgr.RecordEvent(uint(id), req.Name, "updated", fmt.Sprintf("修改节点 %s", req.Name))
	logger.WriteLog("info", "meshnode", fmt.Sprintf("修改组网节点 [%d] %s", id, req.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

// DeleteNode 删除节点
func (h *MeshNodeHandler) DeleteNode(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var node model.MeshNode
	if err := h.db.First(&node, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
		return
	}
	if err := h.db.Delete(&model.MeshNode{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.mgr.RecordEvent(uint(id), node.Name, "deleted", fmt.Sprintf("删除节点 %s", node.Name))
	logger.WriteLog("info", "meshnode", fmt.Sprintf("删除组网节点 [%d] %s", id, node.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// GetNode 获取单个节点详情
func (h *MeshNodeHandler) GetNode(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var node model.MeshNode
	if err := h.db.First(&node, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": node})
}

// CheckNode 手动检测节点连通性
func (h *MeshNodeHandler) CheckNode(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var node model.MeshNode
	if err := h.db.First(&node, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
		return
	}
	// 触发一次心跳检测
	latency, err := h.mgr.PingTarget(node.URL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"is_online": false, "latency": -1, "error": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"is_online": true, "latency": latency}})
}

// GetTopology 获取拓扑数据
func (h *MeshNodeHandler) GetTopology(c *gin.Context) {
	data, err := h.mgr.GetTopology()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": data})
}

// ListEvents 获取节点事件列表
func (h *MeshNodeHandler) ListEvents(c *gin.Context) {
	var events []model.MeshNodeEvent
	query := h.db.Order("event_time desc")

	// 可选过滤
	if nodeID := c.Query("node_id"); nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	if eventType := c.Query("event_type"); eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	// 分页
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	var total int64
	query.Model(&model.MeshNodeEvent{}).Count(&total)
	query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&events)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"list":      events,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// CleanEvents 清理事件
func (h *MeshNodeHandler) CleanEvents(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 {
		days = 30
	}
	cutoff := fmt.Sprintf("-%d days", days)
	if err := h.db.Where("event_time < datetime('now', ?)", cutoff).Delete(&model.MeshNodeEvent{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "清理成功"})
}

// Ping 本机 ping 目标（供其他节点调用）
func (h *MeshNodeHandler) Ping(c *gin.Context) {
	var req struct {
		TargetURL string `json:"target_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.TargetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "target_url 不能为空"})
		return
	}
	latency, err := h.mgr.PingTarget(req.TargetURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"latency": -1, "error": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"latency": latency}})
}

// ProxyToNode 代理请求到远程节点
// 前端调用 /api/v1/mesh/proxy/:nodeId/* 时，将请求转发到对应节点
func (h *MeshNodeHandler) ProxyToNode(c *gin.Context) {
	nodeID, _ := strconv.ParseUint(c.Param("nodeId"), 10, 64)
	// 获取剩余路径
	proxyPath := c.Param("path")
	if proxyPath == "" {
		proxyPath = "/"
	}

	// 读取请求体
	var body []byte
	if c.Request.Body != nil {
		body, _ = io.ReadAll(c.Request.Body)
	}

	// 代理到远程节点
	resp, err := h.mgr.ProxyRequest(uint(nodeID), c.Request.Method, "/api/v1"+proxyPath, body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": 502, "message": fmt.Sprintf("代理请求失败: %v", err)})
		return
	}

	// 直接返回远程节点的响应
	var result json.RawMessage
	if err := json.Unmarshal(resp, &result); err != nil {
		c.Data(http.StatusOK, "application/json", resp)
		return
	}
	c.Data(http.StatusOK, "application/json", resp)
}
