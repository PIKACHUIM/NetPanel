package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/ddns"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type DDNSHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *ddns.Manager
}

func NewDDNSHandler(db *gorm.DB, log *logrus.Logger, mgr *ddns.Manager) *DDNSHandler {
	return &DDNSHandler{db: db, log: log, mgr: mgr}
}

func (h *DDNSHandler) List(c *gin.Context) {
	var tasks []model.DDNSTask
	h.db.Order("id desc").Find(&tasks)
	for i := range tasks {
		tasks[i].Status = h.mgr.GetStatus(tasks[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tasks})
}

func (h *DDNSHandler) Create(c *gin.Context) {
	var task model.DDNSTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	task.Status = "stopped"
	h.db.Create(&task)
	if task.Enable {
		h.mgr.Start(task.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": task, "message": "创建成功"})
}

func (h *DDNSHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DDNSTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.Stop(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	if req.Enable {
		h.mgr.Start(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DDNSHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Delete(&model.DDNSTask{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *DDNSHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.Start(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("enable", true)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *DDNSHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.Stop(uint(id))
	h.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("enable", false)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

func (h *DDNSHandler) RunNow(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.RunNow(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已触发更新"})
}

func (h *DDNSHandler) GetHistory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	var histories []model.DDNSHistory
	var total int64
	h.db.Model(&model.DDNSHistory{}).Where("task_id = ?", id).Count(&total)
	h.db.Where("task_id = ?", id).Order("id desc").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&histories)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      histories,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}
