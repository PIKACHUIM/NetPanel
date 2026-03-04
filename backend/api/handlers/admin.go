package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/utils"
	"github.com/netpanel/netpanel/service/syslog"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ─── 日志查看处理器 ────────────────────────────────────────────────────────────

// SyslogHandler 系统日志处理器
type SyslogHandler struct {
	db     *gorm.DB
	log    *logrus.Logger
	svcMgr *syslog.Manager
}

// NewSyslogHandler 创建系统日志处理器
func NewSyslogHandler(db *gorm.DB, log *logrus.Logger, svcMgr *syslog.Manager) *SyslogHandler {
	return &SyslogHandler{db: db, log: log, svcMgr: svcMgr}
}

// QueryLogs 查询日志列表
// GET /api/v1/admin/logs?service=frp&level=error&keyword=xxx&start_at=2024-01-01T00:00:00Z&end_at=...&page=1&page_size=50&order=desc
func (h *SyslogHandler) QueryLogs(c *gin.Context) {
	params := syslog.QueryParams{
		Service:  c.Query("service"),
		Level:    c.Query("level"),
		Keyword:  c.Query("keyword"),
		Order:    c.DefaultQuery("order", "desc"),
		Page:     1,
		PageSize: 50,
	}

	if p, err := strconv.Atoi(c.Query("page")); err == nil && p > 0 {
		params.Page = p
	}
	if ps, err := strconv.Atoi(c.Query("page_size")); err == nil && ps > 0 {
		params.PageSize = ps
	}
	if s := c.Query("start_at"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			params.StartAt = t
		}
	}
	if s := c.Query("end_at"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			params.EndAt = t
		}
	}

	result, err := h.svcMgr.Query(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
}

// GetLogServices 获取所有出现过的服务类型
// GET /api/v1/admin/logs/services
func (h *SyslogHandler) GetLogServices(c *gin.Context) {
	services := h.svcMgr.GetServices()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": services})
}

// CleanupLogs 清理旧日志
// DELETE /api/v1/admin/logs?days=30
func (h *SyslogHandler) CleanupLogs(c *gin.Context) {
	days := 30
	if d, err := strconv.Atoi(c.Query("days")); err == nil && d > 0 {
		days = d
	}
	count, err := h.svcMgr.Cleanup(days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "清理完成", "data": gin.H{"deleted": count}})
}

// ─── 用户管理处理器 ────────────────────────────────────────────────────────────

// UserHandler 用户管理处理器
type UserHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

// NewUserHandler 创建用户管理处理器
func NewUserHandler(db *gorm.DB, log *logrus.Logger) *UserHandler {
	return &UserHandler{db: db, log: log}
}

// userResponse 用户响应（不含密码）
type userResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Enable    bool      `json:"enable"`
	IsAdmin   bool      `json:"is_admin"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserResponse(u model.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Enable:    u.Enable,
		IsAdmin:   u.IsAdmin,
		Remark:    u.Remark,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
}

// ListUsers 获取用户列表
// GET /api/v1/admin/users
func (h *UserHandler) ListUsers(c *gin.Context) {
	var users []model.User
	h.db.Order("id asc").Find(&users)
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": resp})
}

// CreateUser 创建用户
// POST /api/v1/admin/users
func (h *UserHandler) CreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required,min=2,max=50"`
		Password string `json:"password" binding:"required,min=6"`
		Email    string `json:"email"`
		Enable   bool   `json:"enable"`
		IsAdmin  bool   `json:"is_admin"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 检查用户名是否已存在
	var count int64
	h.db.Model(&model.User{}).Where("username = ?", req.Username).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "用户名已存在"})
		return
	}

	// 加密密码
	hashed, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "密码加密失败"})
		return
	}

	user := model.User{
		Username: req.Username,
		Password: hashed,
		Email:    req.Email,
		Enable:   req.Enable,
		IsAdmin:  req.IsAdmin,
		Remark:   req.Remark,
	}
	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建用户失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": toUserResponse(user), "message": "创建成功"})
}

// UpdateUser 更新用户信息
// PUT /api/v1/admin/users/:id
func (h *UserHandler) UpdateUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	// 获取当前操作者
	currentUser, _ := c.Get("username")
	currentUsername, _ := currentUser.(string)

	var req struct {
		Password string `json:"password"` // 为空则不修改
		Email    string `json:"email"`
		Enable   *bool  `json:"enable"`
		IsAdmin  *bool  `json:"is_admin"`
		Remark   string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "用户不存在"})
		return
	}

	// admin 用户不允许修改 is_admin 字段（防止自我降权）
	if user.Username == "admin" && req.IsAdmin != nil && !*req.IsAdmin {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "admin 用户不允许取消管理员权限"})
		return
	}

	// 非 admin 操作者不能修改 is_admin
	if currentUsername != "admin" && req.IsAdmin != nil {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "只有 admin 可以修改管理员权限"})
		return
	}

	updates := map[string]interface{}{
		"email":  req.Email,
		"remark": req.Remark,
	}
	if req.Enable != nil {
		// admin 用户不允许被禁用
		if user.Username == "admin" && !*req.Enable {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "admin 用户不允许被禁用"})
			return
		}
		updates["enable"] = *req.Enable
	}
	if req.IsAdmin != nil {
		updates["is_admin"] = *req.IsAdmin
	}
	if req.Password != "" {
		if len(req.Password) < 6 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "密码至少6位"})
			return
		}
		hashed, err := utils.HashPassword(req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "密码加密失败"})
			return
		}
		updates["password"] = hashed
		// 同步更新 SystemConfig 中的 admin_password（兼容旧登录逻辑）
		if user.Username == "admin" {
			h.db.Model(&model.SystemConfig{}).Where("key = ?", "admin_password").Update("value", hashed)
		}
	}

	if err := h.db.Model(&user).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
		return
	}

	// 重新查询返回最新数据
	h.db.First(&user, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": toUserResponse(user), "message": "更新成功"})
}

// DeleteUser 删除用户
// DELETE /api/v1/admin/users/:id
func (h *UserHandler) DeleteUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var user model.User
	if err := h.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "用户不存在"})
		return
	}
	if user.Username == "admin" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "admin 用户不允许删除"})
		return
	}

	h.db.Delete(&model.User{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// GetCurrentUser 获取当前登录用户信息
// GET /api/v1/admin/users/me
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	username, _ := c.Get("username")
	var user model.User
	if err := h.db.Where("username = ?", username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": toUserResponse(user)})
}
