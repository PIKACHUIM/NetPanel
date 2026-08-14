package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/tunservice"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// TunserviceHandler 穿透服务处理器（用户视角的统一穿透管理）
type TunserviceHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *tunservice.Manager
}

func NewTunserviceHandler(db *gorm.DB, log *logrus.Logger, mgr *tunservice.Manager) *TunserviceHandler {
	return &TunserviceHandler{db: db, log: log, mgr: mgr}
}

// List 返回全部穿透服务及其线路（聚合状态）
func (h *TunserviceHandler) List(c *gin.Context) {
	views, err := h.mgr.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": views})
}

// Get 返回单个穿透服务详情（含线路实时状态）
func (h *TunserviceHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	view, err := h.mgr.Get(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": view})
}

func (h *TunserviceHandler) Create(c *gin.Context) {
	var svc model.TunService
	if err := c.ShouldBindJSON(&svc); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	svc.Status = "stopped"
	h.db.Create(&svc)
	logger.WriteLog("info", "tunservice", fmt.Sprintf("创建穿透服务 [%d] %s", svc.ID, svc.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": svc, "message": "创建成功"})
}

func (h *TunserviceHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.TunService
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "tunservice", fmt.Sprintf("更新穿透服务 [%d] %s", id, req.Name))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *TunserviceHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.TunService{}, id)
	logger.WriteLog("info", "tunservice", fmt.Sprintf("删除穿透服务 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// Start 一键启动服务关联的全部线路
func (h *TunserviceHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.TunService{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "tunservice", fmt.Sprintf("启动穿透服务 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

// Stop 一键停止服务关联的全部线路
func (h *TunserviceHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.TunService{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "tunservice", fmt.Sprintf("停止穿透服务 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// Candidates 返回可选线路列表（来自 selector，供服务关联线路时选择）
func (h *TunserviceHandler) Candidates(c *gin.Context) {
	type item struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Tool    string `json:"tool"`
		Layer   string `json:"layer"`
		Address string `json:"address"`
	}
	items := []item{}
	// 通过 linereg 的 selector 快照获取线路（tunservice 管理器不直接暴露，
	// 由 List 内嵌线路即可，这里保持空列表兜底）。
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": items})
}

// History 返回服务关联线路的探测历史（延迟趋势）
func (h *TunserviceHandler) History(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	history, err := h.mgr.History(uint(id), limit)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": history})
}

// Speedtest 对服务关联线路做一次即时并发测速（延迟升序、失败排最后）。
// 受 SystemConfig 键 speedtest_popup_enabled 控制（默认开启；设为 false 时 403）。
func (h *TunserviceHandler) Speedtest(c *gin.Context) {
	var cfg model.SystemConfig
	enabled := "true"
	if err := h.db.Where("key = ?", "speedtest_popup_enabled").First(&cfg).Error; err == nil {
		enabled = cfg.Value
	}
	if enabled == "false" || enabled == "0" {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "测速功能已禁用"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	lines, err := h.mgr.Speedtest(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": lines})
}
