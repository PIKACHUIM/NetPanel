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
	"github.com/netpanel/netpanel/pkg/svcutil"
	"github.com/netpanel/netpanel/pkg/sysutil"
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
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

//go:embed embed/dist
var staticFiles embed.FS

// Version 由构建时 ldflags 注入
var (
	Version   = "dev"
	BuildTime = "unknown"
)

var (
	port             = flag.Int("port", 8080, "HTTP 监听端口")
	dataDir          = flag.String("data", "./data", "数据目录")
	serviceMode      = flag.Bool("service", false, "以系统服务模式运行（由 SCM/systemd 调用，勿手动使用）")
	installService   = flag.Bool("install-service", false, "注册系统服务（需要管理员/root 权限）")
	uninstallService = flag.Bool("uninstall-service", false, "卸载系统服务（需要管理员/root 权限）")
)

func main() {
	flag.Parse()

	// ── 服务管理命令（install / uninstall）──────────────────────────────
	if *installService {
		if err := svcutil.InstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 注册服务失败: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *uninstallService {
		if err := svcutil.UninstallService(); err != nil {
			fmt.Fprintf(os.Stderr, "❌ 卸载服务失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// ── 以 Windows Service 模式运行 ────────────────────────────────────
	if svcutil.IsWindowsService() || *serviceMode {
		var srv *http.Server
		runFn := func() { srv = startServer() }
		stopFn := func() {
			// 先停止所有子服务
			if stopAllFn != nil {
				stopAllFn()
			}
			if srv != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
			}
		}
		if err := svcutil.RunService(runFn, stopFn); err != nil {
			fmt.Fprintf(os.Stderr, "服务运行失败: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// ── 普通前台运行 ───────────────────────────────────────────────────
	srv := startServer()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log := logger.Init()
	log.Info("正在关闭 NetPanel...")

	// 停止所有子服务
	if stopAllFn != nil {
		stopAllFn()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Errorf("HTTP 服务关闭出错: %v", err)
	}
	log.Info("NetPanel 已关闭")
}

// startServer 初始化所有服务并启动 HTTP 服务器，返回 *http.Server 供优雅关闭使用。
func startServer() *http.Server {
	log := logger.Init()
	log.Infof("NetPanel %s 启动中...", Version)

	// ── 管理员权限检测 ─────────────────────────────────────────────────
	isAdmin := sysutil.IsAdmin()
	if !isAdmin {
		log.Warn("⚠️  当前进程未以管理员/root 权限运行")
		log.Warn("   EasyTier 客户端将强制使用 --no-tun 模式（TUN 网卡需要管理员权限）")
	}

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

	// 非管理员时：将数据库中所有 EasyTier 客户端/服务端的 no_tun 强制置为 true
	if !isAdmin {
		applyNoTunFallback(db, log)
	}

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
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "接口不存在"})
				return
			}
			filePath := c.Request.URL.Path
			if filePath == "/" || filePath == "" {
				filePath = "index.html"
			} else {
				filePath = filePath[1:]
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

	go func() {
		log.Infof("[系统核心] NetPanel 已启动，监听 http://0.0.0.0%s", addr)
		if !isAdmin {
			log.Infof("[系统核心] 运行模式：非管理员（EasyTier 已降级为 --no-tun）")
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务启动失败: %v", err)
		}
	}()

	// 注册停止回调（用于 service 模式的优雅关闭）
	registerStopHandlers(log, portforwardMgr, stunMgr, frpMgr, npsMgr,
		easytierMgr, ddnsMgr, caddyMgr, cronMgr, storageMgr, dnsmasqMgr, callbackMgr)

	return srv
}

// applyNoTunFallback 在非管理员模式下，将数据库中所有 EasyTier 实例的 no_tun 字段置为 true，
// 避免 easytier-core 因无权创建 TUN 网卡而崩溃。
// 注意：此函数仅修改数据库标记，实际生效依赖 easytier manager 在 StartClient/StartServer 时读取最新配置。
func applyNoTunFallback(db *gorm.DB, log *logrus.Logger) {
	log.Info("[EasyTier] 非管理员模式：强制所有实例使用 --no-tun")

	if err := db.Model(&model.EasytierClient{}).Where("enable = ?", true).Update("no_tun", true).Error; err != nil {
		log.Warnf("[EasyTier] 更新客户端 no_tun 失败: %v", err)
	}
	if err := db.Model(&model.EasytierServer{}).Where("enable = ?", true).Update("no_tun", true).Error; err != nil {
		log.Warnf("[EasyTier] 更新服务端 no_tun 失败: %v", err)
	}
}

// stopAllFn 保存所有子服务的停止函数，供 service 模式和普通模式的优雅关闭使用。
var stopAllFn func()

func registerStopHandlers(
	log *logrus.Logger,
	portforwardMgr interface{ StopAll() },
	stunMgr interface{ StopAll() },
	frpMgr interface{ StopAll() },
	npsMgr interface{ StopAll() },
	easytierMgr interface{ StopAll() },
	ddnsMgr interface{ StopAll() },
	caddyMgr interface{ StopAll() },
	cronMgr interface{ StopAll() },
	storageMgr interface{ StopAll() },
	dnsmasqMgr interface{ StopAll() },
	callbackMgr interface{ Stop() },
) {
	stopAllFn = func() {
		log.Info("正在停止所有服务...")
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
		log.Info("所有服务已停止")
	}
}
