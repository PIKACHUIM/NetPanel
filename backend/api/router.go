package api

import (
	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/api/handlers"
	"github.com/netpanel/netpanel/api/middleware"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/service/access"
	"github.com/netpanel/netpanel/service/ai"
	"github.com/netpanel/netpanel/service/caddy"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/netpanel/netpanel/service/cert"
	"github.com/netpanel/netpanel/service/cftunnel"
	"github.com/netpanel/netpanel/service/cron"
	"github.com/netpanel/netpanel/service/ddns"
	"github.com/netpanel/netpanel/service/dnsmasq"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/firewall"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/linereg"
	"github.com/netpanel/netpanel/service/meshnode"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/portforward"
	"github.com/netpanel/netpanel/service/retention"
	"github.com/netpanel/netpanel/service/storage"
	"github.com/netpanel/netpanel/service/stun"
	"github.com/netpanel/netpanel/service/syslog"
	"github.com/netpanel/netpanel/service/tunservice"
	"github.com/netpanel/netpanel/service/wireguard"
	"github.com/netpanel/netpanel/service/wol"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// RouterOptions 路由选项
type RouterOptions struct {
	DB               *gorm.DB
	Log              *logrus.Logger
	Config           *config.Config
	PortForwardMgr   *portforward.Manager
	StunMgr          *stun.Manager
	FrpMgr           *frp.Manager
	NpsMgr           *nps.Manager
	EasytierMgr      *easytier.Manager
	CftunnelMgr      *cftunnel.Manager
	DdnsMgr          *ddns.Manager
	CaddyMgr         *caddy.Manager
	CronMgr          *cron.Manager
	StorageMgr       *storage.Manager
	AccessMgr        *access.Manager
	FirewallMgr      *firewall.Manager
	DnsmasqMgr       *dnsmasq.Manager
	WolMgr           *wol.Manager
	CertMgr          *cert.Manager
	CallbackMgr      *callback.Manager
	SyslogMgr        *syslog.Manager
	WireguardMgr     *wireguard.Manager
	MeshNodeMgr      *meshnode.Manager
	TunserviceMgr    *tunservice.Manager
	AiMgr            *ai.Manager
	RetentionCleaner *retention.Cleaner
	LineregMgr       *linereg.Manager
}

// NewRouter 创建路由
//
// 权限模型（重要）：
//   - auth 组：任意已登录用户可访问。仅保留「只读」与「对宿主/网络无实质影响」的能力
//     （查看列表、看日志、AI 对话、网络唤醒）。
//   - admin 组：系统管理接口，以及一切「可修改宿主状态 / 向公网暴露内网 / 持有凭据 /
//     决定安全策略」的写操作。这类能力此前散落在 auth 组，导致任意普通用户即可
//     创建整盘文件共享、修改宿主防火墙、建立公网隧道、篡改域名解析，属横向提权。
func NewRouter(opts RouterOptions) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())

	// 可信代理：默认不信任任何代理，使 c.ClientIP() 返回真实对端地址。
	// 若保持 gin 默认（信任 0.0.0.0/0），客户端可伪造 X-Forwarded-For 绕过
	// 登录失败限流与 IP 黑白名单。反向代理部署时通过 NETPANEL_TRUSTED_PROXIES
	// （逗号分隔 IP/CIDR）显式声明可信代理。
	if err := r.SetTrustedProxies(middleware.TrustedProxies()); err != nil {
		if opts.Log != nil {
			opts.Log.Warnf("[安全] NETPANEL_TRUSTED_PROXIES 配置无效，回退为不信任任何代理: %v", err)
		}
		_ = r.SetTrustedProxies(nil)
	}

	// 访问控制中间件必须在注册路由之前挂载：Gin 在注册路由时对中间件链做快照，
	// 路由注册完成后再调用 Use 不会作用于已注册路由（曾经因此导致 IP 黑白名单、
	// Basic Auth、页面登录对全部 /api/v1 接口完全失效）。
	if opts.AccessMgr != nil {
		r.Use(opts.AccessMgr.GinMiddleware())
	}

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
	auth.Use(middleware.JWTAuth(opts.DB))

	// 需要管理员权限的路由（在 JWTAuth 之上叠加 AdminOnly）。
	// 用于系统管理接口，以及可在宿主机/受管主机执行命令、修改宿主网络状态、
	// 持有第三方凭据或向公网暴露内网服务的高危能力。
	admin := apiV1.Group("")
	admin.Use(middleware.JWTAuth(opts.DB), middleware.AdminOnly())

	// 系统信息（配置读写与改密限管理员）
	sysHandler := handlers.NewSystemHandler(opts.DB, opts.Log, opts.Config, opts.RetentionCleaner)
	auth.GET("/system/info", sysHandler.GetInfo)
	auth.GET("/system/stats", sysHandler.GetStats)
	auth.GET("/system/interfaces", sysHandler.GetInterfaces)
	auth.GET("/system/health", sysHandler.GetHealth)
	// 破坏性操作：手动清理必须在 admin 组内执行，且前端二次确认
	auth.POST("/system/cleanup", sysHandler.CleanupRetention)
	auth.GET("/system/cleanup/estimate", sysHandler.EstimateRetention)
	auth.POST("/system/change-password", sysHandler.ChangePassword)
	admin.GET("/system/config", sysHandler.GetConfig)
	admin.PUT("/system/config", sysHandler.UpdateConfig)

	// 端口转发（创建规则即可暴露任意内网目标，属高危能力）
	pfHandler := handlers.NewPortForwardHandler(opts.DB, opts.Log, opts.PortForwardMgr)
	auth.GET("/port-forward", pfHandler.List)
	auth.GET("/port-forward/:id/logs", pfHandler.GetLogs)
	auth.GET("/port-forward/certs", pfHandler.ListCerts)
	admin.POST("/port-forward", pfHandler.Create)
	admin.PUT("/port-forward/:id", pfHandler.Update)
	admin.DELETE("/port-forward/:id", pfHandler.Delete)
	admin.POST("/port-forward/:id/start", pfHandler.Start)
	admin.POST("/port-forward/:id/stop", pfHandler.Stop)

	// STUN 穿透
	stunHandler := handlers.NewStunHandler(opts.DB, opts.Log, opts.StunMgr)
	auth.GET("/stun", stunHandler.List)
	auth.GET("/stun/:id/status", stunHandler.GetStatus)
	admin.POST("/stun", stunHandler.Create)
	admin.PUT("/stun/:id", stunHandler.Update)
	admin.DELETE("/stun/:id", stunHandler.Delete)
	admin.POST("/stun/:id/start", stunHandler.Start)
	admin.POST("/stun/:id/stop", stunHandler.Stop)

	// FRP 客户端
	frpcHandler := handlers.NewFrpcHandler(opts.DB, opts.Log, opts.FrpMgr)
	auth.GET("/frpc", frpcHandler.List)
	auth.GET("/frpc/:id/proxies", frpcHandler.ListProxies)
	admin.POST("/frpc", frpcHandler.Create)
	admin.PUT("/frpc/:id", frpcHandler.Update)
	admin.DELETE("/frpc/:id", frpcHandler.Delete)
	admin.POST("/frpc/:id/start", frpcHandler.Start)
	admin.POST("/frpc/:id/stop", frpcHandler.Stop)
	admin.POST("/frpc/:id/restart", frpcHandler.Restart)
	admin.GET("/frpc/speedtest", frpcHandler.SpeedTest)
	admin.POST("/frpc/:id/proxies", frpcHandler.CreateProxy)
	admin.PUT("/frpc/:id/proxies/:pid", frpcHandler.UpdateProxy)
	admin.DELETE("/frpc/:id/proxies/:pid", frpcHandler.DeleteProxy)

	// FRP 服务端
	frpsHandler := handlers.NewFrpsHandler(opts.DB, opts.Log, opts.FrpMgr)
	auth.GET("/frps", frpsHandler.List)
	auth.GET("/frps/:id/dashboard", frpsHandler.GetDashboardURL)
	admin.POST("/frps", frpsHandler.Create)
	admin.PUT("/frps/:id", frpsHandler.Update)
	admin.DELETE("/frps/:id", frpsHandler.Delete)
	admin.POST("/frps/:id/start", frpsHandler.Start)
	admin.POST("/frps/:id/stop", frpsHandler.Stop)

	// NPS 服务端
	npsServerHandler := handlers.NewNpsServerHandler(opts.DB, opts.Log, opts.NpsMgr)
	auth.GET("/nps/server", npsServerHandler.List)
	admin.POST("/nps/server", npsServerHandler.Create)
	admin.PUT("/nps/server/:id", npsServerHandler.Update)
	admin.DELETE("/nps/server/:id", npsServerHandler.Delete)
	admin.POST("/nps/server/:id/start", npsServerHandler.Start)
	admin.POST("/nps/server/:id/stop", npsServerHandler.Stop)

	// NPS 客户端
	npsClientHandler := handlers.NewNpsClientHandler(opts.DB, opts.Log, opts.NpsMgr)
	auth.GET("/nps/client", npsClientHandler.List)
	auth.GET("/nps/client/:id/tunnels", npsClientHandler.ListTunnels)
	admin.POST("/nps/client", npsClientHandler.Create)
	admin.PUT("/nps/client/:id", npsClientHandler.Update)
	admin.DELETE("/nps/client/:id", npsClientHandler.Delete)
	admin.POST("/nps/client/:id/start", npsClientHandler.Start)
	admin.POST("/nps/client/:id/stop", npsClientHandler.Stop)
	admin.POST("/nps/client/:id/tunnels", npsClientHandler.CreateTunnel)
	admin.PUT("/nps/client/:id/tunnels/:tid", npsClientHandler.UpdateTunnel)
	admin.DELETE("/nps/client/:id/tunnels/:tid", npsClientHandler.DeleteTunnel)

	// EasyTier 客户端
	etHandler := handlers.NewEasytierHandler(opts.DB, opts.Log, opts.EasytierMgr)
	auth.GET("/easytier/client", etHandler.List)
	auth.GET("/easytier/client/:id/status", etHandler.GetStatus)
	auth.GET("/easytier/client/:id/logs", etHandler.GetLogs)
	auth.GET("/easytier/client/:id/peers", etHandler.GetPeers)
	admin.POST("/easytier/client", etHandler.Create)
	admin.PUT("/easytier/client/:id", etHandler.Update)
	admin.DELETE("/easytier/client/:id", etHandler.Delete)
	admin.POST("/easytier/client/:id/start", etHandler.Start)
	admin.POST("/easytier/client/:id/stop", etHandler.Stop)

	// EasyTier 服务端
	etsHandler := handlers.NewEasytierServerHandler(opts.DB, opts.Log, opts.EasytierMgr)
	auth.GET("/easytier/server", etsHandler.List)
	auth.GET("/easytier/server/:id/logs", etsHandler.GetLogs)
	auth.GET("/easytier/server/:id/peers", etsHandler.GetPeers)
	admin.POST("/easytier/server", etsHandler.Create)
	admin.PUT("/easytier/server/:id", etsHandler.Update)
	admin.DELETE("/easytier/server/:id", etsHandler.Delete)
	admin.POST("/easytier/server/:id/start", etsHandler.Start)
	admin.POST("/easytier/server/:id/stop", etsHandler.Stop)

	// 穿透服务（用户视角的统一内网穿透管理）
	tsHandler := handlers.NewTunserviceHandler(opts.DB, opts.Log, opts.TunserviceMgr)
	auth.GET("/tunservice", tsHandler.List)
	auth.GET("/tunservice/:id", tsHandler.Get)
	auth.GET("/tunservice/:id/candidates", tsHandler.Candidates)
	auth.GET("/tunservice/:id/history", tsHandler.History)
	admin.POST("/tunservice", tsHandler.Create)
	admin.PUT("/tunservice/:id", tsHandler.Update)
	admin.DELETE("/tunservice/:id", tsHandler.Delete)
	admin.POST("/tunservice/:id/start", tsHandler.Start)
	admin.POST("/tunservice/:id/stop", tsHandler.Stop)
	admin.GET("/tunservice/:id/speedtest", tsHandler.Speedtest)

	// 线路探测策略（参数化配置）
	lineHandler := handlers.NewLineregHandler(opts.DB, opts.Log, opts.LineregMgr)
	auth.GET("/linereg/config", lineHandler.GetConfig)
	auth.GET("/linereg/rebind-pending", lineHandler.PendingRebinds)
	admin.PUT("/linereg/config", lineHandler.UpdateConfig)
	admin.POST("/linereg/rebind-apply", lineHandler.ApplyRebinds)

	// WireGuard
	wgHandler := handlers.NewWireguardHandler(opts.DB, opts.Log, opts.WireguardMgr)
	auth.GET("/wireguard", wgHandler.List)
	auth.GET("/wireguard/:id/status", wgHandler.GetStatus)
	auth.GET("/wireguard/:id/peers", wgHandler.ListPeers)
	auth.GET("/wireguard/:id/peers/:pid/config", wgHandler.GetPeerConfig)
	auth.GET("/wireguard/:id/peers/:pid/qrcode", wgHandler.GetPeerQRCode)
	admin.POST("/wireguard", wgHandler.Create)
	admin.PUT("/wireguard/:id", wgHandler.Update)
	admin.DELETE("/wireguard/:id", wgHandler.Delete)
	admin.POST("/wireguard/:id/start", wgHandler.Start)
	admin.POST("/wireguard/:id/stop", wgHandler.Stop)
	admin.POST("/wireguard/generate-keypair", wgHandler.GenerateKeyPair)
	admin.POST("/wireguard/:id/peers", wgHandler.CreatePeer)
	admin.PUT("/wireguard/:id/peers/:pid", wgHandler.UpdatePeer)
	admin.DELETE("/wireguard/:id/peers/:pid", wgHandler.DeletePeer)

	// DDNS
	ddnsHandler := handlers.NewDDNSHandler(opts.DB, opts.Log, opts.DdnsMgr)
	auth.GET("/ddns", ddnsHandler.List)
	auth.GET("/ddns/:id/history", ddnsHandler.GetHistory)
	admin.POST("/ddns", ddnsHandler.Create)
	admin.PUT("/ddns/:id", ddnsHandler.Update)
	admin.DELETE("/ddns/:id", ddnsHandler.Delete)
	admin.POST("/ddns/:id/start", ddnsHandler.Start)
	admin.POST("/ddns/:id/stop", ddnsHandler.Stop)
	admin.POST("/ddns/:id/run", ddnsHandler.RunNow)

	// Caddy 网站服务
	caddyHandler := handlers.NewCaddyHandler(opts.DB, opts.Log, opts.CaddyMgr)
	// 站点配置可指定本机目录与任意上游地址，属于高危能力，写操作限管理员
	auth.GET("/caddy", caddyHandler.List)
	admin.POST("/caddy", caddyHandler.Create)
	admin.PUT("/caddy/:id", caddyHandler.Update)
	admin.DELETE("/caddy/:id", caddyHandler.Delete)
	admin.POST("/caddy/:id/start", caddyHandler.Start)
	admin.POST("/caddy/:id/stop", caddyHandler.Stop)

	// WOL 网络唤醒（仅发送局域网魔术包，对宿主与公网无实质影响）
	wolHandler := handlers.NewWolHandler(opts.DB, opts.Log)
	auth.GET("/wol", wolHandler.List)
	auth.POST("/wol", wolHandler.Create)
	auth.PUT("/wol/:id", wolHandler.Update)
	auth.DELETE("/wol/:id", wolHandler.Delete)
	auth.POST("/wol/:id/wake", wolHandler.Wake)

	// 域名账号（持有各服务商 API 密钥，读写均限管理员）
	daHandler := handlers.NewDomainAccountHandler(opts.DB, opts.Log)
	admin.GET("/domain/accounts", daHandler.List)
	admin.POST("/domain/accounts", daHandler.Create)
	admin.PUT("/domain/accounts/:id", daHandler.Update)
	admin.DELETE("/domain/accounts/:id", daHandler.Delete)
	admin.POST("/domain/accounts/:id/test", daHandler.Test)

	// 域名管理（域名列表，参考 dnsmgr domain 表）
	diHandler := handlers.NewDomainInfoHandler(opts.DB, opts.Log)
	auth.GET("/domain/domains", diHandler.List)
	admin.GET("/domain/domains/fetch", diHandler.FetchFromProvider)
	admin.POST("/domain/domains", diHandler.Create)
	admin.PUT("/domain/domains/:id", diHandler.Update)
	admin.DELETE("/domain/domains/:id", diHandler.Delete)
	admin.POST("/domain/domains/:id/refresh", diHandler.Refresh)
	admin.PUT("/domain/domains/:id/auto-sync", diHandler.UpdateAutoSync)

	// 证书账号（含 EAB HMAC 密钥，读写均限管理员）
	certAccountHandler := handlers.NewCertAccountHandler(opts.DB, opts.Log, opts.CertMgr)
	admin.GET("/domain/cert-accounts", certAccountHandler.List)
	admin.POST("/domain/cert-accounts", certAccountHandler.Create)
	admin.PUT("/domain/cert-accounts/:id", certAccountHandler.Update)
	admin.DELETE("/domain/cert-accounts/:id", certAccountHandler.Delete)
	admin.POST("/domain/cert-accounts/:id/verify", certAccountHandler.Verify)

	// 域名证书
	certHandler := handlers.NewCertHandler(opts.DB, opts.Log, opts.Config, opts.CertMgr)
	auth.GET("/domain/certs", certHandler.List)
	auth.GET("/domain/certs/:id/status", certHandler.GetStatus)
	admin.POST("/domain/certs", certHandler.Create)
	admin.PUT("/domain/certs/:id", certHandler.Update)
	admin.DELETE("/domain/certs/:id", certHandler.Delete)
	admin.POST("/domain/certs/:id/apply", certHandler.Apply)
	admin.POST("/domain/certs/:id/renew", certHandler.Renew)
	admin.POST("/domain/certs/:id/step/create-order", certHandler.StepCreateOrder)
	admin.POST("/domain/certs/:id/step/set-dns", certHandler.StepSetDNS)
	admin.POST("/domain/certs/:id/step/validate", certHandler.StepValidate)
	admin.POST("/domain/certs/:id/step/obtain", certHandler.StepObtain)
	admin.POST("/domain/certs/:id/confirm-dns", certHandler.ConfirmDNS)

	// 域名解析（子域名解析记录，按域名ID查询）
	drHandler := handlers.NewDomainRecordHandler(opts.DB, opts.Log)
	auth.GET("/domain/records", drHandler.List)
	admin.POST("/domain/records", drHandler.Create)
	admin.PUT("/domain/records/:id", drHandler.Update)
	admin.DELETE("/domain/records/:id", drHandler.Delete)
	admin.POST("/domain/records/sync/:domainInfoId", drHandler.SyncFromProvider)

	// DNSMasq（篡改面板解析即可劫持域名，限管理员）
	dnsmasqHandler := handlers.NewDnsmasqHandler(opts.DB, opts.Log, opts.DnsmasqMgr)
	auth.GET("/dnsmasq/config", dnsmasqHandler.GetConfig)
	auth.GET("/dnsmasq/records", dnsmasqHandler.ListRecords)
	admin.PUT("/dnsmasq/config", dnsmasqHandler.UpdateConfig)
	admin.POST("/dnsmasq/start", dnsmasqHandler.Start)
	admin.POST("/dnsmasq/stop", dnsmasqHandler.Stop)
	admin.POST("/dnsmasq/records", dnsmasqHandler.CreateRecord)
	admin.PUT("/dnsmasq/records/:id", dnsmasqHandler.UpdateRecord)
	admin.DELETE("/dnsmasq/records/:id", dnsmasqHandler.DeleteRecord)

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

	// 网络存储（可暴露宿主任意目录，限管理员）
	storageHandler := handlers.NewStorageHandler(opts.DB, opts.Log, opts.StorageMgr)
	auth.GET("/storage", storageHandler.List)
	admin.POST("/storage", storageHandler.Create)
	admin.PUT("/storage/:id", storageHandler.Update)
	admin.DELETE("/storage/:id", storageHandler.Delete)
	admin.POST("/storage/:id/start", storageHandler.Start)
	admin.POST("/storage/:id/stop", storageHandler.Stop)

	// IP 地址库（订阅会由服务端主动请求外部 URL）
	ipdbHandler := handlers.NewIPDBHandler(opts.DB, opts.Log)
	auth.GET("/ipdb", ipdbHandler.List)
	auth.GET("/ipdb/query", ipdbHandler.Query)
	auth.GET("/ipdb/subscriptions", ipdbHandler.ListSubscriptions)
	admin.POST("/ipdb", ipdbHandler.Create)
	admin.PUT("/ipdb/:id", ipdbHandler.Update)
	admin.DELETE("/ipdb/:id", ipdbHandler.Delete)
	admin.POST("/ipdb/import", ipdbHandler.Import)
	admin.POST("/ipdb/import-url", ipdbHandler.ImportFromURL)
	admin.POST("/ipdb/subscriptions", ipdbHandler.CreateSubscription)
	admin.PUT("/ipdb/subscriptions/:id", ipdbHandler.UpdateSubscription)
	admin.DELETE("/ipdb/subscriptions/:id", ipdbHandler.DeleteSubscription)
	admin.POST("/ipdb/subscriptions/:id/refresh", ipdbHandler.RefreshSubscription)

	// CF 隧道（Cloudflare Tunnel，cloudflared）：可将内网服务暴露到公网
	cftunnelHandler := handlers.NewCfTunnelHandler(opts.DB, opts.Log, opts.CftunnelMgr)
	auth.GET("/cftunnel", cftunnelHandler.List)
	auth.GET("/cftunnel/:id/status", cftunnelHandler.GetStatus)
	auth.GET("/cftunnel/:id/logs", cftunnelHandler.GetLogs)
	auth.GET("/cftunnel/binary", cftunnelHandler.GetBinaryPath)
	auth.GET("/cftunnel/download/info", cftunnelHandler.GetDownloadInfo)
	admin.POST("/cftunnel", cftunnelHandler.Create)
	admin.PUT("/cftunnel/:id", cftunnelHandler.Update)
	admin.DELETE("/cftunnel/:id", cftunnelHandler.Delete)
	admin.POST("/cftunnel/:id/start", cftunnelHandler.Start)
	admin.POST("/cftunnel/:id/stop", cftunnelHandler.Stop)
	admin.POST("/cftunnel/download", cftunnelHandler.DownloadBinary)

	// 访问控制（安全策略本身，写操作限管理员）
	accessHandler := handlers.NewAccessHandler(opts.DB, opts.Log, opts.AccessMgr, opts.CaddyMgr)
	auth.GET("/access", accessHandler.List)
	admin.POST("/access", accessHandler.Create)
	admin.PUT("/access/:id", accessHandler.Update)
	admin.DELETE("/access/:id", accessHandler.Delete)

	// 系统防火墙（iptables/nftables/ufw/firewalld/Windows）
	firewallHandler := handlers.NewFirewallHandler(opts.DB, opts.Log, opts.FirewallMgr)
	auth.GET("/security/firewall", firewallHandler.List)
	auth.GET("/security/firewall/backend", firewallHandler.DetectBackend)
	auth.GET("/security/firewall/sync-status", firewallHandler.GetSyncStatus)
	admin.POST("/security/firewall", firewallHandler.Create)
	admin.PUT("/security/firewall/:id", firewallHandler.Update)
	admin.DELETE("/security/firewall/:id", firewallHandler.Delete)
	admin.POST("/security/firewall/:id/apply", firewallHandler.Apply)
	admin.POST("/security/firewall/:id/remove", firewallHandler.Remove)
	admin.POST("/security/firewall/sync-system", firewallHandler.SyncSystem)

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

	// 回调账号（Config 中含云厂商密钥，读写均限管理员）
	cbAccountHandler := handlers.NewCallbackAccountHandler(opts.DB, opts.Log, opts.CallbackMgr)
	admin.GET("/callback/accounts", cbAccountHandler.List)
	admin.POST("/callback/accounts", cbAccountHandler.Create)
	admin.PUT("/callback/accounts/:id", cbAccountHandler.Update)
	admin.DELETE("/callback/accounts/:id", cbAccountHandler.Delete)
	admin.POST("/callback/accounts/:id/test", cbAccountHandler.Test)

	// 回调任务
	cbTaskHandler := handlers.NewCallbackTaskHandler(opts.DB, opts.Log)
	auth.GET("/callback/tasks", cbTaskHandler.List)
	admin.POST("/callback/tasks", cbTaskHandler.Create)
	admin.PUT("/callback/tasks/:id", cbTaskHandler.Update)
	admin.DELETE("/callback/tasks/:id", cbTaskHandler.Delete)

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
	// 节点可被代理转发任意请求到对端，且 CheckNode/Ping 会主动外联，写操作限管理员。
	meshHandler := handlers.NewMeshNodeHandler(opts.DB, opts.Log, opts.MeshNodeMgr)
	auth.GET("/mesh/nodes", meshHandler.ListNodes)
	auth.GET("/mesh/nodes/:id", meshHandler.GetNode)
	auth.GET("/mesh/topology", meshHandler.GetTopology)
	auth.GET("/mesh/events", meshHandler.ListEvents)
	admin.POST("/mesh/nodes", meshHandler.CreateNode)
	admin.PUT("/mesh/nodes/:id", meshHandler.UpdateNode)
	admin.DELETE("/mesh/nodes/:id", meshHandler.DeleteNode)
	admin.POST("/mesh/nodes/:id/check", meshHandler.CheckNode)
	admin.DELETE("/mesh/events", meshHandler.CleanEvents)
	admin.POST("/mesh/ping", meshHandler.Ping)
	// 代理请求到远程节点。
	// 该接口把任意路径原样转发到对端节点的 /api/v1 之下，本地的 AdminOnly
	// 限制在远端并不生效（例如可代理到对端 /admin/users 创建管理员），
	// 因此必须挂在 admin 组上，避免成为权限旁路。
	admin.Any("/mesh/proxy/:nodeId/*path", meshHandler.ProxyToNode)

	// ── AI 管理 ────────────────────────────────────────────────────────────────
	aiHandler := handlers.NewAiHandler(opts.DB, opts.Log, opts.AiMgr)
	// API 来源（持有第三方 API Key，写操作限管理员）
	auth.GET("/ai/providers", aiHandler.ListProviders)
	admin.POST("/ai/providers", aiHandler.CreateProvider)
	admin.PUT("/ai/providers/:id", aiHandler.UpdateProvider)
	admin.DELETE("/ai/providers/:id", aiHandler.DeleteProvider)
	admin.POST("/ai/providers/:id/fetch-models", aiHandler.FetchModels)
	admin.POST("/ai/providers/:id/test", aiHandler.TestProvider)
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
	// AI 定时任务（可在宿主机触发任务）
	auth.GET("/ai/cron-tasks", aiHandler.ListCronTasks)
	auth.GET("/ai/cron-tasks/:id/logs", aiHandler.ListCronLogs)
	admin.POST("/ai/cron-tasks", aiHandler.CreateCronTask)
	admin.PUT("/ai/cron-tasks/:id", aiHandler.UpdateCronTask)
	admin.DELETE("/ai/cron-tasks/:id", aiHandler.DeleteCronTask)
	admin.POST("/ai/cron-tasks/:id/enable", aiHandler.EnableCronTask)
	admin.POST("/ai/cron-tasks/:id/disable", aiHandler.DisableCronTask)
	admin.POST("/ai/cron-tasks/:id/run", aiHandler.RunCronTask)
	// AI 插件（可被 AI 调用执行动作，写操作限管理员）
	auth.GET("/ai/plugins", aiHandler.ListPlugins)
	admin.POST("/ai/plugins", aiHandler.CreatePlugin)
	admin.PUT("/ai/plugins/:id", aiHandler.UpdatePlugin)
	admin.DELETE("/ai/plugins/:id", aiHandler.DeletePlugin)
	admin.POST("/ai/plugins/:id/toggle", aiHandler.TogglePlugin)

	// ── 服务监控 ────────────────────────────────────────────────────────────────
	// 服务器凭据与探测写操作限管理员（可经 SSH 在受管主机执行命令）
	monitorHandler := handlers.NewMonitorHandler(opts.DB, opts.FrpMgr, opts.NpsMgr, opts.EasytierMgr, opts.CftunnelMgr, opts.WireguardMgr)
	auth.GET("/monitor/servers", monitorHandler.ListServers)
	auth.GET("/monitor/servers/:id", monitorHandler.GetServer)
	admin.POST("/monitor/servers", monitorHandler.CreateServer)
	admin.PUT("/monitor/servers/:id", monitorHandler.UpdateServer)
	admin.DELETE("/monitor/servers/:id", monitorHandler.DeleteServer)
	admin.POST("/monitor/servers/sync/:nodeId", monitorHandler.SyncFromMeshNode)
	// 监控指标
	auth.GET("/monitor/servers/:id/metrics/latest", monitorHandler.GetLatestMetrics)
	auth.GET("/monitor/servers/:id/metrics/history", monitorHandler.GetMetricsHistory)
	// 服务探测
	auth.GET("/monitor/probes", monitorHandler.ListProbes)
	auth.GET("/monitor/probes/:id/results", monitorHandler.GetProbeResults)
	admin.POST("/monitor/probes", monitorHandler.CreateProbe)
	admin.PUT("/monitor/probes/:id", monitorHandler.UpdateProbe)
	admin.DELETE("/monitor/probes/:id", monitorHandler.DeleteProbe)
	// 任务管理（可经 SSH 在受管主机执行任意命令）
	auth.GET("/monitor/tasks", monitorHandler.ListTasks)
	auth.GET("/monitor/tasks/logs", monitorHandler.GetTaskLogs)
	admin.POST("/monitor/tasks", monitorHandler.CreateTask)
	admin.PUT("/monitor/tasks/:id", monitorHandler.UpdateTask)
	admin.DELETE("/monitor/tasks/:id", monitorHandler.DeleteTask)
	admin.POST("/monitor/tasks/:id/execute", monitorHandler.ExecuteTask)
	// 告警规则
	auth.GET("/monitor/alerts", monitorHandler.ListAlerts)
	auth.GET("/monitor/alerts/records", monitorHandler.GetAlertRecords)
	admin.POST("/monitor/alerts", monitorHandler.CreateAlert)
	admin.PUT("/monitor/alerts/:id", monitorHandler.UpdateAlert)
	admin.DELETE("/monitor/alerts/:id", monitorHandler.DeleteAlert)
	// DDNS 绑定
	auth.GET("/monitor/ddns", monitorHandler.GetDDNSBindings)
	admin.POST("/monitor/ddns", monitorHandler.CreateDDNSBinding)
	admin.PUT("/monitor/ddns/:id", monitorHandler.UpdateDDNSBinding)
	admin.DELETE("/monitor/ddns/:id", monitorHandler.DeleteDDNSBinding)
	admin.POST("/monitor/ddns/:id/trigger", monitorHandler.TriggerDDNSUpdate)
	// 通知渠道（含 Webhook/SMTP 凭据）
	admin.GET("/monitor/notifications", monitorHandler.GetNotificationChannels)
	admin.POST("/monitor/notifications", monitorHandler.CreateNotificationChannel)
	admin.PUT("/monitor/notifications/:id", monitorHandler.UpdateNotificationChannel)
	admin.DELETE("/monitor/notifications/:id", monitorHandler.DeleteNotificationChannel)
	admin.POST("/monitor/notifications/test", monitorHandler.SendTestNotification)
	// 隧道绑定
	auth.GET("/monitor/tunnels", monitorHandler.GetTunnelBindings)
	admin.POST("/monitor/tunnels", monitorHandler.CreateTunnelBinding)
	admin.PUT("/monitor/tunnels/:id", monitorHandler.UpdateTunnelBinding)
	admin.DELETE("/monitor/tunnels/:id", monitorHandler.DeleteTunnelBinding)
	admin.POST("/monitor/tunnels/:id/sync", monitorHandler.SyncTunnelStatus)
	// WebSocket 终端：浏览器 WebSocket 无法自定义请求头，token 经 query 传入，
	// 由 HandleTerminal 内部完成鉴权（校验 token + 管理员权限 + Origin）
	r.GET("/ws/terminal", monitorHandler.HandleTerminal)

	return r
}
