// Package handlers frpc 多节点 Master 控制面 API。
//
// 管理面（面板 JWT）：
//
//	GET    /api/v1/frpmaster/nodes           节点列表（含在线状态）
//	POST   /api/v1/frpmaster/nodes           注册节点（返回一次性 node_token）
//	DELETE /api/v1/frpmaster/nodes/:id       移除节点
//	GET    /api/v1/frpmaster/nodes/:id/config 预览节点 frpc.toml
//
// 控制面（远程节点，node_id + token 认证，无面板 JWT）：
//
//	POST /api/v1/frpmaster/agent/heartbeat   心跳
//	POST /api/v1/frpmaster/agent/status      状态上报（隧道列表 JSON）
//	GET  /api/v1/frpmaster/agent/config      拉取本节点 frpc.toml
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/frpmaster"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// FrpMasterHandler frpc 多节点 Master 控制面 API。
type FrpMasterHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *frpmaster.Manager
}

// NewFrpMasterHandler 创建节点管理处理器。
func NewFrpMasterHandler(db *gorm.DB, log *logrus.Logger, mgr *frpmaster.Manager) *FrpMasterHandler {
	return &FrpMasterHandler{db: db, log: log, mgr: mgr}
}

// nodeView 节点视图：嵌入模型字段（token 相关不回显），创建时附一次性明文 token。
type nodeView struct {
	model.FrpMasterNode
	NodeToken string `json:"node_token,omitempty"`
}

// ---- 管理面 ----

// List GET /api/v1/frpmaster/nodes
func (h *FrpMasterHandler) List(c *gin.Context) {
	nodes, err := h.mgr.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询节点失败: " + err.Error()})
		return
	}
	views := make([]nodeView, 0, len(nodes))
	for _, n := range nodes {
		views = append(views, nodeView{FrpMasterNode: n})
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": views})
}

// createNodeReq 注册节点入参。
type createNodeReq struct {
	Name       string `json:"name"`
	Region     string `json:"region"`
	ServerAddr string `json:"server_addr"`
	ServerPort int    `json:"server_port"`
	FrpsToken  string `json:"frps_token"`
	Remark     string `json:"remark"`
}

// Create POST /api/v1/frpmaster/nodes —— 创建成功后 node_token 仅此一次返回。
func (h *FrpMasterHandler) Create(c *gin.Context) {
	var req createNodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败"})
		return
	}
	node, token, err := h.mgr.Create(frpmaster.CreateRequest{
		Name:       req.Name,
		Region:     req.Region,
		ServerAddr: req.ServerAddr,
		ServerPort: req.ServerPort,
		FrpsToken:  req.FrpsToken,
		Remark:     req.Remark,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"message": "创建成功，节点 token 仅返回一次，请妥善保存",
		"data":    nodeView{FrpMasterNode: *node, NodeToken: token},
	})
}

// Delete DELETE /api/v1/frpmaster/nodes/:id
func (h *FrpMasterHandler) Delete(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.mgr.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除节点失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已删除"})
}

// ConfigPreview GET /api/v1/frpmaster/nodes/:id/config —— 管理面预览下发配置。
func (h *FrpMasterHandler) ConfigPreview(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	toml, err := h.mgr.Config(id)
	if err != nil {
		if err == frpmaster.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成配置失败: " + err.Error()})
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(toml))
}

// ---- 控制面（节点 → Master）----

// agentHeartbeatReq 节点心跳。
type agentHeartbeatReq struct {
	NodeID uint   `json:"node_id"`
	Token  string `json:"token"`
}

// agentStatusReq 节点状态上报。
type agentStatusReq struct {
	NodeID  uint            `json:"node_id"`
	Token   string          `json:"token"`
	Tunnels json.RawMessage `json:"tunnels"` // 隧道列表 JSON，M4 解析展示
}

// Heartbeat POST /api/v1/frpmaster/agent/heartbeat
func (h *FrpMasterHandler) Heartbeat(c *gin.Context) {
	var req agentHeartbeatReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败"})
		return
	}
	if !h.mgr.Authenticate(req.NodeID, req.Token) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "节点 token 校验失败"})
		return
	}
	if err := h.mgr.Heartbeat(req.NodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "心跳写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// ReportStatus POST /api/v1/frpmaster/agent/status
func (h *FrpMasterHandler) ReportStatus(c *gin.Context) {
	var req agentStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败"})
		return
	}
	if !h.mgr.Authenticate(req.NodeID, req.Token) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "节点 token 校验失败"})
		return
	}
	if err := h.mgr.SaveStatus(req.NodeID, string(req.Tunnels)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "状态写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok"})
}

// FetchConfig GET /api/v1/frpmaster/agent/config?node_id=&token=
func (h *FrpMasterHandler) FetchConfig(c *gin.Context) {
	nodeID, err := strconv.ParseUint(c.Query("node_id"), 10, 64)
	if err != nil || nodeID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "node_id 非法"})
		return
	}
	token := c.Query("token")
	if !h.mgr.Authenticate(uint(nodeID), token) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "节点 token 校验失败"})
		return
	}
	toml, err := h.mgr.Config(uint(nodeID))
	if err != nil {
		if err == frpmaster.ErrNodeNotFound {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "节点不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "生成配置失败: " + err.Error()})
		return
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(toml))
}

// agentLogsReq 节点日志回传。
type agentLogsReq struct {
	NodeID uint     `json:"node_id"`
	Token  string   `json:"token"`
	Logs   []string `json:"logs"`
}

// ReportLogs POST /api/v1/frpmaster/agent/logs —— 节点日志回传聚合
// （落 SystemLog，Service=frpmaster；单批上限由 Manager 截断）。
func (h *FrpMasterHandler) ReportLogs(c *gin.Context) {
	var req agentLogsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请求体解析失败"})
		return
	}
	if !h.mgr.Authenticate(req.NodeID, req.Token) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "节点 token 校验失败"})
		return
	}
	n, err := h.mgr.SaveLogs(req.NodeID, req.Logs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "日志写入失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "ok", "written": n})
}
