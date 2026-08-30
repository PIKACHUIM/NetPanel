package handlers

import (
	"fmt"
	"net"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

// ===== FRP 客户端 =====

type FrpcHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *frp.Manager

	// SpeedTest IP 级限流：10 次/分钟，防止接口被滥用为端口扫描
	speedTestMu       sync.Mutex
	speedTestLimiters map[string]*speedTestLimiterEntry
}

type speedTestLimiterEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

func NewFrpcHandler(db *gorm.DB, log *logrus.Logger, mgr *frp.Manager) *FrpcHandler {
	return &FrpcHandler{
		db:                db,
		log:               log,
		mgr:               mgr,
		speedTestLimiters: make(map[string]*speedTestLimiterEntry),
	}
}

// getSpeedTestLimiter 返回指定客户端 IP 的限流器，同时惰性清理 10 分钟未使用的条目。
func (h *FrpcHandler) getSpeedTestLimiter(clientIP string) *rate.Limiter {
	h.speedTestMu.Lock()
	defer h.speedTestMu.Unlock()

	if entry, ok := h.speedTestLimiters[clientIP]; ok {
		entry.lastUsed = time.Now()
		return entry.limiter
	}

	limiter := rate.NewLimiter(rate.Every(time.Minute/10), 1)
	h.speedTestLimiters[clientIP] = &speedTestLimiterEntry{
		limiter:  limiter,
		lastUsed: time.Now(),
	}

	// 惰性清理过期条目，避免内存无限增长
	now := time.Now()
	for ip, entry := range h.speedTestLimiters {
		if now.Sub(entry.lastUsed) > 10*time.Minute {
			delete(h.speedTestLimiters, ip)
		}
	}

	return limiter
}

func (h *FrpcHandler) List(c *gin.Context) {
	var configs []model.FrpcConfig
	h.db.Preload("Proxies").Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetClientStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *FrpcHandler) Create(c *gin.Context) {
	var cfg model.FrpcConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	logger.WriteLog("info", "frp", fmt.Sprintf("创建FRP客户端 [%d] %s", cfg.ID, cfg.Name))
	if cfg.Enable {
		h.mgr.StartClient(cfg.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *FrpcHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.FrpcConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopClient(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "frp", fmt.Sprintf("修改FRP客户端 [%d] %s", id, req.Name))
	if req.Enable {
		h.mgr.StartClient(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *FrpcHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Where("frpc_id = ?", id).Delete(&model.FrpcProxy{})
	h.db.Delete(&model.FrpcConfig{}, id)
	logger.WriteLog("info", "frp", fmt.Sprintf("删除FRP客户端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *FrpcHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartClient(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "frp", fmt.Sprintf("启动FRP客户端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *FrpcHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopClient(uint(id))
	h.db.Model(&model.FrpcConfig{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "frp", fmt.Sprintf("停止FRP客户端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// Restart 重启 FRP 客户端
func (h *FrpcHandler) Restart(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.RestartClient(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	logger.WriteLog("info", "frp", fmt.Sprintf("重启FRP客户端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已重启"})
}

func (h *FrpcHandler) ListProxies(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var proxies []model.FrpcProxy
	h.db.Where("frpc_id = ?", id).Find(&proxies)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": proxies})
}

func (h *FrpcHandler) CreateProxy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var proxy model.FrpcProxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	proxy.FrpcID = uint(id)
	h.db.Create(&proxy)
	logger.WriteLog("info", "frp", fmt.Sprintf("创建FRP代理 [%d] 客户端[%d] %s", proxy.ID, id, proxy.Name))
	// 重启客户端以应用新代理
	h.mgr.RestartClient(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": proxy, "message": "创建成功"})
}

func (h *FrpcHandler) UpdateProxy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	var proxy model.FrpcProxy
	if err := c.ShouldBindJSON(&proxy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	proxy.ID = uint(pid)
	proxy.FrpcID = uint(id)
	h.db.Save(&proxy)
	logger.WriteLog("info", "frp", fmt.Sprintf("修改FRP代理 [%d] 客户端[%d] %s", pid, id, proxy.Name))
	h.mgr.RestartClient(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": proxy, "message": "更新成功"})
}

func (h *FrpcHandler) DeleteProxy(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	pid, _ := strconv.ParseUint(c.Param("pid"), 10, 64)
	h.db.Delete(&model.FrpcProxy{}, pid)
	logger.WriteLog("info", "frp", fmt.Sprintf("删除FRP代理 [%d] 客户端[%d]", pid, id))
	h.mgr.RestartClient(uint(id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// speedTestTarget 测速目标（去重后的 FRP 服务端地址）
type speedTestTarget struct {
	ID         uint
	Name       string
	ServerAddr string
	ServerPort int
}

// SpeedTest 线路测速：测量所有 FRP 客户端指向的服务端 TCP 握手延迟
func (h *FrpcHandler) SpeedTest(c *gin.Context) {
	// IP 级限流（10 次/分钟），防止接口被滥用为端口扫描
	clientIP := c.ClientIP()
	if !h.getSpeedTestLimiter(clientIP).Allow() {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"code":  429,
			"error": "rate limit exceeded (max 10/min)",
		})
		return
	}

	var configs []model.FrpcConfig
	h.db.Select("id, name, server_addr, server_port").Find(&configs)

	// 按 (server_addr, server_port) 去重，多个客户端可能指向同一台服务端
	seen := make(map[string]struct{})
	var targets []speedTestTarget
	for _, cfg := range configs {
		key := fmt.Sprintf("%s:%d", cfg.ServerAddr, cfg.ServerPort)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, speedTestTarget{cfg.ID, cfg.Name, cfg.ServerAddr, cfg.ServerPort})
	}

	if len(targets) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": []gin.H{}})
		return
	}

	timeout := 5 * time.Second
	results := make([]gin.H, len(targets))
	var wg sync.WaitGroup
	// 并发上限：目标服务端较多时避免不受控地同时建立大量 TCP 连接
	sem := make(chan struct{}, 8)
	for i, t := range targets {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, t speedTestTarget) {
			defer wg.Done()
			defer func() { <-sem }()
			addr := net.JoinHostPort(t.ServerAddr, strconv.Itoa(t.ServerPort))
			start := time.Now()
			conn, err := net.DialTimeout("tcp", addr, timeout)
			if err != nil {
				results[i] = gin.H{
					"id": t.ID, "name": t.Name,
					"server_addr": t.ServerAddr, "server_port": t.ServerPort,
					"latency_ms": int64(0), "status": "unreachable", "error": err.Error(),
				}
				return
			}
			conn.Close()
			results[i] = gin.H{
				"id": t.ID, "name": t.Name,
				"server_addr": t.ServerAddr, "server_port": t.ServerPort,
				"latency_ms": time.Since(start).Milliseconds(), "status": "ok",
			}
		}(i, t)
	}
	wg.Wait()

	// 按延迟升序排序，不可达的排最后
	sort.Slice(results, func(i, j int) bool {
		si := results[i]["status"].(string)
		sj := results[j]["status"].(string)
		if si != sj {
			return si == "ok"
		}
		return results[i]["latency_ms"].(int64) < results[j]["latency_ms"].(int64)
	})

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": results})
}

// ===== FRP 服务端 =====

type FrpsHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *frp.Manager
}

func NewFrpsHandler(db *gorm.DB, log *logrus.Logger, mgr *frp.Manager) *FrpsHandler {
	return &FrpsHandler{db: db, log: log, mgr: mgr}
}

func (h *FrpsHandler) List(c *gin.Context) {
	var configs []model.FrpsConfig
	h.db.Order("id desc").Find(&configs)
	for i := range configs {
		configs[i].Status = h.mgr.GetServerStatus(configs[i].ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": configs})
}

func (h *FrpsHandler) Create(c *gin.Context) {
	var cfg model.FrpsConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cfg.Status = "stopped"
	h.db.Create(&cfg)
	logger.WriteLog("info", "frp", fmt.Sprintf("创建FRP服务端 [%d] %s", cfg.ID, cfg.Name))
	if cfg.Enable {
		h.mgr.StartServer(cfg.ID)
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cfg, "message": "创建成功"})
}

func (h *FrpsHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.FrpsConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.mgr.StopServer(uint(id))
	req.ID = uint(id)
	h.db.Save(&req)
	logger.WriteLog("info", "frp", fmt.Sprintf("修改FRP服务端 [%d] %s", id, req.Name))
	if req.Enable {
		h.mgr.StartServer(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *FrpsHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Delete(&model.FrpsConfig{}, id)
	logger.WriteLog("info", "frp", fmt.Sprintf("删除FRP服务端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *FrpsHandler) Start(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.StartServer(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	h.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("enable", true)
	logger.WriteLog("info", "frp", fmt.Sprintf("启动FRP服务端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已启动"})
}

func (h *FrpsHandler) Stop(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.mgr.StopServer(uint(id))
	h.db.Model(&model.FrpsConfig{}).Where("id = ?", id).Update("enable", false)
	logger.WriteLog("info", "frp", fmt.Sprintf("停止FRP服务端 [%d]", id))
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "已停止"})
}

// GetDashboardURL 返回 frps Dashboard 的访问地址
func (h *FrpsHandler) GetDashboardURL(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var cfg model.FrpsConfig
	if err := h.db.First(&cfg, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "配置不存在"})
		return
	}

	if cfg.DashboardPort == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "未配置 Dashboard 端口"})
		return
	}

	status := h.mgr.GetServerStatus(uint(id))
	if status != "running" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "FRP 服务端未运行"})
		return
	}

	addr := cfg.DashboardAddr
	if addr == "" || addr == "0.0.0.0" {
		// 监听在所有网卡时，使用请求来源的 Host（去掉端口部分）
		host := c.Request.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			addr = h
		} else {
			addr = host
		}
	}

	url := fmt.Sprintf("http://%s:%d", addr, cfg.DashboardPort)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{"url": url}})
}
