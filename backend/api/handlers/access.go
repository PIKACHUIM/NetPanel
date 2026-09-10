package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/access"
	"github.com/netpanel/netpanel/service/caddy"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== 访问控制 =====

type AccessHandler struct {
	db       *gorm.DB
	log      *logrus.Logger
	mgr      *access.Manager
	caddyMgr *caddy.Manager
}

func NewAccessHandler(db *gorm.DB, log *logrus.Logger, mgr *access.Manager, caddyMgr *caddy.Manager) *AccessHandler {
	return &AccessHandler{db: db, log: log, mgr: mgr, caddyMgr: caddyMgr}
}

func (h *AccessHandler) List(c *gin.Context) {
	var rules []model.AccessRule
	h.db.Order("id desc").Find(&rules)

	// 查询所有 IPDB 条目和 Caddy 站点，供前端选择
	var ipdbEntries []model.IPDBEntry
	h.db.Select("id, cidr, location, tags").Order("id desc").Find(&ipdbEntries)

	var caddySites []model.CaddySite
	h.db.Select("id, name, domain, port, site_type").Order("id desc").Find(&caddySites)

	// 查询所有用户，供用户认证配置选择
	var users []model.User
	h.db.Select("id, username, email, enable, is_admin, remark").Order("id asc").Find(&users)

	c.JSON(http.StatusOK, gin.H{
		"code":         200,
		"data":         rules,
		"ipdb_entries": ipdbEntries,
		"caddy_sites":  caddySites,
		"users":        users,
	})
}

func (h *AccessHandler) Create(c *gin.Context) {
	var rule model.AccessRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&rule)
	h.mgr.Reload()
	h.restartBoundSites(rule.BindSiteIDs)
	logger.WriteLog("info", "access", fmt.Sprintf("创建访问控制规则 [%d]", rule.ID))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rule, "message": "创建成功"})
}

func (h *AccessHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.AccessRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	h.mgr.Reload()
	h.restartBoundSites(req.BindSiteIDs)
	logger.WriteLog("info", "access", fmt.Sprintf("更新访问控制规则 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *AccessHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// 先查出规则以获取绑定的站点
	var rule model.AccessRule
	h.db.First(&rule, id)
	h.db.Delete(&model.AccessRule{}, id)
	h.mgr.Reload()
	h.restartBoundSites(rule.BindSiteIDs)
	logger.WriteLog("info", "access", fmt.Sprintf("删除访问控制规则 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// restartBoundSites 重启规则绑定的所有运行中的 Caddy 站点
func (h *AccessHandler) restartBoundSites(bindSiteIDs string) {
	if bindSiteIDs == "" || h.caddyMgr == nil {
		return
	}
	var siteIDs []uint
	if err := json.Unmarshal([]byte(bindSiteIDs), &siteIDs); err != nil || len(siteIDs) == 0 {
		return
	}
	for _, siteID := range siteIDs {
		// 只重启正在运行的站点
		var site model.CaddySite
		if err := h.db.First(&site, siteID).Error; err != nil {
			continue
		}
		if site.Status == "running" && site.Enable {
			h.log.Infof("[访问控制] 规则变更，自动重启站点 [%s]", site.Name)
			h.caddyMgr.Restart(siteID)
		}
	}
}
