package model

import (
	"time"
)

// ===== 基础模型 =====

// BaseModel 公共字段
type BaseModel struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SystemConfig 系统配置表
type SystemConfig struct {
	ID    uint   `gorm:"primarykey" json:"id"`
	Key   string `gorm:"uniqueIndex;size:100" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}

// ===== 端口转发 =====

// PortForwardRule 端口转发规则
type PortForwardRule struct {
	BaseModel
	Name          string `gorm:"size:100;not null" json:"name"`
	Enable        bool   `gorm:"default:false" json:"enable"`
	Protocol      string `gorm:"size:20;default:'tcp'" json:"protocol"` // tcp/udp/tcp+udp
	ListenIP      string `gorm:"size:100;default:'0.0.0.0'" json:"listen_ip"`
	ListenPort    int    `gorm:"not null" json:"listen_port"`
	TargetAddress string `gorm:"size:255;not null" json:"target_address"` // IP或域名
	TargetPort    int    `gorm:"not null" json:"target_port"`
	Remark        string `gorm:"size:500" json:"remark"`
	// 高级选项
	MaxConnections int64  `gorm:"default:256" json:"max_connections"`
	UDPPacketSize  int    `gorm:"default:1500" json:"udp_packet_size"`
	Status         string `gorm:"size:20;default:'stopped'" json:"status"` // running/stopped/error
	LastError      string `gorm:"type:text" json:"last_error"`
}

// ===== STUN 内网穿透 =====

// StunRule STUN 穿透规则
type StunRule struct {
	BaseModel
	Name          string `gorm:"size:100;not null" json:"name"`
	Enable        bool   `gorm:"default:false" json:"enable"`
	TargetAddress string `gorm:"size:255" json:"target_address"` // 转发目标IP/域名
	TargetPort    int    `json:"target_port"`
	UseUPnP       bool   `gorm:"default:false" json:"use_upnp"`
	UseNATMAP     bool   `gorm:"default:false" json:"use_natmap"`
	// STUN 服务器
	StunServer string `gorm:"size:255;default:'stun.l.google.com:19302'" json:"stun_server"`
	// 回调
	CallbackTaskID uint   `json:"callback_task_id"`
	CurrentIP      string `gorm:"size:100" json:"current_ip"`
	CurrentPort    int    `json:"current_port"`
	NATType        string `gorm:"size:50" json:"nat_type"`
	Status         string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError      string `gorm:"type:text" json:"last_error"`
	Remark         string `gorm:"size:500" json:"remark"`
}

// ===== FRP 客户端 =====

// FrpcConfig FRP 客户端配置
type FrpcConfig struct {
	BaseModel
	Name       string `gorm:"size:100;not null" json:"name"`
	Enable     bool   `gorm:"default:false" json:"enable"`
	ServerAddr string `gorm:"size:255;not null" json:"server_addr"`
	ServerPort int    `gorm:"default:7000" json:"server_port"`
	Token      string `gorm:"size:255" json:"token"`
	// 传输协议：tcp/kcp/quic/websocket/wss
	TransportProtocol string `gorm:"size:20;default:'tcp'" json:"transport_protocol"`
	// KCP 连接端口（使用 KCP 协议时指定，0 表示与 ServerPort 相同）
	KCPPort int `gorm:"default:0" json:"kcp_port"`
	// QUIC 连接端口（使用 QUIC 协议时指定，0 表示与 ServerPort 相同）
	QUICPort  int  `gorm:"default:0" json:"quic_port"`
	TLSEnable bool `gorm:"default:false" json:"tls_enable"`
	LogLevel  string      `gorm:"size:20;default:'info'" json:"log_level"`
	Proxies   []FrpcProxy `gorm:"foreignKey:FrpcID" json:"proxies"`
	Status    string      `gorm:"size:20;default:'stopped'" json:"status"`
	LastError string      `gorm:"type:text" json:"last_error"`
	Remark    string      `gorm:"size:500" json:"remark"`
}

// FrpcProxy FRP 代理配置（子表）
type FrpcProxy struct {
	BaseModel
	FrpcID     uint   `gorm:"not null;index" json:"frpc_id"`
	Name       string `gorm:"size:100;not null" json:"name"`
	Type       string `gorm:"size:20;not null" json:"type"` // tcp/udp/http/https/stcp/xtcp/xudp
	LocalIP    string `gorm:"size:100;default:'127.0.0.1'" json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`
	// HTTP/HTTPS 专用
	CustomDomains string `gorm:"size:500" json:"custom_domains"`
	Subdomain     string `gorm:"size:255" json:"subdomain"`
	// STCP/XTCP/XUDP 专用
	SecretKey  string `gorm:"size:255" json:"secret_key"`
	AllowUsers string `gorm:"size:500" json:"allow_users"` // 逗号分隔，允许访问的用户
	// 加密压缩
	UseEncryption  bool `gorm:"default:false" json:"use_encryption"`
	UseCompression bool `gorm:"default:false" json:"use_compression"`
	Enable         bool `gorm:"default:true" json:"enable"`
}

// ===== FRP 服务端 =====

// FrpsConfig FRP 服务端配置
type FrpsConfig struct {
	BaseModel
	Name     string `gorm:"size:100;not null" json:"name"`
	Enable   bool   `gorm:"default:false" json:"enable"`
	BindAddr string `gorm:"size:100;default:'0.0.0.0'" json:"bind_addr"`
	BindPort int    `gorm:"default:7000" json:"bind_port"`
	// KCP 监听端口（UDP），0 表示不启用
	KCPBindPort int `gorm:"default:0" json:"kcp_bind_port"`
	// QUIC 监听端口（UDP），0 表示不启用
	QUICBindPort int `gorm:"default:0" json:"quic_bind_port"`
	// HTTP 虚拟主机端口，0 表示不启用
	VhostHTTPPort int `gorm:"default:0" json:"vhost_http_port"`
	// HTTPS 虚拟主机端口，0 表示不启用
	VhostHTTPSPort int `gorm:"default:0" json:"vhost_https_port"`
	// 子域名根域名，用于 HTTP/HTTPS 代理的子域名功能
	SubDomainHost     string `gorm:"size:255" json:"sub_domain_host"`
	Token             string `gorm:"size:255" json:"token"`
	DashboardAddr     string `gorm:"size:100" json:"dashboard_addr"`
	DashboardPort     int    `json:"dashboard_port"`
	DashboardUser     string `gorm:"size:100" json:"dashboard_user"`
	DashboardPassword string `gorm:"size:255" json:"dashboard_password"`
	MaxPortsPerClient int    `gorm:"default:0" json:"max_ports_per_client"`
	LogLevel          string `gorm:"size:20;default:'info'" json:"log_level"`
	Status            string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError         string `gorm:"type:text" json:"last_error"`
	Remark            string `gorm:"size:500" json:"remark"`
}

// ===== NPS 服务端 =====

// NpsServerConfig NPS 服务端配置
type NpsServerConfig struct {
	BaseModel
	Name              string `gorm:"size:100;not null" json:"name"`
	Enable            bool   `gorm:"default:false" json:"enable"`
	BindAddr          string `gorm:"size:100;default:'0.0.0.0'" json:"bind_addr"`
	BridgePort        int    `gorm:"default:8024" json:"bridge_port"`   // 客户端连接端口
	HTTPPort          int    `gorm:"default:80" json:"http_port"`       // HTTP 代理端口
	HTTPSPort         int    `gorm:"default:443" json:"https_port"`     // HTTPS 代理端口
	WebPort           int    `gorm:"default:8080" json:"web_port"`      // Web 管理端口
	WebUsername       string `gorm:"size:100;default:'admin'" json:"web_username"`
	WebPassword       string `gorm:"size:255;default:'123456'" json:"web_password"`
	AuthKey           string `gorm:"size:255" json:"auth_key"`          // 连接认证密钥
	LogLevel          string `gorm:"size:20;default:'info'" json:"log_level"`
	Status            string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError         string `gorm:"type:text" json:"last_error"`
	Remark            string `gorm:"size:500" json:"remark"`
}

// ===== NPS 客户端 =====

// NpsClientConfig NPS 客户端配置
type NpsClientConfig struct {
	BaseModel
	Name       string `gorm:"size:100;not null" json:"name"`
	Enable     bool   `gorm:"default:false" json:"enable"`
	ServerAddr string `gorm:"size:255;not null" json:"server_addr"` // NPS 服务器地址
	ServerPort int    `gorm:"default:8024" json:"server_port"`      // NPS 服务器桥接端口
	ConnType   string `gorm:"size:20;default:'tcp'" json:"conn_type"` // 连接类型: tcp/tls/kcp/quic/ws/wss
	AuthKey    string `gorm:"size:255" json:"auth_key"`             // 连接认证密钥
	VkeyOrID   string `gorm:"size:255" json:"vkey_or_id"`           // 客户端唯一标识/vkey
	LogLevel   string `gorm:"size:20;default:'info'" json:"log_level"`
	Status     string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError  string `gorm:"type:text" json:"last_error"`
	Remark     string `gorm:"size:500" json:"remark"`
}

// ===== EasyTier 客户端 =====

// EasytierClient EasyTier 客户端配置
type EasytierClient struct {
	BaseModel
	Name            string `gorm:"size:100;not null" json:"name"`
	Enable          bool   `gorm:"default:false" json:"enable"`
	ServerAddr      string `gorm:"size:500" json:"server_addr"` // 支持多个，逗号分隔，格式：tcp://ip:port
	NetworkName     string `gorm:"size:255" json:"network_name"`
	NetworkPassword string `gorm:"size:255" json:"network_password"`
	VirtualIP       string `gorm:"size:50" json:"virtual_ip"`   // 留空自动分配，格式：10.0.0.1/24
	Hostname        string `gorm:"size:255" json:"hostname"`    // --hostname：自定义节点主机名
	// 本地监听端口，支持多个，逗号分隔，格式：tcp:11010,udp:11011 或 12345（基准端口）
	ListenPorts   string `gorm:"size:500" json:"listen_ports"`
	// 映射监听器（用于 NAT 后公告外部地址），逗号分隔，格式：tcp://1.2.3.4:11010
	MappedListeners string `gorm:"size:500" json:"mapped_listeners"`
	// 子网代理（将本机子网共享给虚拟网络），逗号分隔，格式：192.168.1.0/24 或 192.168.1.0/24->10.0.0.0/24
	ProxyCidrs string `gorm:"size:500" json:"proxy_cidrs"`
	// 出口节点（使用其他节点作为出口），逗号分隔，格式：10.0.0.1
	ExitNodes string `gorm:"size:500" json:"exit_nodes"`

	// ===== 网络行为选项 =====
	NoTun               bool `gorm:"default:false" json:"no_tun"`                // --no-tun：不创建 TUN 虚拟网卡（无需 WinPcap/Npcap）
	EnableDhcp          bool `gorm:"default:false" json:"enable_dhcp"`           // --dhcp：DHCP 自动分配虚拟 IP
	DisableP2P          bool `gorm:"default:false" json:"disable_p2p"`           // --disable-p2p：禁用 P2P 直连，强制走中继
	P2POnly             bool `gorm:"default:false" json:"p2p_only"`              // --p2p-only：仅 P2P，禁用中继
	LatencyFirst        bool `gorm:"default:false" json:"latency_first"`         // --latency-first：延迟优先路由
	EnableExitNode      bool `gorm:"default:false" json:"enable_exit_node"`      // --enable-exit-node：允许本节点作为出口节点
	RelayAllPeerRpc     bool `gorm:"default:false" json:"relay_all_peer_rpc"`    // --relay-all-peer-rpc：中继所有对等 RPC

	// ===== 打洞选项 =====
	DisableUdpHolePunching bool `gorm:"default:false" json:"disable_udp_hole_punching"` // --disable-udp-hole-punching
	DisableTcpHolePunching bool `gorm:"default:false" json:"disable_tcp_hole_punching"` // --disable-tcp-hole-punching
	DisableSymHolePunching bool `gorm:"default:false" json:"disable_sym_hole_punching"` // --disable-sym-hole-punching（对称 NAT）

	// ===== 协议加速选项 =====
	EnableKcpProxy  bool `gorm:"default:false" json:"enable_kcp_proxy"`  // --enable-kcp-proxy：KCP 加速代理
	EnableQuicProxy bool `gorm:"default:false" json:"enable_quic_proxy"` // --enable-quic-proxy：QUIC 加速代理

	// ===== TUN/网卡选项 =====
	DevName      string `gorm:"size:100" json:"dev_name"`       // --dev-name：自定义 TUN 设备名
	UseSmoltcp   bool   `gorm:"default:false" json:"use_smoltcp"` // --use-smoltcp：使用 smoltcp 用户态协议栈
	DisableIpv6  bool   `gorm:"default:false" json:"disable_ipv6"` // --disable-ipv6：禁用 IPv6
	Mtu          int    `gorm:"default:0" json:"mtu"`            // --mtu：MTU 大小（0 表示使用默认值）
	EnableMagicDns bool `gorm:"default:false" json:"enable_magic_dns"` // --enable-magic-dns：启用 Magic DNS

	// ===== 安全选项 =====
	DisableEncryption bool `gorm:"default:false" json:"disable_encryption"` // --disable-encryption：禁用加密（不推荐）
	EnablePrivateMode bool `gorm:"default:false" json:"enable_private_mode"` // --enable-private-mode：私有模式（仅允许已知节点）

	// ===== 中继选项 =====
	RelayNetworkWhitelist string `gorm:"size:500" json:"relay_network_whitelist"` // --relay-network-whitelist：允许中继的网络白名单

	// ===== VPN 门户 =====
	EnableVpnPortal          bool   `gorm:"default:false" json:"enable_vpn_portal"`           // 启用 WireGuard VPN 门户
	VpnPortalListenPort      int    `gorm:"default:0" json:"vpn_portal_listen_port"`          // VPN 门户 WireGuard 监听端口
	VpnPortalClientNetwork   string `gorm:"size:100" json:"vpn_portal_client_network"`        // VPN 客户端网段，格式：10.14.14.0/24

	// ===== SOCKS5 代理 =====
	EnableSocks5 bool `gorm:"default:false" json:"enable_socks5"` // 启用 SOCKS5 代理
	Socks5Port   int  `gorm:"default:0" json:"socks5_port"`       // SOCKS5 监听端口

	// ===== 手动路由 =====
	EnableManualRoutes bool   `gorm:"default:false" json:"enable_manual_routes"` // --manual-routes：启用手动路由
	ManualRoutes       string `gorm:"type:text" json:"manual_routes"`            // 手动路由列表，逗号分隔，格式：10.0.0.0/24

	// ===== 端口转发 =====
	// 格式：proto:bind_ip:bind_port:dst_ip:dst_port，多条用换行分隔，如 tcp:0.0.0.0:8080:192.168.1.1:80
	PortForwards string `gorm:"type:text" json:"port_forwards"`

	// ===== 运行时选项 =====
	MultiThread bool `gorm:"default:false" json:"multi_thread"` // --multi-thread：启用多线程运行时

	ExtraArgs string `gorm:"type:text" json:"extra_args"` // 额外命令行参数（兜底）
	Status    string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError string `gorm:"type:text" json:"last_error"`
	Remark    string `gorm:"size:500" json:"remark"`
}

// ===== EasyTier 服务端 =====

// EasytierServer EasyTier 服务端配置
type EasytierServer struct {
	BaseModel
	Name   string `gorm:"size:100;not null" json:"name"`
	Enable bool   `gorm:"default:false" json:"enable"`

	// ServerMode 运行模式：standalone（独立部署，默认）或 config-server（节点模式，连接到 config-server）
	// standalone 模式下可配置所有参数；config-server 模式下只需填写 ConfigServerAddr
	ServerMode string `gorm:"size:20;default:'standalone'" json:"server_mode"`
	// ConfigServerAddr config-server 地址，仅 config-server 模式下使用，格式：tcp://host:port
	ConfigServerAddr string `gorm:"size:500" json:"config_server_addr"`

	ListenAddr string `gorm:"size:100;default:'0.0.0.0'" json:"listen_addr"`
	// 监听端口，支持多个，逗号分隔，格式：tcp:11010,udp:11011 或 12345（基准端口）
	ListenPorts     string `gorm:"size:500" json:"listen_ports"`
	NetworkName     string `gorm:"size:255" json:"network_name"`
	NetworkPassword string `gorm:"size:255" json:"network_password"`
	Hostname        string `gorm:"size:255" json:"hostname"` // --hostname：自定义节点主机名

	// ===== 网络行为选项 =====
	NoTun           bool `gorm:"default:false" json:"no_tun"`            // --no-tun：不创建 TUN 虚拟网卡
	DisableP2P      bool `gorm:"default:false" json:"disable_p2p"`       // --disable-p2p：禁用 P2P 直连
	RelayAllPeerRpc bool `gorm:"default:false" json:"relay_all_peer_rpc"` // --relay-all-peer-rpc：中继所有对等 RPC
	EnableExitNode  bool `gorm:"default:false" json:"enable_exit_node"`  // --enable-exit-node：允许作为出口节点

	// ===== 协议加速选项 =====
	EnableKcpProxy  bool `gorm:"default:false" json:"enable_kcp_proxy"`  // --enable-kcp-proxy
	EnableQuicProxy bool `gorm:"default:false" json:"enable_quic_proxy"` // --enable-quic-proxy

	// ===== 安全选项 =====
	DisableEncryption bool `gorm:"default:false" json:"disable_encryption"` // --disable-encryption
	EnablePrivateMode bool `gorm:"default:false" json:"enable_private_mode"` // --enable-private-mode

	// ===== 中继选项 =====
	RelayNetworkWhitelist string `gorm:"size:500" json:"relay_network_whitelist"` // --relay-network-whitelist

	// ===== 手动路由 =====
	EnableManualRoutes bool   `gorm:"default:false" json:"enable_manual_routes"` // --manual-routes：启用手动路由
	ManualRoutes       string `gorm:"type:text" json:"manual_routes"`            // 手动路由列表，逗号分隔，格式：10.0.0.0/24

	// ===== 端口转发 =====
	// 格式：proto:bind_ip:bind_port:dst_ip:dst_port，多条用换行分隔，如 tcp:0.0.0.0:8080:192.168.1.1:80
	PortForwards string `gorm:"type:text" json:"port_forwards"`

	// ===== 运行时选项 =====
	MultiThread bool `gorm:"default:true" json:"multi_thread"` // --multi-thread：启用多线程运行时

	ExtraArgs string `gorm:"type:text" json:"extra_args"`
	Status    string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError string `gorm:"type:text" json:"last_error"`
	Remark    string `gorm:"size:500" json:"remark"`
}

// ===== DDNS =====

// DDNSTask DDNS 任务
type DDNSTask struct {
	BaseModel
	Name             string `gorm:"size:100;not null" json:"name"`
	Enable           bool   `gorm:"default:false" json:"enable"`
	TaskType         string `gorm:"size:10;default:'IPv4'" json:"task_type"` // IPv4/IPv6
	Provider         string `gorm:"size:50;not null" json:"provider"`        // alidns/cloudflare/dnspod/...
	DomainAccountID  uint   `json:"domain_account_id"`                       // 关联域名账号（可选）
	AccessID         string `gorm:"size:255" json:"access_id"`
	AccessSecret     string `gorm:"size:500" json:"access_secret"`
	Domains          string `gorm:"type:text" json:"domains"`   // JSON 数组
	IPGetType        string `gorm:"size:20;default:'url'" json:"ip_get_type"` // url/interface/custom
	IPGetURLs        string `gorm:"type:text" json:"ip_get_urls"` // JSON 数组
	NetInterface     string `gorm:"size:100" json:"net_interface"`
	IPRegex          string `gorm:"size:255" json:"ip_regex"`
	TTL              string `gorm:"size:20;default:'600'" json:"ttl"`
	Interval         int    `gorm:"default:300" json:"interval"` // 检查间隔（秒）
	CurrentIP        string `gorm:"size:100" json:"current_ip"`
	LastUpdateTime   *time.Time `json:"last_update_time"`
	Status           string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError        string `gorm:"type:text" json:"last_error"`
	Remark           string `gorm:"size:500" json:"remark"`
}

// ===== Caddy 网站服务 =====

// CaddySite Caddy 站点配置
type CaddySite struct {
	BaseModel
	Name       string `gorm:"size:100;not null" json:"name"`
	Enable     bool   `gorm:"default:false" json:"enable"`
	Domain     string `gorm:"size:255" json:"domain"`
	Port       int    `gorm:"default:80" json:"port"`
	SiteType   string `gorm:"size:30;default:'reverse_proxy'" json:"site_type"` // reverse_proxy/static/redirect/rewrite
	// 反向代理
	UpstreamAddr string `gorm:"size:500" json:"upstream_addr"`
	// 静态文件
	RootPath  string `gorm:"size:500" json:"root_path"`
	FileList  bool   `gorm:"default:false" json:"file_list"`
	// 重定向
	RedirectTo   string `gorm:"size:500" json:"redirect_to"`
	RedirectCode int    `gorm:"default:301" json:"redirect_code"`
	// SSL
	TLSEnable      bool   `gorm:"default:false" json:"tls_enable"`
	TLSMode        string `gorm:"size:20;default:'auto'" json:"tls_mode"` // auto/manual/acme
	TLSCertFile    string `gorm:"size:500" json:"tls_cert_file"`
	TLSKeyFile     string `gorm:"size:500" json:"tls_key_file"`
	DomainCertID   uint   `json:"domain_cert_id"`
	// 高级
	ExtraConfig string `gorm:"type:text" json:"extra_config"` // 额外 Caddyfile 片段
	Status      string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError   string `gorm:"type:text" json:"last_error"`
	Remark      string `gorm:"size:500" json:"remark"`
}

// ===== WOL 网络唤醒 =====

// WolDevice WOL 设备
type WolDevice struct {
	BaseModel
	Name           string `gorm:"size:100;not null" json:"name"`
	MACAddress     string `gorm:"size:20;not null" json:"mac_address"`
	BroadcastIP    string `gorm:"size:100;default:'255.255.255.255'" json:"broadcast_ip"`
	Port           int    `gorm:"default:9" json:"port"`
	NetInterface   string `gorm:"size:100" json:"net_interface"`
	Remark         string `gorm:"size:500" json:"remark"`
}

// ===== 域名账号 =====

// DomainAccount 域名服务商账号
type DomainAccount struct {
	BaseModel
	Name         string `gorm:"size:100;not null" json:"name"`
	Provider     string `gorm:"size:50;not null" json:"provider"` // alidns/cloudflare/dnspod/...
	AccessID     string `gorm:"size:255" json:"access_id"`
	AccessSecret string `gorm:"size:500" json:"access_secret"`
	Remark       string `gorm:"size:500" json:"remark"`
}

// ===== 域名证书 =====

// DomainCert ACME 域名证书
type DomainCert struct {
	BaseModel
	Name            string     `gorm:"size:100;not null" json:"name"`
	Domains         string     `gorm:"type:text;not null" json:"domains"` // JSON 数组
	Email           string     `gorm:"size:255" json:"email"`
	CA              string     `gorm:"size:50;default:'letsencrypt'" json:"ca"` // letsencrypt/zerossl
	ChallengeType   string     `gorm:"size:20;default:'dns'" json:"challenge_type"` // dns/http
	DomainAccountID uint       `json:"domain_account_id"`
	CertFile        string     `gorm:"size:500" json:"cert_file"`
	KeyFile         string     `gorm:"size:500" json:"key_file"`
	ExpireAt        *time.Time `json:"expire_at"`
	AutoRenew       bool       `gorm:"default:true" json:"auto_renew"`
	Status          string     `gorm:"size:20;default:'pending'" json:"status"` // pending/valid/expired/error
	LastError       string     `gorm:"type:text" json:"last_error"`
	Remark          string     `gorm:"size:500" json:"remark"`
}

// ===== 域名解析 =====

// DomainRecord 域名解析记录
type DomainRecord struct {
	BaseModel
	DomainAccountID uint   `gorm:"not null;index" json:"domain_account_id"`
	Domain          string `gorm:"size:255;not null" json:"domain"`
	RecordType      string `gorm:"size:20;not null" json:"record_type"` // A/AAAA/CNAME/MX/TXT/...
	Host            string `gorm:"size:255;not null" json:"host"`       // 主机记录
	Value           string `gorm:"size:500;not null" json:"value"`      // 记录值
	TTL             int    `gorm:"default:600" json:"ttl"`
	RemoteID        string `gorm:"size:255" json:"remote_id"` // 服务商记录ID
	Remark          string `gorm:"size:500" json:"remark"`
}

// ===== DNSMasq =====

// DnsmasqConfig DNSMasq 全局配置
type DnsmasqConfig struct {
	BaseModel
	Enable      bool   `gorm:"default:false" json:"enable"`
	ListenAddr  string `gorm:"size:100;default:'0.0.0.0'" json:"listen_addr"`
	ListenPort  int    `gorm:"default:53" json:"listen_port"`
	UpstreamDNS string `gorm:"type:text" json:"upstream_dns"` // JSON 数组
	Status      string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError   string `gorm:"type:text" json:"last_error"`
}

// DnsmasqRecord DNSMasq 自定义解析记录
type DnsmasqRecord struct {
	BaseModel
	Domain  string `gorm:"size:255;not null" json:"domain"`
	IP      string `gorm:"size:100;not null" json:"ip"`
	Enable  bool   `gorm:"default:true" json:"enable"`
	Remark  string `gorm:"size:500" json:"remark"`
}

// ===== 计划任务 =====

// CronTask 计划任务
type CronTask struct {
	BaseModel
	Name         string     `gorm:"size:100;not null" json:"name"`
	Enable       bool       `gorm:"default:false" json:"enable"`
	CronExpr     string     `gorm:"size:100;not null" json:"cron_expr"`
	TaskType     string     `gorm:"size:20;default:'shell'" json:"task_type"` // shell/http/internal
	Command      string     `gorm:"type:text" json:"command"`
	HTTPURL      string     `gorm:"size:500" json:"http_url"`
	HTTPMethod   string     `gorm:"size:10;default:'GET'" json:"http_method"`
	HTTPBody     string     `gorm:"type:text" json:"http_body"`
	LastRunTime  *time.Time `json:"last_run_time"`
	LastRunResult string    `gorm:"type:text" json:"last_run_result"`
	Status       string     `gorm:"size:20;default:'stopped'" json:"status"`
	Remark       string     `gorm:"size:500" json:"remark"`
}

// ===== 网络存储 =====

// StorageConfig 网络存储配置
type StorageConfig struct {
	BaseModel
	Name       string `gorm:"size:100;not null" json:"name"`
	Enable     bool   `gorm:"default:false" json:"enable"`
	Protocol   string `gorm:"size:20;not null" json:"protocol"` // webdav/sftp/smb
	ListenAddr string `gorm:"size:100;default:'0.0.0.0'" json:"listen_addr"`
	ListenPort int    `json:"listen_port"`
	RootPath   string `gorm:"size:500;not null" json:"root_path"`
	Username   string `gorm:"size:100" json:"username"`
	Password   string `gorm:"size:255" json:"password"`
	ReadOnly   bool   `gorm:"default:false" json:"read_only"`
	Status     string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError  string `gorm:"type:text" json:"last_error"`
	Remark     string `gorm:"size:500" json:"remark"`
}

// ===== IP 地址库 =====

// IPDBEntry IP 地址库条目
type IPDBEntry struct {
	BaseModel
	CIDR     string `gorm:"size:50;not null;uniqueIndex" json:"cidr"`
	Location string `gorm:"size:255" json:"location"`
	Tags     string `gorm:"size:500" json:"tags"` // 逗号分隔
	Remark   string `gorm:"size:500" json:"remark"`
}

// ===== 访问控制 =====

// AccessRule 访问控制规则
type AccessRule struct {
	BaseModel
	Name    string `gorm:"size:100;not null" json:"name"`
	Enable  bool   `gorm:"default:false" json:"enable"`
	Mode    string `gorm:"size:20;default:'blacklist'" json:"mode"` // blacklist/whitelist
	IPList  string `gorm:"type:text" json:"ip_list"`                // JSON 数组，支持 CIDR
	Remark  string `gorm:"size:500" json:"remark"`
}

// ===== 回调账号 =====

// CallbackAccount 回调账号
type CallbackAccount struct {
	BaseModel
	Name     string `gorm:"size:100;not null" json:"name"`
	Type     string `gorm:"size:30;not null" json:"type"` // cf_origin/ali_esa/tencent_eo/webhook
	Config   string `gorm:"type:text" json:"config"`      // JSON 配置
	Remark   string `gorm:"size:500" json:"remark"`
}

// ===== 回调任务 =====

// CallbackTask 回调任务
type CallbackTask struct {
	BaseModel
	Name              string `gorm:"size:100;not null" json:"name"`
	Enable            bool   `gorm:"default:false" json:"enable"`
	AccountType       string `gorm:"size:20;default:'callback'" json:"account_type"` // callback/domain
	AccountID         uint   `json:"account_id"`
	TriggerType       string `gorm:"size:30;default:'stun'" json:"trigger_type"` // stun/frp/easytier
	TriggerSourceID   uint   `json:"trigger_source_id"`
	ActionConfig      string `gorm:"type:text" json:"action_config"` // JSON 配置
	LastTriggerTime   *time.Time `json:"last_trigger_time"`
	LastTriggerResult string `gorm:"type:text" json:"last_trigger_result"`
	Remark            string `gorm:"size:500" json:"remark"`
}
