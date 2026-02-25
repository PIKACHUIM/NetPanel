package api

import (
	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api/handlers"
	"github.com/netpanel/netpanel/api/middleware"
	"github.com/netpanel/netpanel/pkg/config"
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

// RouterOptions 路由选项
type RouterOptions struct {
	DB             *gorm.DB
	Log            *logrus.Logger
	Config         *config.Config
	PortForwardMgr *portforward.Manager
	StunMgr        *stun.Manager
	FrpMgr         *frp.Manager
	NpsMgr         *nps.Manager
	EasytierMgr    *easytier.Manager
	DdnsMgr        *ddns.Manager
	CaddyMgr       *caddy.Manager
	CronMgr        *cron.Manager
	StorageMgr     *storage.Manager
	AccessMgr      *access.Manager
	DnsmasqMgr     *dnsmasq.Manager
	WolMgr         *wol.Manager
	CertMgr        *cert.Manager
	CallbackMgr    *callback.Manager
}

// NewRouter 创建路由
func NewRouter(opts RouterOptions) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// API 路由组
	apiV1 := r.Group("/api/v1")

	// 公开路由（无需认证）
	authHandler := handlers.NewAuthHandler(opts.DB, opts.Log)
	apiV1.POST("/auth/login", authHandler.Login)
	apiV1.POST("/auth/logout", authHandler.Logout)

	// 需要认证的路由
	auth := apiV1.Group("")
	auth.Use(middleware.JWTAuth())

	// 系统信息
	sysHandler := handlers.NewSystemHandler(opts.DB, opts.Log, opts.Config)
	auth.GET("/system/info", sysHandler.GetInfo)
	auth.GET("/system/stats", sysHandler.GetStats)
	auth.GET("/system/config", sysHandler.GetConfig)
	auth.PUT("/system/config", sysHandler.UpdateConfig)
	auth.GET("/system/interfaces", sysHandler.GetInterfaces)
	auth.POST("/system/change-password", sysHandler.ChangePassword)

	// 端口转发（路径与前端保持一致）
	pfHandler := handlers.NewPortForwardHandler(opts.DB, opts.Log, opts.PortForwardMgr)
	auth.GET("/port-forward", pfHandler.List)
	auth.POST("/port-forward", pfHandler.Create)
	auth.PUT("/port-forward/:id", pfHandler.Update)
	auth.DELETE("/port-forward/:id", pfHandler.Delete)
	auth.POST("/port-forward/:id/start", pfHandler.Start)
	auth.POST("/port-forward/:id/stop", pfHandler.Stop)
	auth.GET("/port-forward/:id/logs", pfHandler.GetLogs)

	// STUN 穿透
	stunHandler := handlers.NewStunHandler(opts.DB, opts.Log, opts.StunMgr)
	auth.GET("/stun", stunHandler.List)
	auth.POST("/stun", stunHandler.Create)
	auth.PUT("/stun/:id", stunHandler.Update)
	auth.DELETE("/stun/:id", stunHandler.Delete)
	auth.POST("/stun/:id/start", stunHandler.Start)
	auth.POST("/stun/:id/stop", stunHandler.Stop)
	auth.GET("/stun/:id/status", stunHandler.GetStatus)

	// FRP 客户端
	frpcHandler := handlers.NewFrpcHandler(opts.DB, opts.Log, opts.FrpMgr)
	auth.GET("/frpc", frpcHandler.List)
	auth.POST("/frpc", frpcHandler.Create)
	auth.PUT("/frpc/:id", frpcHandler.Update)
	auth.DELETE("/frpc/:id", frpcHandler.Delete)
	auth.POST("/frpc/:id/start", frpcHandler.Start)
	auth.POST("/frpc/:id/stop", frpcHandler.Stop)
	auth.POST("/frpc/:id/restart", frpcHandler.Restart)
	// FRP 代理
	auth.GET("/frpc/:id/proxies", frpcHandler.ListProxies)
	auth.POST("/frpc/:id/proxies", frpcHandler.CreateProxy)
	auth.PUT("/frpc/:id/proxies/:pid", frpcHandler.UpdateProxy)
	auth.DELETE("/frpc/:id/proxies/:pid", frpcHandler.DeleteProxy)

	// FRP 服务端
	frpsHandler := handlers.NewFrpsHandler(opts.DB, opts.Log, opts.FrpMgr)
	auth.GET("/frps", frpsHandler.List)
	auth.POST("/frps", frpsHandler.Create)
	auth.PUT("/frps/:id", frpsHandler.Update)
	auth.DELETE("/frps/:id", frpsHandler.Delete)
	auth.POST("/frps/:id/start", frpsHandler.Start)
	auth.POST("/frps/:id/stop", frpsHandler.Stop)

	// NPS 服务端
	npsServerHandler := handlers.NewNpsServerHandler(opts.DB, opts.Log, opts.NpsMgr)
	auth.GET("/nps/server", npsServerHandler.List)
	auth.POST("/nps/server", npsServerHandler.Create)
	auth.PUT("/nps/server/:id", npsServerHandler.Update)
	auth.DELETE("/nps/server/:id", npsServerHandler.Delete)
	auth.POST("/nps/server/:id/start", npsServerHandler.Start)
	auth.POST("/nps/server/:id/stop", npsServerHandler.Stop)

	// NPS 客户端
	npsClientHandler := handlers.NewNpsClientHandler(opts.DB, opts.Log, opts.NpsMgr)
	auth.GET("/nps/client", npsClientHandler.List)
	auth.POST("/nps/client", npsClientHandler.Create)
	auth.PUT("/nps/client/:id", npsClientHandler.Update)
	auth.DELETE("/nps/client/:id", npsClientHandler.Delete)
	auth.POST("/nps/client/:id/start", npsClientHandler.Start)
	auth.POST("/nps/client/:id/stop", npsClientHandler.Stop)

	// EasyTier 客户端
	auth.GET("/frps/:id/dashboard", frpsHandler.GetDashboardURL)

	// EasyTier 客户端
	etHandler := handlers.NewEasytierHandler(opts.DB, opts.Log, opts.EasytierMgr)
	auth.GET("/easytier/client", etHandler.List)
	auth.POST("/easytier/client", etHandler.Create)
	auth.PUT("/easytier/client/:id", etHandler.Update)
	auth.DELETE("/easytier/client/:id", etHandler.Delete)
	auth.POST("/easytier/client/:id/start", etHandler.Start)
	auth.POST("/easytier/client/:id/stop", etHandler.Stop)
	auth.GET("/easytier/client/:id/status", etHandler.GetStatus)

	// EasyTier 服务端
	etsHandler := handlers.NewEasytierServerHandler(opts.DB, opts.Log, opts.EasytierMgr)
	auth.GET("/easytier/server", etsHandler.List)
	auth.POST("/easytier/server", etsHandler.Create)
	auth.PUT("/easytier/server/:id", etsHandler.Update)
	auth.DELETE("/easytier/server/:id", etsHandler.Delete)
	auth.POST("/easytier/server/:id/start", etsHandler.Start)
	auth.POST("/easytier/server/:id/stop", etsHandler.Stop)

	// DDNS
	ddnsHandler := handlers.NewDDNSHandler(opts.DB, opts.Log, opts.DdnsMgr)
	auth.GET("/ddns", ddnsHandler.List)
	auth.POST("/ddns", ddnsHandler.Create)
	auth.PUT("/ddns/:id", ddnsHandler.Update)
	auth.DELETE("/ddns/:id", ddnsHandler.Delete)
	auth.POST("/ddns/:id/start", ddnsHandler.Start)
	auth.POST("/ddns/:id/stop", ddnsHandler.Stop)
	auth.POST("/ddns/:id/run", ddnsHandler.RunNow)

	// Caddy 网站服务
	caddyHandler := handlers.NewCaddyHandler(opts.DB, opts.Log, opts.CaddyMgr)
	auth.GET("/caddy", caddyHandler.List)
	auth.POST("/caddy", caddyHandler.Create)
	auth.PUT("/caddy/:id", caddyHandler.Update)
	auth.DELETE("/caddy/:id", caddyHandler.Delete)
	auth.POST("/caddy/:id/start", caddyHandler.Start)
	auth.POST("/caddy/:id/stop", caddyHandler.Stop)

	// WOL 网络唤醒
	wolHandler := handlers.NewWolHandler(opts.DB, opts.Log)
	auth.GET("/wol", wolHandler.List)
	auth.POST("/wol", wolHandler.Create)
	auth.PUT("/wol/:id", wolHandler.Update)
	auth.DELETE("/wol/:id", wolHandler.Delete)
	auth.POST("/wol/:id/wake", wolHandler.Wake)

	// 域名账号
	daHandler := handlers.NewDomainAccountHandler(opts.DB, opts.Log)
	auth.GET("/domain/accounts", daHandler.List)
	auth.POST("/domain/accounts", daHandler.Create)
	auth.PUT("/domain/accounts/:id", daHandler.Update)
	auth.DELETE("/domain/accounts/:id", daHandler.Delete)

	// 域名证书
	certHandler := handlers.NewCertHandler(opts.DB, opts.Log, opts.Config)
	auth.GET("/domain/certs", certHandler.List)
	auth.POST("/domain/certs", certHandler.Create)
	auth.PUT("/domain/certs/:id", certHandler.Update)
	auth.DELETE("/domain/certs/:id", certHandler.Delete)
	auth.POST("/domain/certs/:id/apply", certHandler.Renew)

	// 域名解析
	drHandler := handlers.NewDomainRecordHandler(opts.DB, opts.Log)
	auth.GET("/domain/records", drHandler.List)
	auth.POST("/domain/records", drHandler.Create)
	auth.PUT("/domain/records/:id", drHandler.Update)
	auth.DELETE("/domain/records/:id", drHandler.Delete)
	auth.POST("/domain/records/sync/:accountId", drHandler.SyncFromProvider)

	// DNSMasq
	dnsmasqHandler := handlers.NewDnsmasqHandler(opts.DB, opts.Log, opts.DnsmasqMgr)
	auth.GET("/dnsmasq/config", dnsmasqHandler.GetConfig)
	auth.PUT("/dnsmasq/config", dnsmasqHandler.UpdateConfig)
	auth.POST("/dnsmasq/start", dnsmasqHandler.Start)
	auth.POST("/dnsmasq/stop", dnsmasqHandler.Stop)
	auth.GET("/dnsmasq/records", dnsmasqHandler.ListRecords)
	auth.POST("/dnsmasq/records", dnsmasqHandler.CreateRecord)
	auth.PUT("/dnsmasq/records/:id", dnsmasqHandler.UpdateRecord)
	auth.DELETE("/dnsmasq/records/:id", dnsmasqHandler.DeleteRecord)

	// 计划任务
	cronHandler := handlers.NewCronHandler(opts.DB, opts.Log, opts.CronMgr)
	auth.GET("/cron", cronHandler.List)
	auth.POST("/cron", cronHandler.Create)
	auth.PUT("/cron/:id", cronHandler.Update)
	auth.DELETE("/cron/:id", cronHandler.Delete)
	auth.POST("/cron/:id/enable", cronHandler.Enable)
	auth.POST("/cron/:id/disable", cronHandler.Disable)
	auth.POST("/cron/:id/run", cronHandler.RunNow)

	// 网络存储
	storageHandler := handlers.NewStorageHandler(opts.DB, opts.Log, opts.StorageMgr)
	auth.GET("/storage", storageHandler.List)
	auth.POST("/storage", storageHandler.Create)
	auth.PUT("/storage/:id", storageHandler.Update)
	auth.DELETE("/storage/:id", storageHandler.Delete)
	auth.POST("/storage/:id/start", storageHandler.Start)
	auth.POST("/storage/:id/stop", storageHandler.Stop)

	// IP 地址库
	ipdbHandler := handlers.NewIPDBHandler(opts.DB, opts.Log)
	auth.GET("/ipdb", ipdbHandler.List)
	auth.POST("/ipdb", ipdbHandler.Create)
	auth.PUT("/ipdb/:id", ipdbHandler.Update)
	auth.DELETE("/ipdb/:id", ipdbHandler.Delete)
	auth.POST("/ipdb/import", ipdbHandler.Import)
	auth.POST("/ipdb/query", ipdbHandler.Query)

	// 访问控制
	accessHandler := handlers.NewAccessHandler(opts.DB, opts.Log, opts.AccessMgr)
	auth.GET("/access", accessHandler.List)
	auth.POST("/access", accessHandler.Create)
	auth.PUT("/access/:id", accessHandler.Update)
	auth.DELETE("/access/:id", accessHandler.Delete)

	// 回调账号
	cbAccountHandler := handlers.NewCallbackAccountHandler(opts.DB, opts.Log, opts.CallbackMgr)
	auth.GET("/callback/accounts", cbAccountHandler.List)
	auth.POST("/callback/accounts", cbAccountHandler.Create)
	auth.PUT("/callback/accounts/:id", cbAccountHandler.Update)
	auth.DELETE("/callback/accounts/:id", cbAccountHandler.Delete)
	auth.POST("/callback/accounts/:id/test", cbAccountHandler.Test)

	// 回调任务
	cbTaskHandler := handlers.NewCallbackTaskHandler(opts.DB, opts.Log)
	auth.GET("/callback/tasks", cbTaskHandler.List)
	auth.POST("/callback/tasks", cbTaskHandler.Create)
	auth.PUT("/callback/tasks/:id", cbTaskHandler.Update)
	auth.DELETE("/callback/tasks/:id", cbTaskHandler.Delete)

	return r
}
