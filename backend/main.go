package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/pkg/logger"
	"github.com/netpanel/netpanel/service/access"
	"github.com/netpanel/netpanel/service/caddy"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/netpanel/netpanel/service/cert"
	"github.com/netpanel/netpanel/service/cron"
	"github.com/netpanel/netpanel/service/ddns"
	"github.com/netpanel/netpanel/service/dnsmasq"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/portforward"
	"github.com/netpanel/netpanel/service/storage"
	"github.com/netpanel/netpanel/service/stun"
	"github.com/netpanel/netpanel/service/wol"
)

//go:embed embed/dist
var staticFiles embed.FS

// Version 由构建时 ldflags 注入
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var (
	port    = flag.Int("port", 8080, "HTTP 监听端口")
	dataDir = flag.String("data", "./data", "数据目录")
)

func main() {
	flag.Parse()

	// 初始化日志
	log := logger.Init()
	log.Infof("NetPanel 启动中...")

	// 确保数据目录存在
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("创建数据目录失败: %v", err)
	}

	// 初始化配置
	cfg := config.Init(*dataDir)

	// 初始化数据库
	db, err := model.InitDB(*dataDir)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}

	// 初始化各服务管理器
	portforwardMgr := portforward.NewManager(db, log)
	stunMgr := stun.NewManager(db, log)
	frpMgr := frp.NewManager(db, log)
	npsMgr := nps.NewManager(db, log, *dataDir)
	easytierMgr := easytier.NewManager(db, log, *dataDir)
	ddnsMgr := ddns.NewManager(db, log)
	caddyMgr := caddy.NewManager(db, log, *dataDir)
	cronMgr := cron.NewManager(db, log)
	storageMgr := storage.NewManager(db, log, *dataDir)
	accessMgr := access.NewManager(db, log)
	dnsmasqMgr := dnsmasq.NewManager(db, log)
	wolMgr := wol.NewManager(db, log)
	certMgr := cert.NewManager(db, log, *dataDir)
	callbackMgr := callback.NewManager(db, log)

	// 启动所有已启用的服务
	portforwardMgr.StartAll()
	stunMgr.StartAll()
	frpMgr.StartAll()
	npsMgr.StartAll()
	easytierMgr.StartAll()
	ddnsMgr.StartAll()
	caddyMgr.StartAll()
	cronMgr.StartAll()
	storageMgr.StartAll()
	dnsmasqMgr.StartAll()
	certMgr.StartAll()
	callbackMgr.Start()

	// 设置 Gin 模式
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化路由
	router := api.NewRouter(api.RouterOptions{
		DB:             db,
		Log:            log,
		Config:         cfg,
		PortForwardMgr: portforwardMgr,
		StunMgr:        stunMgr,
		FrpMgr:         frpMgr,
		NpsMgr:         npsMgr,
		EasytierMgr:    easytierMgr,
		DdnsMgr:        ddnsMgr,
		CaddyMgr:       caddyMgr,
		CronMgr:        cronMgr,
		StorageMgr:     storageMgr,
		AccessMgr:      accessMgr,
		DnsmasqMgr:     dnsmasqMgr,
		WolMgr:         wolMgr,
		CertMgr:        certMgr,
		CallbackMgr:    callbackMgr,
	})

	// 挂载前端静态文件（SPA 模式：所有非 /api 路径均返回 index.html）
	distFS, fsErr := fs.Sub(staticFiles, "embed/dist")
	if fsErr != nil {
		log.Warnf("前端静态文件加载失败（开发模式）: %v", fsErr)
	} else {
		fileServer := http.FileServer(http.FS(distFS))
		router.NoRoute(func(c *gin.Context) {
			// API 路由未匹配时返回 404 JSON
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "接口不存在"})
				return
			}
			// 尝试提供静态文件，不存在则返回 index.html（SPA 路由）
			filePath := c.Request.URL.Path
			if filePath == "/" || filePath == "" {
				filePath = "index.html"
			} else {
				filePath = filePath[1:] // 去掉前导 /
			}
			if _, openErr := distFS.Open(filePath); openErr != nil {
				c.Request.URL.Path = "/"
			}
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	// 访问控制中间件注入
	accessMgr.SetGinEngine(router)

	addr := fmt.Sprintf(":%d", *port)
	srv := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// 优雅关闭
	go func() {
		log.Infof("NetPanel 已启动，监听 http://0.0.0.0%s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("正在关闭 NetPanel...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 停止所有服务
	portforwardMgr.StopAll()
	stunMgr.StopAll()
	frpMgr.StopAll()
	npsMgr.StopAll()
	easytierMgr.StopAll()
	ddnsMgr.StopAll()
	caddyMgr.StopAll()
	cronMgr.StopAll()
	storageMgr.StopAll()
	dnsmasqMgr.StopAll()
	callbackMgr.Stop()

	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("HTTP 服务关闭出错: %v", err)
	}

	log.Info("NetPanel 已关闭")
}
