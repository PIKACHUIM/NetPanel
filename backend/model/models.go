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
	ListenIP       string `gorm:"size:100;default:'0.0.0.0'" json:"listen_ip"`
	ListenPort     int    `gorm:"not null" json:"listen_port"`
	ListenPortType string `gorm:"size:20;default:'tcp'" json:"listen_port_type"` // tcp/udp/http/https/socks/websocket
	TargetAddress  string `gorm:"size:255;not null" json:"target_address"`       // IP或域名（单目标，兼容旧版）
	TargetPort     int    `gorm:"not null" json:"target_port"`
	TargetPortType string `gorm:"size:20;default:'tcp'" json:"target_port_type"` // tcp/udp/http/https/socks/websocket
	// 多目标地址（负载均衡），JSON数组，格式：["ip1:port1","ip2:port2"]
	// 若设置此字段则忽略 TargetAddress/TargetPort（参考 lucky PortForwardsRule 多目标）
	TargetAddresses string `gorm:"type:text" json:"target_addresses"`
	Remark          string `gorm:"size:500" json:"remark"`
	// 高级选项（参考 lucky RelayRuleOptions）
	MaxConnections int64  `gorm:"default:256" json:"max_connections"`
	UDPPacketSize  int    `gorm:"default:1500" json:"udp_packet_size"`
	// HTTPS 监听时关联的域名证书 ID（对应 DomainCert.ID），0 表示不使用
	DomainCertID   uint   `gorm:"default:0" json:"domain_cert_id"`
	Status         string `gorm:"size:20;default:'stopped'" json:"status"` // running/stopped/error
	LastError      string `gorm:"type:text" json:"last_error"`
}

// ===== STUN 内网穿透 =====

// StunRule STUN 穿透规则
type StunRule struct {
	BaseModel
	Name   string `gorm:"size:100;not null" json:"name"`
	Enable bool   `gorm:"default:false" json:"enable"`

	// ===== 转发模式 =====
	// proxy: 本机代理（本地监听端口 → STUN 穿透 → 转发到目标，不强制 UPnP/NATMAP）
	// direct: 直接转发（UPnP/NATMAP 直接映射到目标，强制要求 UPnP 或 NATMAP）
	ForwardMode string `gorm:"size:20;default:'proxy'" json:"forward_mode"`

	// ===== 本机代理模式：本地监听端口 =====
	// 仅 forward_mode=proxy 时有效，STUN 穿透此端口后将流量转发到 target_address:target_port
	ListenPort int `gorm:"default:0" json:"listen_port"`

	// ===== 转发目标 =====
	TargetAddress  string `gorm:"size:255" json:"target_address"`          // 转发目标 IP/域名
	TargetPort     int    `json:"target_port"`                             // 转发目标端口
	TargetProtocol string `gorm:"size:10;default:'tcp'" json:"target_protocol"` // tcp/udp

	// ===== NAT 穿透辅助 =====
	UseUPnP   bool `gorm:"default:false" json:"use_upnp"`
	UseNATMAP bool `gorm:"default:false" json:"use_natmap"`

	// UPnP 配置（参考 lucky UPnP 选项）
	UpnpServerIP   string `gorm:"size:100" json:"upnp_server_ip"`   // UPnP 服务器 IP，留空自动发现
	UpnpExternalIP string `gorm:"size:100" json:"upnp_external_ip"` // 指定外部 IP（可选）

	// NATMAP 配置（参考 lucky NATMAP 选项）
	NatmapServerAddr string `gorm:"size:255" json:"natmap_server_addr"` // NATMAP 服务器地址，如 stun.miwifi.com:3478
	NatmapKeepAlive  int    `gorm:"default:30" json:"natmap_keepalive"` // 保活间隔（秒）

	// ===== STUN 服务器 =====
	StunServer string `gorm:"size:255;default:'stun.l.google.com:19302'" json:"stun_server"`

	// ===== 高级选项 =====
	// DisableValidation: 禁用有效性检测，勾选后不检测 NAT 类型，直接使用 STUN 返回的地址
	DisableValidation bool `gorm:"default:false" json:"disable_validation"`

	// ===== 回调 =====
	CallbackTaskID uint `json:"callback_task_id"`

	// ===== 运行时状态 =====
	CurrentIP   string `gorm:"size:100" json:"current_ip"`
	CurrentPort int    `json:"current_port"`
	NATType     string `gorm:"size:50" json:"nat_type"`
	// StunStatus: STUN 穿透状态，running 时细化为 penetrating/timeout/failed
	// penetrating: 穿透中（已获取到有效 IP/端口）
	// timeout: 检测超时
	// failed: 检测失败
	StunStatus string `gorm:"size:20" json:"stun_status"`
	Status     string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError  string `gorm:"type:text" json:"last_error"`
	Remark     string `gorm:"size:500" json:"remark"`
}

// ===== FRP 客户端 =====

// FrpcConfig FRP 客户端配置
type FrpcConfig struct {
	BaseModel
	Name   string `gorm:"size:100;not null" json:"name"`
	Enable bool   `gorm:"default:false" json:"enable"`
	// 用户名，设置后代理名称变为 {user}.{proxyName}
	User       string `gorm:"size:100" json:"user"`
	ServerAddr string `gorm:"size:255;not null" json:"server_addr"`
	ServerPort int    `gorm:"default:7000" json:"server_port"`
	// 认证方式：token/oidc，默认 token
	AuthMethod string `gorm:"size:20;default:'token'" json:"auth_method"`
	Token      string `gorm:"size:255" json:"token"`
	// 传输协议：tcp/kcp/quic/websocket/wss
	TransportProtocol string `gorm:"size:20;default:'tcp'" json:"transport_protocol"`
	// KCP 连接端口（使用 KCP 协议时指定，0 表示与 ServerPort 相同）
	KCPPort int `gorm:"default:0" json:"kcp_port"`
	// QUIC 连接端口（使用 QUIC 协议时指定，0 表示与 ServerPort 相同）
	QUICPort  int  `gorm:"default:0" json:"quic_port"`
	TLSEnable bool `gorm:"default:true" json:"tls_enable"`
	// 连接池大小
	PoolCount int `gorm:"default:5" json:"pool_count"`
	// TCP 多路复用，默认启用
	TCPMux bool `gorm:"default:true" json:"tcp_mux"`
	// tcp_mux 心跳检查间隔（秒）
	TCPMuxKeepaliveInterval int `gorm:"default:0" json:"tcp_mux_keepalive_interval"`
	// 连接服务端超时（秒），默认 10
	DialServerTimeout int `gorm:"default:10" json:"dial_server_timeout"`
	// 底层 TCP keepalive 间隔（秒），0 表示不设置
	DialServerKeepalive int `gorm:"default:0" json:"dial_server_keepalive"`
	// 心跳包发送间隔（秒），默认 30，-1 表示禁用
	HeartbeatInterval int `gorm:"default:30" json:"heartbeat_interval"`
	// 心跳超时（秒），默认 90
	HeartbeatTimeout int `gorm:"default:90" json:"heartbeat_timeout"`
	// 连接服务端时绑定的本地 IP
	ConnectServerLocalIP string `gorm:"size:100" json:"connect_server_local_ip"`
	// 连接服务端使用的代理地址，格式：{protocol}://user:passwd@host:port
	ProxyURL string `gorm:"size:500" json:"proxy_url"`
	// xtcp 打洞所需的 STUN 服务器地址
	NatHoleStunServer string `gorm:"size:255" json:"nat_hole_stun_server"`
	// 自定义 DNS 服务器地址
	DNSServer string `gorm:"size:255" json:"dns_server"`
	// 第一次登录失败后是否退出，默认 true
	LoginFailExit bool `gorm:"default:true" json:"login_fail_exit"`
	// UDP 最大包长度（字节），默认 1500
	UDPPacketSize int `gorm:"default:1500" json:"udp_packet_size"`
	// Web 管理端口
	WebServerPort     int    `gorm:"default:0" json:"web_server_port"`
	WebServerUser     string `gorm:"size:100" json:"web_server_user"`
	WebServerPassword string `gorm:"size:255" json:"web_server_password"`
	LogLevel          string      `gorm:"size:20;default:'info'" json:"log_level"`
	Proxies           []FrpcProxy `gorm:"foreignKey:FrpcID" json:"proxies"`
	Status            string      `gorm:"size:20;default:'stopped'" json:"status"`
	LastError         string      `gorm:"type:text" json:"last_error"`
	Remark            string      `gorm:"size:500" json:"remark"`
}

// FrpcProxy FRP 代理配置（子表）
type FrpcProxy struct {
	BaseModel
	FrpcID uint   `gorm:"not null;index" json:"frpc_id"`
	Name   string `gorm:"size:100;not null" json:"name"`
	// 代理类型：tcp/udp/http/https/stcp/sudp/xtcp/tcpmux
	Type      string `gorm:"size:20;not null;default:'tcp'" json:"type"`
	LocalIP   string `gorm:"size:100;default:'127.0.0.1'" json:"local_ip"`
	LocalPort int    `json:"local_port"`
	// TCP/UDP 远程端口
	RemotePort int `json:"remote_port"`
	// HTTP/HTTPS/TCPMUX 专用：域名
	CustomDomains string `gorm:"size:500" json:"custom_domains"`
	Subdomain     string `gorm:"size:255" json:"subdomain"`
	// HTTP/HTTPS/TCPMUX 专用：路由
	Locations         string `gorm:"size:500" json:"locations"`          // 路径匹配，逗号分隔，如 /,/api
	HostHeaderRewrite string `gorm:"size:255" json:"host_header_rewrite"` // Host 头重写
	RequestHeaders    string `gorm:"type:text" json:"request_headers"`   // 自定义请求头，key=value 换行分隔
	// HTTP/HTTPS/TCPMUX 专用：Basic Auth
	HTTPUser     string `gorm:"size:100" json:"http_user"`
	HTTPPassword string `gorm:"size:255" json:"http_password"`
	// TCPMUX 专用
	Multiplexer string `gorm:"size:50;default:'httpconnect'" json:"multiplexer"` // 目前仅支持 httpconnect
	// STCP/SUDP/XTCP 专用：私密访问
	SecretKey  string `gorm:"size:255" json:"secret_key"`
	AllowUsers string `gorm:"size:500" json:"allow_users"` // 逗号分隔，允许访问的用户，* 表示所有
	// 加密压缩
	UseEncryption  bool `gorm:"default:false" json:"use_encryption"`
	UseCompression bool `gorm:"default:false" json:"use_compression"`
	Enable         bool `gorm:"default:true" json:"enable"`
	// 带宽限制
	BandwidthLimit     string `gorm:"size:50" json:"bandwidth_limit"`                       // 如 "1MB"，空表示不限制
	BandwidthLimitMode string `gorm:"size:20;default:'client'" json:"bandwidth_limit_mode"` // client/server
	// 健康检查
	HealthCheckType      string `gorm:"size:20" json:"health_check_type"`       // tcp/http，空表示不启用
	HealthCheckPath      string `gorm:"size:500" json:"health_check_path"`      // HTTP 健康检查路径，如 /health
	HealthCheckTimeoutS  int    `gorm:"default:3" json:"health_check_timeout_s"` // 超时（秒）
	HealthCheckIntervalS int    `gorm:"default:10" json:"health_check_interval_s"` // 检查间隔（秒）
	HealthCheckMaxFailed int    `gorm:"default:3" json:"health_check_max_failed"`  // 最大失败次数
	// 负载均衡
	LoadBalancerGroup    string `gorm:"size:100" json:"load_balancer_group"`     // 负载均衡组名，空表示不参与
	LoadBalancerGroupKey string `gorm:"size:255" json:"load_balancer_group_key"` // 组密钥
	// 插件（参考 frp plugin）
	PluginType   string `gorm:"size:50" json:"plugin_type"`     // http_proxy/socks5/static_file/unix_domain_socket
	PluginConfig string `gorm:"type:text" json:"plugin_config"` // JSON 配置
	Remark       string `gorm:"size:500" json:"remark"`
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
	// 代理监听地址，默认同 BindAddr
	ProxyBindAddr string `gorm:"size:100" json:"proxy_bind_addr"`
	// HTTP 虚拟主机端口，0 表示不启用
	VhostHTTPPort int `gorm:"default:0" json:"vhost_http_port"`
	// HTTP 虚拟主机 ResponseHeader 超时（秒），默认 60
	VhostHTTPTimeout int `gorm:"default:60" json:"vhost_http_timeout"`
	// HTTPS 虚拟主机端口，0 表示不启用
	VhostHTTPSPort int `gorm:"default:0" json:"vhost_https_port"`
	// tcpmux httpconnect 代理监听端口，0 表示不启用
	TcpmuxHTTPConnectPort int  `gorm:"default:0" json:"tcpmux_http_connect_port"`
	// tcpmux 是否透传 CONNECT 请求
	TcpmuxPassthrough bool `gorm:"default:false" json:"tcpmux_passthrough"`
	// 子域名根域名，用于 HTTP/HTTPS 代理的子域名功能
	SubDomainHost string `gorm:"size:255" json:"sub_domain_host"`
	// 自定义 404 错误页面地址
	Custom404Page string `gorm:"size:500" json:"custom_404_page"`
	// 认证 Token
	Token string `gorm:"size:255" json:"token"`
	// Dashboard（WebServer）配置
	DashboardAddr     string `gorm:"size:100" json:"dashboard_addr"`
	DashboardPort     int    `json:"dashboard_port"`
	DashboardUser     string `gorm:"size:100" json:"dashboard_user"`
	DashboardPassword string `gorm:"size:255" json:"dashboard_password"`
	// 是否启用 Prometheus 监控（需同时启用 Dashboard）
	EnablePrometheus bool `gorm:"default:false" json:"enable_prometheus"`
	// 限制
	MaxPortsPerClient int `gorm:"default:0" json:"max_ports_per_client"`
	// 用户建立连接后等待客户端响应的超时时间（秒），默认 10
	UserConnTimeout int `gorm:"default:10" json:"user_conn_timeout"`
	// UDP 包最大长度（字节），默认 1500，需与客户端一致
	UDPPacketSize int `gorm:"default:1500" json:"udp_packet_size"`
	// 打洞策略数据保留时间（小时），默认 168（7天）
	NatholeAnalysisDataReserveHours int `gorm:"default:168" json:"nathole_analysis_data_reserve_hours"`
	// 服务端返回详细错误信息给客户端，默认 true
	DetailedErrorsToClient bool `gorm:"default:true" json:"detailed_errors_to_client"`
	// 日志配置
	LogLevel   string `gorm:"size:20;default:'info'" json:"log_level"`
	LogFile    string `gorm:"size:500" json:"log_file"`    // 留空输出到控制台
	LogMaxDays int    `gorm:"default:3" json:"log_max_days"` // 日志保留天数
	// 传输层配置
	TransportMaxPoolCount      int  `gorm:"default:5" json:"transport_max_pool_count"`       // 最大连接池数量
	TransportHeartbeatTimeout  int  `gorm:"default:90" json:"transport_heartbeat_timeout"`   // 心跳超时（秒）
	TransportTCPMuxKeepalive   int  `gorm:"default:0" json:"transport_tcp_mux_keepalive"`    // TCP mux 心跳间隔（秒），0 使用默认
	TransportTCPKeepalive      int  `gorm:"default:0" json:"transport_tcp_keepalive"`        // TCP keepalive 间隔（秒），负数禁用
	TransportTLSForce          bool `gorm:"default:false" json:"transport_tls_force"`        // 仅接受 TLS 连接
	TransportTLSCertFile       string `gorm:"size:500" json:"transport_tls_cert_file"`       // TLS 证书文件
	TransportTLSKeyFile        string `gorm:"size:500" json:"transport_tls_key_file"`        // TLS 私钥文件
	TransportTLSTrustedCAFile  string `gorm:"size:500" json:"transport_tls_trusted_ca_file"` // 受信任 CA 文件（双向 TLS）
	// SSH 隧道网关配置
	SSHTunnelGatewayBindPort        int    `gorm:"default:0" json:"ssh_tunnel_gateway_bind_port"`          // SSH 服务器监听端口，0 表示不启用
	SSHTunnelGatewayPrivateKeyFile  string `gorm:"size:500" json:"ssh_tunnel_gateway_private_key_file"`    // SSH 私钥文件，留空自动生成
	SSHTunnelGatewayAutoGenKeyPath  string `gorm:"size:500" json:"ssh_tunnel_gateway_auto_gen_key_path"`   // 自动生成私钥路径，默认 ./.autogen_ssh_key
	SSHTunnelGatewayAuthorizedKeys  string `gorm:"size:500" json:"ssh_tunnel_gateway_authorized_keys_file"` // 授权公钥文件，留空不鉴权
	Status    string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError string `gorm:"type:text" json:"last_error"`
	Remark    string `gorm:"size:500" json:"remark"`
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

// NpsTunnel NPS 隧道配置（子表，参考 nps 隧道类型）
type NpsTunnel struct {
	BaseModel
	NpsClientID  uint   `gorm:"not null;index" json:"nps_client_id"`
	Name         string `gorm:"size:100;not null" json:"name"`
	Type         string `gorm:"size:20;not null" json:"type"` // tcp/udp/http/https/socks5/p2p
	LocalIP      string `gorm:"size:100;default:'127.0.0.1'" json:"local_ip"`
	LocalPort    int    `json:"local_port"`
	RemotePort   int    `json:"remote_port"`
	// HTTP/HTTPS 专用
	HostHeader string `gorm:"size:255" json:"host_header"`
	// 认证
	Password string `gorm:"size:255" json:"password"`
	Enable   bool   `gorm:"default:true" json:"enable"`
	Remark   string `gorm:"size:500" json:"remark"`
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
	IPv6            string `gorm:"size:100" json:"ipv6"`        // --ipv6：IPv6 地址，可与 IPv4 同时使用
	Hostname        string `gorm:"size:255" json:"hostname"`    // --hostname：自定义节点主机名
	InstanceName    string `gorm:"size:255" json:"instance_name"` // --instance-name：实例名称，同机多节点时区分
	// 本地监听端口，支持多个，逗号分隔，格式：tcp:11010,udp:11011 或 12345（基准端口）
	ListenPorts   string `gorm:"size:500" json:"listen_ports"`
	NoListener    bool   `gorm:"default:false" json:"no_listener"` // --no-listener：不监听任何端口，只连接对等节点
	// 映射监听器（用于 NAT 后公告外部地址），逗号分隔，格式：tcp://1.2.3.4:11010
	MappedListeners string `gorm:"size:500" json:"mapped_listeners"`
	// 子网代理（将本机子网共享给虚拟网络），逗号分隔，格式：192.168.1.0/24 或 192.168.1.0/24->10.0.0.0/24
	ProxyCidrs string `gorm:"size:500" json:"proxy_cidrs"`
	// 出口节点（使用其他节点作为出口），逗号分隔，格式：10.0.0.1
	ExitNodes string `gorm:"size:500" json:"exit_nodes"`
	// 外部节点（公共共享节点，用于发现对等节点），逗号分隔
	ExternalNodes string `gorm:"size:500" json:"external_nodes"`

	// ===== RPC 设置 =====
	RpcPortal          string `gorm:"size:100" json:"rpc_portal"`           // --rpc-portal：RPC 管理门户地址，如 0 或 12345 或 0.0.0.0:12345
	RpcPortalWhitelist string `gorm:"size:500" json:"rpc_portal_whitelist"` // --rpc-portal-whitelist：RPC 门户白名单

	// ===== 网络行为选项 =====
	NoTun               bool `gorm:"default:false" json:"no_tun"`                // --no-tun：不创建 TUN 虚拟网卡（无需 WinPcap/Npcap）
	EnableDhcp          bool `gorm:"default:false" json:"enable_dhcp"`           // --dhcp：DHCP 自动分配虚拟 IP
	DisableP2P          bool `gorm:"default:false" json:"disable_p2p"`           // --disable-p2p：禁用 P2P 直连，强制走中继
	P2POnly             bool `gorm:"default:false" json:"p2p_only"`              // --p2p-only：仅 P2P，禁用中继
	LatencyFirst        bool `gorm:"default:false" json:"latency_first"`         // --latency-first：延迟优先路由
	EnableExitNode      bool `gorm:"default:false" json:"enable_exit_node"`      // --enable-exit-node：允许本节点作为出口节点
	RelayAllPeerRpc     bool `gorm:"default:false" json:"relay_all_peer_rpc"`    // --relay-all-peer-rpc：中继所有对等 RPC
	ProxyForwardBySystem bool `gorm:"default:false" json:"proxy_forward_by_system"` // --proxy-forward-by-system：通过系统内核转发子网代理包
	DefaultProtocol    string `gorm:"size:20" json:"default_protocol"`           // --default-protocol：连接对等节点时使用的默认协议

	// ===== 打洞选项 =====
	DisableUdpHolePunching bool `gorm:"default:false" json:"disable_udp_hole_punching"` // --disable-udp-hole-punching
	DisableTcpHolePunching bool `gorm:"default:false" json:"disable_tcp_hole_punching"` // --disable-tcp-hole-punching
	DisableSymHolePunching bool `gorm:"default:false" json:"disable_sym_hole_punching"` // --disable-sym-hole-punching（对称 NAT）

	// ===== 协议加速选项 =====
	EnableKcpProxy   bool `gorm:"default:false" json:"enable_kcp_proxy"`   // --enable-kcp-proxy：KCP 加速代理
	DisableKcpInput  bool `gorm:"default:false" json:"disable_kcp_input"`  // --disable-kcp-input：禁止其他节点使用 KCP 代理到本节点
	EnableQuicProxy  bool `gorm:"default:false" json:"enable_quic_proxy"`  // --enable-quic-proxy：QUIC 加速代理
	DisableQuicInput bool `gorm:"default:false" json:"disable_quic_input"` // --disable-quic-input：禁止其他节点使用 QUIC 代理到本节点
	QuicListenPort   int  `gorm:"default:0" json:"quic_listen_port"`       // --quic-listen-port：QUIC 监听端口（0 为随机）

	// ===== TUN/网卡选项 =====
	DevName      string `gorm:"size:100" json:"dev_name"`         // --dev-name：自定义 TUN 设备名
	UseSmoltcp   bool   `gorm:"default:false" json:"use_smoltcp"` // --use-smoltcp：使用 smoltcp 用户态协议栈
	DisableIpv6  bool   `gorm:"default:false" json:"disable_ipv6"` // --disable-ipv6：禁用 IPv6
	Mtu          int    `gorm:"default:0" json:"mtu"`             // --mtu：MTU 大小（0 表示使用默认值）
	AcceptDns    bool   `gorm:"default:false" json:"accept_dns"`  // --accept-dns：启用 Magic DNS
	TldDnsZone   string `gorm:"size:100" json:"tld_dns_zone"`     // --tld-dns-zone：Magic DNS 顶级域名区域（如 et.net.）
	BindDevice   string `gorm:"size:100" json:"bind_device"`      // --bind-device：绑定物理设备名称

	// ===== 安全选项 =====
	DisableEncryption   bool   `gorm:"default:false" json:"disable_encryption"`    // --disable-encryption：禁用加密（不推荐）
	EncryptionAlgorithm string `gorm:"size:50" json:"encryption_algorithm"`        // --encryption-algorithm：加密算法
	PrivateMode         bool   `gorm:"default:false" json:"private_mode"`          // --private-mode：私有模式（仅允许已知节点）

	// ===== 中继选项 =====
	RelayNetworkWhitelist      string `gorm:"size:500" json:"relay_network_whitelist"`       // --relay-network-whitelist：允许中继的网络白名单
	ForeignRelayBpsLimit       int64  `gorm:"default:0" json:"foreign_relay_bps_limit"`      // --foreign-relay-bps-limit：限制转发流量带宽（bps，0不限制）
	DisableRelayKcp            bool   `gorm:"default:false" json:"disable_relay_kcp"`        // --disable-relay-kcp：禁止转发 KCP 数据包
	EnableRelayForeignNetworkKcp bool `gorm:"default:false" json:"enable_relay_foreign_network_kcp"` // --enable-relay-foreign-network-kcp：作为共享节点时转发其他网络 KCP 包

	// ===== 流量控制 =====
	TcpWhitelist string `gorm:"size:500" json:"tcp_whitelist"` // --tcp-whitelist：TCP 端口白名单，如 80,8000-9000
	UdpWhitelist string `gorm:"size:500" json:"udp_whitelist"` // --udp-whitelist：UDP 端口白名单，如 53,5000-6000
	Compression  string `gorm:"size:20" json:"compression"`   // --compression：压缩算法（none/zstd）

	// ===== STUN 服务器 =====
	StunServers   string `gorm:"type:text" json:"stun_servers"`    // --stun-servers：覆盖默认 STUN 服务器列表（逗号分隔）
	StunServersV6 string `gorm:"type:text" json:"stun_servers_v6"` // --stun-servers-v6：覆盖默认 IPv6 STUN 服务器列表

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

	// ===== 日志选项 =====
	ConsoleLogLevel string `gorm:"size:20" json:"console_log_level"` // --console-log-level：控制台日志级别
	FileLogLevel    string `gorm:"size:20" json:"file_log_level"`    // --file-log-level：文件日志级别
	FileLogDir      string `gorm:"size:500" json:"file_log_dir"`     // --file-log-dir：日志文件目录
	FileLogSize     int    `gorm:"default:0" json:"file_log_size"`   // --file-log-size：单个日志文件大小（MB，0使用默认100MB）
	FileLogCount    int    `gorm:"default:0" json:"file_log_count"`  // --file-log-count：最大日志文件数量（0使用默认10）

	// ===== 运行时选项 =====
	MultiThread      bool `gorm:"default:false" json:"multi_thread"`       // --multi-thread：启用多线程运行时
	MultiThreadCount int  `gorm:"default:0" json:"multi_thread_count"`     // --multi-thread-count：线程数（0使用默认2，需>2）

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
	MachineID        string `gorm:"size:255" json:"machine_id"` // --machine-id：Web 配置服务器用于识别机器的唯一 ID

	ListenAddr string `gorm:"size:100;default:'0.0.0.0'" json:"listen_addr"`
	// 监听端口，支持多个，逗号分隔，格式：tcp:11010,udp:11011 或 12345（基准端口）
	ListenPorts     string `gorm:"size:500" json:"listen_ports"`
	NetworkName     string `gorm:"size:255" json:"network_name"`
	NetworkPassword string `gorm:"size:255" json:"network_password"`
	Hostname        string `gorm:"size:255" json:"hostname"`     // --hostname：自定义节点主机名
	InstanceName    string `gorm:"size:255" json:"instance_name"` // --instance-name：实例名称，同机多节点时区分

	// ===== RPC 设置 =====
	RpcPortal          string `gorm:"size:100" json:"rpc_portal"`           // --rpc-portal：RPC 管理门户地址
	RpcPortalWhitelist string `gorm:"size:500" json:"rpc_portal_whitelist"` // --rpc-portal-whitelist：RPC 门户白名单

	// ===== 网络行为选项 =====
	NoTun               bool   `gorm:"default:false" json:"no_tun"`                // --no-tun：不创建 TUN 虚拟网卡
	DisableP2P          bool   `gorm:"default:false" json:"disable_p2p"`           // --disable-p2p：禁用 P2P 直连
	RelayAllPeerRpc     bool   `gorm:"default:false" json:"relay_all_peer_rpc"`    // --relay-all-peer-rpc：中继所有对等 RPC
	EnableExitNode      bool   `gorm:"default:false" json:"enable_exit_node"`      // --enable-exit-node：允许作为出口节点
	DefaultProtocol     string `gorm:"size:20" json:"default_protocol"`            // --default-protocol：连接对等节点时使用的默认协议
	ProxyForwardBySystem bool  `gorm:"default:false" json:"proxy_forward_by_system"` // --proxy-forward-by-system：通过系统内核转发子网代理包

	// ===== 协议加速选项 =====
	EnableKcpProxy   bool `gorm:"default:false" json:"enable_kcp_proxy"`   // --enable-kcp-proxy
	DisableKcpInput  bool `gorm:"default:false" json:"disable_kcp_input"`  // --disable-kcp-input
	EnableQuicProxy  bool `gorm:"default:false" json:"enable_quic_proxy"`  // --enable-quic-proxy
	DisableQuicInput bool `gorm:"default:false" json:"disable_quic_input"` // --disable-quic-input
	QuicListenPort   int  `gorm:"default:0" json:"quic_listen_port"`       // --quic-listen-port

	// ===== 安全选项 =====
	DisableEncryption   bool   `gorm:"default:false" json:"disable_encryption"`    // --disable-encryption
	EncryptionAlgorithm string `gorm:"size:50" json:"encryption_algorithm"`        // --encryption-algorithm：加密算法
	PrivateMode         bool   `gorm:"default:false" json:"private_mode"`          // --private-mode：私有模式

	// ===== 中继选项 =====
	RelayNetworkWhitelist      string `gorm:"size:500" json:"relay_network_whitelist"`       // --relay-network-whitelist
	ForeignRelayBpsLimit       int64  `gorm:"default:0" json:"foreign_relay_bps_limit"`      // --foreign-relay-bps-limit：限制转发流量带宽
	DisableRelayKcp            bool   `gorm:"default:false" json:"disable_relay_kcp"`        // --disable-relay-kcp
	EnableRelayForeignNetworkKcp bool `gorm:"default:false" json:"enable_relay_foreign_network_kcp"` // --enable-relay-foreign-network-kcp

	// ===== 流量控制 =====
	TcpWhitelist string `gorm:"size:500" json:"tcp_whitelist"` // --tcp-whitelist：TCP 端口白名单
	UdpWhitelist string `gorm:"size:500" json:"udp_whitelist"` // --udp-whitelist：UDP 端口白名单
	Compression  string `gorm:"size:20" json:"compression"`   // --compression：压缩算法（none/zstd）

	// ===== STUN 服务器 =====
	StunServers   string `gorm:"type:text" json:"stun_servers"`    // --stun-servers：覆盖默认 STUN 服务器列表
	StunServersV6 string `gorm:"type:text" json:"stun_servers_v6"` // --stun-servers-v6：覆盖默认 IPv6 STUN 服务器列表

	// ===== 手动路由 =====
	EnableManualRoutes bool   `gorm:"default:false" json:"enable_manual_routes"` // --manual-routes：启用手动路由
	ManualRoutes       string `gorm:"type:text" json:"manual_routes"`            // 手动路由列表，逗号分隔，格式：10.0.0.0/24

	// ===== 端口转发 =====
	// 格式：proto:bind_ip:bind_port:dst_ip:dst_port，多条用换行分隔，如 tcp:0.0.0.0:8080:192.168.1.1:80
	PortForwards string `gorm:"type:text" json:"port_forwards"`

	// ===== 日志选项 =====
	ConsoleLogLevel string `gorm:"size:20" json:"console_log_level"` // --console-log-level：控制台日志级别
	FileLogLevel    string `gorm:"size:20" json:"file_log_level"`    // --file-log-level：文件日志级别
	FileLogDir      string `gorm:"size:500" json:"file_log_dir"`     // --file-log-dir：日志文件目录
	FileLogSize     int    `gorm:"default:0" json:"file_log_size"`   // --file-log-size：单个日志文件大小（MB）
	FileLogCount    int    `gorm:"default:0" json:"file_log_count"`  // --file-log-count：最大日志文件数量

	// ===== 运行时选项 =====
	MultiThread      bool `gorm:"default:true" json:"multi_thread"`        // --multi-thread：启用多线程运行时
	MultiThreadCount int  `gorm:"default:0" json:"multi_thread_count"`     // --multi-thread-count：线程数（0使用默认2，需>2）

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
	// Webhook 通知（参考 lucky DDNSTask.Webhook）
	WebhookEnable  bool   `gorm:"default:false" json:"webhook_enable"`
	WebhookURL     string `gorm:"size:500" json:"webhook_url"`
	WebhookMethod  string `gorm:"size:10;default:'POST'" json:"webhook_method"`
	WebhookHeaders string `gorm:"type:text" json:"webhook_headers"` // JSON 数组，格式：[{"key":"X-Token","value":"xxx"}]
	WebhookBody    string `gorm:"type:text" json:"webhook_body"`    // 支持变量：{ip},{domain},{type}
	// 强制更新间隔（参考 lucky ForceInterval），0=禁用，单位秒
	ForceInterval int `gorm:"default:0" json:"force_interval"`
	// HTTP 客户端超时（参考 lucky HttpClientTimeout），单位秒
	HttpTimeout int    `gorm:"default:10" json:"http_timeout"`
	Status      string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError   string `gorm:"type:text" json:"last_error"`
	Remark      string `gorm:"size:500" json:"remark"`
}

// DDNSHistory DDNS IP 变化历史记录（参考 ddnsgo 历史功能）
type DDNSHistory struct {
	BaseModel
	TaskID   uint   `gorm:"not null;index" json:"task_id"`
	OldIP    string `gorm:"size:100" json:"old_ip"`
	NewIP    string `gorm:"size:100" json:"new_ip"`
	Domain   string `gorm:"size:255" json:"domain"`
	Provider string `gorm:"size:50" json:"provider"`
	Success  bool   `gorm:"default:true" json:"success"`
	Message  string `gorm:"type:text" json:"message"`
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
	// 多上游地址（负载均衡），JSON 数组，参考 lucky Locations 多目标
	UpstreamAddrs string `gorm:"type:text" json:"upstream_addrs"`
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
	// 安全特性（参考 lucky SubReverProxyRule）
	EnableBasicAuth bool   `gorm:"default:false" json:"enable_basic_auth"` // 启用 BasicAuth
	BasicAuthUser   string `gorm:"size:100" json:"basic_auth_user"`
	BasicAuthPasswd string `gorm:"size:255" json:"basic_auth_passwd"`
	// IP 过滤模式（参考 lucky SafeIPMode）
	SafeIPMode string `gorm:"size:20" json:"safe_ip_mode"` // blacklist/whitelist/""（空表示不过滤）
	// UserAgent 过滤（参考 lucky SafeUserAgentMode）
	SafeUAMode string `gorm:"size:20" json:"safe_ua_mode"` // blacklist/whitelist/""
	UAFilter   string `gorm:"type:text" json:"ua_filter"`  // JSON 数组
	// 自定义 robots.txt（参考 lucky CustomRobotTxt）
	CustomRobotsTxt bool   `gorm:"default:false" json:"custom_robots_txt"`
	RobotsTxt       string `gorm:"type:text" json:"robots_txt"`
	// 访问日志
	EnableAccessLog bool `gorm:"default:false" json:"enable_access_log"`
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

// ===== 证书账号 =====

// CertAccount SSL 证书申请账号（ACME CA 账号，参考 dnsmgr cert_account 表设计）
type CertAccount struct {
	BaseModel
	Name string `gorm:"size:100;not null" json:"name"`
	// CA 类型：letsencrypt/zerossl/buypass/google
	Type string `gorm:"size:50;not null" json:"type"`
	// ACME 注册邮箱
	Email string `gorm:"size:255" json:"email"`
	// EAB（External Account Binding）凭据，ZeroSSL / Google Trust Services 等需要
	EabKid     string `gorm:"size:255" json:"eab_kid"`
	EabHmacKey string `gorm:"size:500" json:"eab_hmac_key"`
	// 扩展信息（ACME 注册后返回的账号 URL 等）
	Ext    string `gorm:"type:text" json:"ext"`
	Remark string `gorm:"size:500" json:"remark"`
}

// ===== 域名证书 =====

// DomainCert ACME 域名证书
type DomainCert struct {
	BaseModel
	Name            string      `gorm:"size:100;not null" json:"name"`
	Domains         string      `gorm:"type:text;not null" json:"domains"` // JSON 数组
	CA              string      `gorm:"size:50;default:'letsencrypt'" json:"ca"` // letsencrypt/zerossl/buypass/google
	ChallengeType   string      `gorm:"size:20;default:'dns'" json:"challenge_type"` // dns/http
	DomainAccountID uint        `json:"domain_account_id"`
	// 关联证书账号（ACME CA 账号）
	CertAccountID   uint        `json:"cert_account_id"`
	CertAccount     CertAccount `gorm:"foreignKey:CertAccountID" json:"cert_account,omitempty"`
	CertFile        string      `gorm:"size:500" json:"cert_file"`
	KeyFile         string      `gorm:"size:500" json:"key_file"`
	ExpireAt        *time.Time  `json:"expire_at"`
	AutoRenew       bool        `gorm:"default:true" json:"auto_renew"`
	// 提前续期天数，默认 7 天
	RenewBeforeDays int         `gorm:"default:7" json:"renew_before_days"`
	Status          string      `gorm:"size:20;default:'pending'" json:"status"` // pending/valid/expired/error/applying
	LastError       string      `gorm:"type:text" json:"last_error"`
	Remark          string      `gorm:"size:500" json:"remark"`
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

// IPDBSubscription IP 地址库订阅
type IPDBSubscription struct {
	BaseModel
	Name       string     `gorm:"size:100;not null" json:"name"`
	Enable     bool       `gorm:"default:true" json:"enable"`
	URL        string     `gorm:"size:1000;not null" json:"url"`
	Location   string     `gorm:"size:255" json:"location"`   // 默认归属地（可选）
	Tags       string     `gorm:"size:500" json:"tags"`       // 默认标签（可选）
	ClearFirst bool       `gorm:"default:false" json:"clear_first"` // 刷新前是否清空
	// 自动刷新间隔（小时），0 表示不自动刷新
	Interval      int        `gorm:"default:0" json:"interval"`
	LastSyncTime  *time.Time `json:"last_sync_time"`
	LastSyncCount int        `json:"last_sync_count"` // 上次同步条目数
	LastSyncError string     `gorm:"type:text" json:"last_sync_error"`
	Remark        string     `gorm:"size:500" json:"remark"`
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

// ===== WAF 防火墙 =====

// WafConfig Coraza WAF 配置（参考 coraza WAF 和 lucky 安全模块）
type WafConfig struct {
	BaseModel
	Name   string `gorm:"size:100;not null" json:"name"`
	Enable bool   `gorm:"default:false" json:"enable"`
	// 规则集配置
	EnableCRS  bool   `gorm:"default:true" json:"enable_crs"`   // 启用 OWASP CRS 规则集
	CRSVersion string `gorm:"size:20;default:'4.0'" json:"crs_version"` // CRS 版本
	// 自定义规则（SecLang 格式）
	CustomRules string `gorm:"type:text" json:"custom_rules"`
	// 审计日志
	AuditLogEnable bool   `gorm:"default:true" json:"audit_log_enable"`
	AuditLogPath   string `gorm:"size:500" json:"audit_log_path"`
	// 拦截模式：detection（检测模式，只记录不拦截）/ prevention（防护模式，拦截恶意请求）
	Mode string `gorm:"size:20;default:'detection'" json:"mode"`
	// 绑定到哪些网站服务（JSON 数组，存储 CaddySite.ID 列表）
	BindSiteIDs string `gorm:"type:text" json:"bind_site_ids"`
	Status      string `gorm:"size:20;default:'stopped'" json:"status"`
	LastError   string `gorm:"type:text" json:"last_error"`
	Remark      string `gorm:"size:500" json:"remark"`
}

// WafLog WAF 拦截/检测日志
type WafLog struct {
	BaseModel
	WafConfigID uint   `gorm:"not null;index" json:"waf_config_id"`
	ClientIP    string `gorm:"size:100;index" json:"client_ip"`
	Method      string `gorm:"size:10" json:"method"`
	URI         string `gorm:"size:1000" json:"uri"`
	RuleID      int    `gorm:"index" json:"rule_id"`
	RuleMsg     string `gorm:"size:500" json:"rule_msg"`
	Severity    string `gorm:"size:20" json:"severity"` // CRITICAL/ERROR/WARNING/NOTICE
	Action      string `gorm:"size:20" json:"action"`   // block/detect
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
