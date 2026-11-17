package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/crud"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)



type CallbackAccountHandler struct {
	repo crud.Store[model.CallbackAccount]
	log  *logrus.Logger
	mgr  *callback.Manager
}

func NewCallbackAccountHandler(db *gorm.DB, log *logrus.Logger, mgr *callback.Manager) *CallbackAccountHandler {
	return &CallbackAccountHandler{repo: crud.New[model.CallbackAccount](db), log: log, mgr: mgr}
}

func (h *CallbackAccountHandler) List(c *gin.Context) {
	accounts, err := h.repo.List(c.Request.Context(), crud.ListOptions{OrderBy: "id desc"})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *CallbackAccountHandler) Create(c *gin.Context) {
	var account model.CallbackAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.repo.Create(c.Request.Context(), &account); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "callback", fmt.Sprintf("创建回调账号 [%d]", account.ID))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *CallbackAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	if err := h.repo.Update(c.Request.Context(), uint(id), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "callback", fmt.Sprintf("更新回调账号 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.repo.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "callback", fmt.Sprintf("删除回调账号 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CallbackAccountHandler) Test(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.TestAccount(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "测试失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "测试成功"})
}

// ===== 回调任务 =====

type CallbackTaskHandler struct {
	repo crud.Store[model.CallbackTask]
	log  *logrus.Logger
}

func NewCallbackTaskHandler(db *gorm.DB, log *logrus.Logger) *CallbackTaskHandler {
	return &CallbackTaskHandler{repo: crud.New[model.CallbackTask](db), log: log}
}

func (h *CallbackTaskHandler) List(c *gin.Context) {
	tasks, err := h.repo.List(c.Request.Context(), crud.ListOptions{OrderBy: "id desc"})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tasks})
}

func (h *CallbackTaskHandler) Create(c *gin.Context) {
	var task model.CallbackTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.repo.Create(c.Request.Context(), &task); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "callback", fmt.Sprintf("创建回调任务 [%d]", task.ID))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": task, "message": "创建成功"})
}

func (h *CallbackTaskHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	if err := h.repo.Update(c.Request.Context(), uint(id), &req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "callback", fmt.Sprintf("更新回调任务 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackTaskHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.repo.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "callback", fmt.Sprintf("删除回调任务 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}
