package api

import (
	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api/handlers"
	"github.com/netpanel/netpanel/api/middleware"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/service/access"
	"github.com/netpanel/netpanel/service/caddy"
	"github.com/netpanel/netpanel/service/firewall"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/netpanel/netpanel/service/cert"
	"github.com/netpanel/netpanel/service/cftunnel"
	"github.com/netpanel/netpanel/service/cron"
	"github.com/netpanel/netpanel/service/ddns"
	"github.com/netpanel/netpanel/service/dnsmasq"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/linereg"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/portforward"
	"github.com/netpanel/netpanel/service/storage"
	"github.com/netpanel/netpanel/service/stun"
	"github.com/netpanel/netpanel/service/syslog"
	"github.com/netpanel/netpanel/service/tunservice"
	"github.com/netpanel/netpanel/service/ai"
	"github.com/netpanel/netpanel/service/meshnode"
	"github.com/netpanel/netpanel/service/wireguard"
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
	CftunnelMgr    *cftunnel.Manager
	DdnsMgr        *ddns.Manager
	CaddyMgr       *caddy.Manager
	CronMgr        *cron.Manager
	StorageMgr     *storage.Manager
	AccessMgr      *access.Manager
	FirewallMgr    *firewall.Manager
	DnsmasqMgr     *dnsmasq.Manager
	WolMgr         *wol.Manager
	CertMgr        *cert.Manager
	CallbackMgr    *callback.Manager
	SyslogMgr      *syslog.Manager
	WireguardMgr   *wireguard.Manager
	MeshNodeMgr    *meshnode.Manager
	TunserviceMgr  *tunservice.Manager
	AiMgr          *ai.Manager
	LineregMgr     *linereg.Manager
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

	// 首次初始化（无管理员时强制建首个管理员）
	initHandler := handlers.NewInitHandler(opts.DB, opts.Log)
	apiV1.GET("/init/status", initHandler.Status)
	apiV1.POST("/init/setup", initHandler.Setup)

	// OAuth2/OIDC 公开路由
	oauthHandler := handlers.NewOAuthHandler(opts.DB, opts.Log)
	apiV1.GET("/auth/oauth/providers", oauthHandler.ListPublicProviders)
	apiV1.GET("/auth/oauth/:provider/authorize", oauthHandler.Authorize)
	apiV1.GET("/auth/oauth/:provider/callback", oauthHandler.Callback)

	// 需要认证的路由
	auth := apiV1.Group("")
	auth.Use(middleware.JWTAuth())

	// 需要管理员权限的路由（在 JWTAuth 之上叠加 AdminOnly）。
	// 用于系统管理接口，以及可在宿主机/受管主机执行命令的高危能力。
	admin := apiV1.Group("")
	admin.Use(middleware.JWTAuth(), middleware.AdminOnly())

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
	auth.GET("/port-forward/certs", pfHandler.ListCerts)

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
	auth.GET("/frpc/speedtest", frpcHandler.SpeedTest)
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
	// NPS 隧道（子表，参考 nps 隧道类型）
	auth.GET("/nps/client/:id/tunnels", npsClientHandler.ListTunnels)
	auth.POST("/nps/client/:id/tunnels", npsClientHandler.CreateTunnel)
	auth.PUT("/nps/client/:id/tunnels/:tid", npsClientHandler.UpdateTunnel)
	auth.DELETE("/nps/client/:id/tunnels/:tid", npsClientHandler.DeleteTunnel)

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
	auth.GET("/easytier/client/:id/logs", etHandler.GetLogs)
	auth.GET("/easytier/client/:id/peers", etHandler.GetPeers)

	// EasyTier 服务端
	etsHandler := handlers.NewEasytierServerHandler(opts.DB, opts.Log, opts.EasytierMgr)
	auth.GET("/easytier/server", etsHandler.List)
	auth.POST("/easytier/server", etsHandler.Create)
	auth.PUT("/easytier/server/:id", etsHandler.Update)
	auth.DELETE("/easytier/server/:id", etsHandler.Delete)
	auth.POST("/easytier/server/:id/start", etsHandler.Start)
	auth.POST("/easytier/server/:id/stop", etsHandler.Stop)
	auth.GET("/easytier/server/:id/logs", etsHandler.GetLogs)
	auth.GET("/easytier/server/:id/peers", etsHandler.GetPeers)

	// 穿透服务（用户视角的统一内网穿透管理）
	tsHandler := handlers.NewTunserviceHandler(opts.DB, opts.Log, opts.TunserviceMgr)
	auth.GET("/tunservice", tsHandler.List)
	auth.GET("/tunservice/:id", tsHandler.Get)
	auth.POST("/tunservice", tsHandler.Create)
	auth.PUT("/tunservice/:id", tsHandler.Update)
	auth.DELETE("/tunservice/:id", tsHandler.Delete)
	auth.POST("/tunservice/:id/start", tsHandler.Start)
	auth.POST("/tunservice/:id/stop", tsHandler.Stop)
	auth.GET("/tunservice/:id/candidates", tsHandler.Candidates)
	auth.GET("/tunservice/:id/history", tsHandler.History)
	auth.GET("/tunservice/:id/speedtest", tsHandler.Speedtest)

	// 线路探测策略（参数化配置）
	lineHandler := handlers.NewLineregHandler(opts.DB, opts.Log, opts.LineregMgr)
	auth.GET("/linereg/config", lineHandler.GetConfig)
	auth.PUT("/linereg/config", lineHandler.UpdateConfig)
	auth.GET("/linereg/rebind-pending", lineHandler.PendingRebinds)
	auth.POST("/linereg/rebind-apply", lineHandler.ApplyRebinds)

	// WireGuard
	wgHandler := handlers.NewWireguardHandler(opts.DB, opts.Log, opts.WireguardMgr)
	auth.GET("/wireguard", wgHandler.List)
	auth.POST("/wireguard", wgHandler.Create)
	auth.PUT("/wireguard/:id", wgHandler.Update)
	auth.DELETE("/wireguard/:id", wgHandler.Delete)
	auth.POST("/wireguard/:id/start", wgHandler.Start)
	auth.POST("/wireguard/:id/stop", wgHandler.Stop)
	auth.GET("/wireguard/:id/status", wgHandler.GetStatus)
	auth.POST("/wireguard/generate-keypair", wgHandler.GenerateKeyPair)
	// WireGuard 对等节点
	auth.GET("/wireguard/:id/peers", wgHandler.ListPeers)
	auth.POST("/wireguard/:id/peers", wgHandler.CreatePeer)
	auth.PUT("/wireguard/:id/peers/:pid", wgHandler.UpdatePeer)
	auth.DELETE("/wireguard/:id/peers/:pid", wgHandler.DeletePeer)
	auth.GET("/wireguard/:id/peers/:pid/config", wgHandler.GetPeerConfig)
	auth.GET("/wireguard/:id/peers/:pid/qrcode", wgHandler.GetPeerQRCode)

	// DDNS
	ddnsHandler := handlers.NewDDNSHandler(opts.DB, opts.Log, opts.DdnsMgr)
	auth.GET("/ddns", ddnsHandler.List)
	auth.POST("/ddns", ddnsHandler.Create)
	auth.PUT("/ddns/:id", ddnsHandler.Update)
	auth.DELETE("/ddns/:id", ddnsHandler.Delete)
	auth.POST("/ddns/:id/start", ddnsHandler.Start)
	auth.POST("/ddns/:id/stop", ddnsHandler.Stop)
	auth.POST("/ddns/:id/run", ddnsHandler.RunNow)
	auth.GET("/ddns/:id/history", ddnsHandler.GetHistory)

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
	auth.POST("/domain/accounts/:id/test", daHandler.Test)

	// 域名管理（域名列表，参考 dnsmgr domain 表）
	diHandler := handlers.NewDomainInfoHandler(opts.DB, opts.Log)
		auth.GET("/domain/domains", diHandler.List)
		auth.GET("/domain/domains/fetch", diHandler.FetchFromProvider)
		auth.POST("/domain/domains", diHandler.Create)
		auth.PUT("/domain/domains/:id", diHandler.Update)
		auth.DELETE("/domain/domains/:id", diHandler.Delete)
		auth.POST("/domain/domains/:id/refresh", diHandler.Refresh)
		auth.PUT("/domain/domains/:id/auto-sync", diHandler.UpdateAutoSync)

	// 证书账号（ACME CA 账号，参考 dnsmgr cert_account）
	certAccountHandler := handlers.NewCertAccountHandler(opts.DB, opts.Log, opts.CertMgr)
	auth.GET("/domain/cert-accounts", certAccountHandler.List)
	auth.POST("/domain/cert-accounts", certAccountHandler.Create)
	auth.PUT("/domain/cert-accounts/:id", certAccountHandler.Update)
	auth.DELETE("/domain/cert-accounts/:id", certAccountHandler.Delete)
	auth.POST("/domain/cert-accounts/:id/verify", certAccountHandler.Verify)

	// 域名证书
	certHandler := handlers.NewCertHandler(opts.DB, opts.Log, opts.Config, opts.CertMgr)
	auth.GET("/domain/certs", certHandler.List)
	auth.POST("/domain/certs", certHandler.Create)
	auth.PUT("/domain/certs/:id", certHandler.Update)
	auth.DELETE("/domain/certs/:id", certHandler.Delete)
	auth.POST("/domain/certs/:id/apply", certHandler.Apply)
	auth.POST("/domain/certs/:id/renew", certHandler.Renew)
	auth.GET("/domain/certs/:id/status", certHandler.GetStatus)
	auth.POST("/domain/certs/:id/step/create-order", certHandler.StepCreateOrder)
	auth.POST("/domain/certs/:id/step/set-dns", certHandler.StepSetDNS)
	auth.POST("/domain/certs/:id/step/validate", certHandler.StepValidate)
		auth.POST("/domain/certs/:id/step/obtain", certHandler.StepObtain)
		auth.POST("/domain/certs/:id/confirm-dns", certHandler.ConfirmDNS)

	// 域名解析（子域名解析记录，按域名ID查询）
	drHandler := handlers.NewDomainRecordHandler(opts.DB, opts.Log)
	auth.GET("/domain/records", drHandler.List)
	auth.POST("/domain/records", drHandler.Create)
	auth.PUT("/domain/records/:id", drHandler.Update)
	auth.DELETE("/domain/records/:id", drHandler.Delete)
	auth.POST("/domain/records/sync/:domainInfoId", drHandler.SyncFromProvider)

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

	// 注入 DNS 解析记录同步回调到计划任务管理器
	opts.CronMgr.SetSyncDNSRecordFunc(diHandler.DoSyncFromProvider)

	// 计划任务（写操作限管理员：shell 类型可在宿主机执行任意命令）
	cronHandler := handlers.NewCronHandler(opts.DB, opts.Log, opts.CronMgr)
	auth.GET("/cron", cronHandler.List)
	admin.POST("/cron", cronHandler.Create)
	admin.PUT("/cron/:id", cronHandler.Update)
	admin.DELETE("/cron/:id", cronHandler.Delete)
	admin.POST("/cron/:id/enable", cronHandler.Enable)
	admin.POST("/cron/:id/disable", cronHandler.Disable)
	admin.POST("/cron/:id/run", cronHandler.RunNow)

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
	auth.POST("/ipdb/import-url", ipdbHandler.ImportFromURL)
	auth.GET("/ipdb/query", ipdbHandler.Query)
	// IP 地址库订阅
	auth.GET("/ipdb/subscriptions", ipdbHandler.ListSubscriptions)
	auth.POST("/ipdb/subscriptions", ipdbHandler.CreateSubscription)
	auth.PUT("/ipdb/subscriptions/:id", ipdbHandler.UpdateSubscription)
	auth.DELETE("/ipdb/subscriptions/:id", ipdbHandler.DeleteSubscription)
	auth.POST("/ipdb/subscriptions/:id/refresh", ipdbHandler.RefreshSubscription)

	// CF 隧道（Cloudflare Tunnel，cloudflared）
	cftunnelHandler := handlers.NewCfTunnelHandler(opts.DB, opts.Log, opts.CftunnelMgr)
	auth.GET("/cftunnel", cftunnelHandler.List)
	auth.POST("/cftunnel", cftunnelHandler.Create)
	auth.PUT("/cftunnel/:id", cftunnelHandler.Update)
	auth.DELETE("/cftunnel/:id", cftunnelHandler.Delete)
	auth.POST("/cftunnel/:id/start", cftunnelHandler.Start)
	auth.POST("/cftunnel/:id/stop", cftunnelHandler.Stop)
	auth.GET("/cftunnel/:id/status", cftunnelHandler.GetStatus)
	auth.GET("/cftunnel/:id/logs", cftunnelHandler.GetLogs)
	// cloudflared 二进制管理（隧道启动强依赖该二进制）
	auth.GET("/cftunnel/binary", cftunnelHandler.GetBinaryPath)
	auth.GET("/cftunnel/download/info", cftunnelHandler.GetDownloadInfo)
	auth.POST("/cftunnel/download", cftunnelHandler.DownloadBinary)

	// 访问控制
	accessHandler := handlers.NewAccessHandler(opts.DB, opts.Log, opts.AccessMgr, opts.CaddyMgr)
	auth.GET("/access", accessHandler.List)
	auth.POST("/access", accessHandler.Create)
	auth.PUT("/access/:id", accessHandler.Update)
	auth.DELETE("/access/:id", accessHandler.Delete)

	// 系统防火墙（iptables/nftables/ufw/firewalld/Windows）
	firewallHandler := handlers.NewFirewallHandler(opts.DB, opts.Log, opts.FirewallMgr)
	auth.GET("/security/firewall", firewallHandler.List)
	auth.POST("/security/firewall", firewallHandler.Create)
	auth.PUT("/security/firewall/:id", firewallHandler.Update)
	auth.DELETE("/security/firewall/:id", firewallHandler.Delete)
	auth.POST("/security/firewall/:id/apply", firewallHandler.Apply)
	auth.POST("/security/firewall/:id/remove", firewallHandler.Remove)
	auth.GET("/security/firewall/backend", firewallHandler.DetectBackend)
	auth.POST("/security/firewall/sync-system", firewallHandler.SyncSystem)
	auth.GET("/security/firewall/sync-status", firewallHandler.GetSyncStatus)

	// WAF 防火墙（Coraza，参考 coraza WAF 和 lucky 安全模块）
	wafHandler := handlers.NewWafHandler(opts.DB, opts.Log, opts.FirewallMgr)
	// 读操作：任意认证用户可查看
	auth.GET("/security/waf", wafHandler.List)
	auth.GET("/security/waf/:id/logs", wafHandler.GetLogs)
	// 安全中心：攻击事件与态势统计
	auth.GET("/security/waf/events", wafHandler.EventList)
	auth.GET("/security/waf/stats", wafHandler.Stats)
	// 安全中心：封禁 / 黑白名单
	auth.GET("/security/waf/bans", wafHandler.BanList)

	// 写操作：可修改宿主防火墙/运行 WAF 规则，仅管理员
	admin.POST("/security/waf", wafHandler.Create)
	admin.PUT("/security/waf/:id", wafHandler.Update)
	admin.DELETE("/security/waf/:id", wafHandler.Delete)
	admin.POST("/security/waf/:id/start", wafHandler.Start)
	admin.POST("/security/waf/:id/stop", wafHandler.Stop)
	admin.POST("/security/waf/:id/test", wafHandler.TestRule)
	admin.POST("/security/waf/bans", wafHandler.BanCreate)
	admin.DELETE("/security/waf/bans/:id", wafHandler.BanDelete)
	admin.POST("/security/waf/bans/:id/apply", wafHandler.BanApply)
	admin.POST("/security/waf/bans/:id/remove", wafHandler.BanRemove)

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

	// ── 系统管理（仅管理员）────────────────────────────────────────────────────
	// 此前这些接口仅校验"是否登录"，任意普通用户可创建管理员账号实现提权。
	// 日志查看
	syslogHandler := handlers.NewSyslogHandler(opts.DB, opts.Log, opts.SyslogMgr)
	admin.GET("/admin/logs", syslogHandler.QueryLogs)
	admin.GET("/admin/logs/services", syslogHandler.GetLogServices)
	admin.DELETE("/admin/logs", syslogHandler.CleanupLogs)

	// 用户管理
	userHandler := handlers.NewUserHandler(opts.DB, opts.Log)
	admin.GET("/admin/users", userHandler.ListUsers)
	admin.POST("/admin/users", userHandler.CreateUser)
	admin.PUT("/admin/users/:id", userHandler.UpdateUser)
	admin.DELETE("/admin/users/:id", userHandler.DeleteUser)
	// 查询自身信息属于普通登录用户能力，不纳入 admin 组
	auth.GET("/admin/users/me", userHandler.GetCurrentUser)

	// OAuth Provider 管理
	admin.GET("/admin/oauth-providers", oauthHandler.ListProviders)
	admin.POST("/admin/oauth-providers", oauthHandler.CreateProvider)
	admin.PUT("/admin/oauth-providers/:id", oauthHandler.UpdateProvider)
	admin.DELETE("/admin/oauth-providers/:id", oauthHandler.DeleteProvider)

	// ── 组网节点管理 ──────────────────────────────────────────────────────────
	meshHandler := handlers.NewMeshNodeHandler(opts.DB, opts.Log, opts.MeshNodeMgr)
	auth.GET("/mesh/nodes", meshHandler.ListNodes)
	auth.POST("/mesh/nodes", meshHandler.CreateNode)
	auth.GET("/mesh/nodes/:id", meshHandler.GetNode)
	auth.PUT("/mesh/nodes/:id", meshHandler.UpdateNode)
	auth.DELETE("/mesh/nodes/:id", meshHandler.DeleteNode)
	auth.POST("/mesh/nodes/:id/check", meshHandler.CheckNode)
	auth.GET("/mesh/topology", meshHandler.GetTopology)
	auth.GET("/mesh/events", meshHandler.ListEvents)
	auth.DELETE("/mesh/events", meshHandler.CleanEvents)
	auth.POST("/mesh/ping", meshHandler.Ping)
	// 代理请求到远程节点
	auth.Any("/mesh/proxy/:nodeId/*path", meshHandler.ProxyToNode)

	// ── AI 管理 ────────────────────────────────────────────────────────────────
	aiHandler := handlers.NewAiHandler(opts.DB, opts.Log, opts.AiMgr)
	// API 来源
	auth.GET("/ai/providers", aiHandler.ListProviders)
	auth.POST("/ai/providers", aiHandler.CreateProvider)
	auth.PUT("/ai/providers/:id", aiHandler.UpdateProvider)
	auth.DELETE("/ai/providers/:id", aiHandler.DeleteProvider)
	auth.POST("/ai/providers/:id/fetch-models", aiHandler.FetchModels)
	auth.POST("/ai/providers/:id/test", aiHandler.TestProvider)
	// 对话
	auth.GET("/ai/conversations", aiHandler.ListConversations)
	auth.POST("/ai/conversations", aiHandler.CreateConversation)
	auth.PUT("/ai/conversations/:id", aiHandler.UpdateConversation)
	auth.DELETE("/ai/conversations/:id", aiHandler.DeleteConversation)
	auth.GET("/ai/conversations/:id/messages", aiHandler.ListMessages)
	auth.POST("/ai/conversations/:id/send", aiHandler.SendMessage)
	auth.POST("/ai/conversations/:id/stream", aiHandler.StreamMessage)
	auth.GET("/ai/conversations/:id/export", aiHandler.ExportConversation)
	auth.POST("/ai/conversations/import", aiHandler.ImportConversation)
	// AI 助理
	auth.GET("/ai/assistants", aiHandler.ListAssistants)
	auth.POST("/ai/assistants", aiHandler.CreateAssistant)
	auth.PUT("/ai/assistants/:id", aiHandler.UpdateAssistant)
	auth.DELETE("/ai/assistants/:id", aiHandler.DeleteAssistant)
	// AI 定时任务
	auth.GET("/ai/cron-tasks", aiHandler.ListCronTasks)
	auth.POST("/ai/cron-tasks", aiHandler.CreateCronTask)
	auth.PUT("/ai/cron-tasks/:id", aiHandler.UpdateCronTask)
	auth.DELETE("/ai/cron-tasks/:id", aiHandler.DeleteCronTask)
	auth.POST("/ai/cron-tasks/:id/enable", aiHandler.EnableCronTask)
	auth.POST("/ai/cron-tasks/:id/disable", aiHandler.DisableCronTask)
	auth.POST("/ai/cron-tasks/:id/run", aiHandler.RunCronTask)
	auth.GET("/ai/cron-tasks/:id/logs", aiHandler.ListCronLogs)
	// AI 插件
	auth.GET("/ai/plugins", aiHandler.ListPlugins)
	auth.POST("/ai/plugins", aiHandler.CreatePlugin)
	auth.PUT("/ai/plugins/:id", aiHandler.UpdatePlugin)
	auth.DELETE("/ai/plugins/:id", aiHandler.DeletePlugin)
	auth.POST("/ai/plugins/:id/toggle", aiHandler.TogglePlugin)

	// ── 服务监控 ────────────────────────────────────────────────────────────────
	monitorHandler := handlers.NewMonitorHandler(opts.DB, opts.FrpMgr, opts.NpsMgr, opts.EasytierMgr, opts.CftunnelMgr, opts.WireguardMgr)
	// 服务器管理
	auth.GET("/monitor/servers", monitorHandler.ListServers)
	auth.GET("/monitor/servers/:id", monitorHandler.GetServer)
	auth.POST("/monitor/servers", monitorHandler.CreateServer)
	auth.PUT("/monitor/servers/:id", monitorHandler.UpdateServer)
	auth.DELETE("/monitor/servers/:id", monitorHandler.DeleteServer)
	auth.POST("/monitor/servers/sync/:nodeId", monitorHandler.SyncFromMeshNode)
	// 监控指标
	auth.GET("/monitor/servers/:id/metrics/latest", monitorHandler.GetLatestMetrics)
	auth.GET("/monitor/servers/:id/metrics/history", monitorHandler.GetMetricsHistory)
	// 服务探测
	auth.GET("/monitor/probes", monitorHandler.ListProbes)
	auth.POST("/monitor/probes", monitorHandler.CreateProbe)
	auth.PUT("/monitor/probes/:id", monitorHandler.UpdateProbe)
	auth.DELETE("/monitor/probes/:id", monitorHandler.DeleteProbe)
	auth.GET("/monitor/probes/:id/results", monitorHandler.GetProbeResults)
	// 任务管理（写操作限管理员：可通过 SSH 在受管主机执行任意命令）
	auth.GET("/monitor/tasks", monitorHandler.ListTasks)
	admin.POST("/monitor/tasks", monitorHandler.CreateTask)
	admin.PUT("/monitor/tasks/:id", monitorHandler.UpdateTask)
	admin.DELETE("/monitor/tasks/:id", monitorHandler.DeleteTask)
	admin.POST("/monitor/tasks/:id/execute", monitorHandler.ExecuteTask)
	auth.GET("/monitor/tasks/logs", monitorHandler.GetTaskLogs)
	// 告警规则
	auth.GET("/monitor/alerts", monitorHandler.ListAlerts)
	auth.POST("/monitor/alerts", monitorHandler.CreateAlert)
	auth.PUT("/monitor/alerts/:id", monitorHandler.UpdateAlert)
	auth.DELETE("/monitor/alerts/:id", monitorHandler.DeleteAlert)
	auth.GET("/monitor/alerts/records", monitorHandler.GetAlertRecords)
	// DDNS 绑定
	auth.GET("/monitor/ddns", monitorHandler.GetDDNSBindings)
	auth.POST("/monitor/ddns", monitorHandler.CreateDDNSBinding)
	auth.PUT("/monitor/ddns/:id", monitorHandler.UpdateDDNSBinding)
	auth.DELETE("/monitor/ddns/:id", monitorHandler.DeleteDDNSBinding)
	auth.POST("/monitor/ddns/:id/trigger", monitorHandler.TriggerDDNSUpdate)
	// 通知渠道
	auth.GET("/monitor/notifications", monitorHandler.GetNotificationChannels)
	auth.POST("/monitor/notifications", monitorHandler.CreateNotificationChannel)
	auth.PUT("/monitor/notifications/:id", monitorHandler.UpdateNotificationChannel)
	auth.DELETE("/monitor/notifications/:id", monitorHandler.DeleteNotificationChannel)
	auth.POST("/monitor/notifications/test", monitorHandler.SendTestNotification)
	// 隧道绑定
	auth.GET("/monitor/tunnels", monitorHandler.GetTunnelBindings)
	auth.POST("/monitor/tunnels", monitorHandler.CreateTunnelBinding)
	auth.PUT("/monitor/tunnels/:id", monitorHandler.UpdateTunnelBinding)
	auth.DELETE("/monitor/tunnels/:id", monitorHandler.DeleteTunnelBinding)
	auth.POST("/monitor/tunnels/:id/sync", monitorHandler.SyncTunnelStatus)
// WebSocket 终端：浏览器 WebSocket 无法自定义请求头，token 经 query 传入，
// 由 HandleTerminal 内部完成鉴权（校验 token + 管理员权限 + Origin）
r.GET("/ws/terminal", monitorHandler.HandleTerminal)

	return r
}
