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
	VirtualIP       string `gorm:"size:50" json:"virtual_ip"` // 留空自动分配
	// 本地监听端口，支持多个，逗号分隔，格式：tcp:11010,udp:11011 或 12345（基准端口）
	ListenPorts string `gorm:"size:500" json:"listen_ports"`
	// 高级选项
	ExtraArgs string `gorm:"type:text" json:"extra_args"` // 额外命令行参数
	Status    string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError string `gorm:"type:text" json:"last_error"`
	Remark    string `gorm:"size:500" json:"remark"`
}

// ===== EasyTier 服务端 =====

// EasytierServer EasyTier 服务端配置
type EasytierServer struct {
	BaseModel
	Name            string `gorm:"size:100;not null" json:"name"`
	Enable          bool   `gorm:"default:false" json:"enable"`
	ListenAddr      string `gorm:"size:100;default:'0.0.0.0'" json:"listen_addr"`
	// 监听端口，支持多个，逗号分隔，格式：tcp:11010,udp:11011 或 12345（基准端口）
	ListenPorts     string `gorm:"size:500" json:"listen_ports"`
	NetworkName     string `gorm:"size:255" json:"network_name"`
	NetworkPassword string `gorm:"size:255" json:"network_password"`
	ExtraArgs       string `gorm:"type:text" json:"extra_args"`
	Status          string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError       string `gorm:"type:text" json:"last_error"`
	Remark          string `gorm:"size:500" json:"remark"`
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
