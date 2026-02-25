package connection

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/beego/beego"
	"github.com/djylb/nps/lib/logs"
	"github.com/djylb/nps/lib/mux"
	"github.com/djylb/nps/lib/pmux"
)

var pMux *pmux.PortMux
var BridgeIp string
var BridgeHost string
var BridgeTcpIp string
var BridgeKcpIp string
var BridgeQuicIp string
var BridgeTlsIp string
var BridgeWsIp string
var BridgeWssIp string
var BridgePort int
var BridgeTcpPort int
var BridgeKcpPort int
var BridgeQuicPort int
var BridgeTlsPort int
var BridgeWsPort int
var BridgeWssPort int
var BridgePath string
var BridgeTrustedIps string
var BridgeRealIpHeader string
var HttpIp string
var HttpPort int
var HttpsPort int
var Http3Port int
var WebIp string
var WebPort int
var P2pIp string
var P2pPort int
var QuicAlpn []string
var QuicKeepAliveSec int
var QuicIdleTimeoutSec int
var QuicMaxStreams int64
var MuxPingIntervalSec int

func InitConnectionService() {
	BridgeIp = beego.AppConfig.DefaultString("bridge_ip", beego.AppConfig.DefaultString("bridge_tcp_ip", "0.0.0.0"))
	BridgeHost = beego.AppConfig.DefaultString("bridge_host", "")
	BridgeTcpIp = beego.AppConfig.DefaultString("bridge_tcp_ip", BridgeIp)
	BridgeKcpIp = beego.AppConfig.DefaultString("bridge_kcp_ip", BridgeIp)
	BridgeQuicIp = beego.AppConfig.DefaultString("bridge_quic_ip", BridgeIp)
	BridgeTlsIp = beego.AppConfig.DefaultString("bridge_tls_ip", BridgeIp)
	BridgeWsIp = beego.AppConfig.DefaultString("bridge_ws_ip", BridgeIp)
	BridgeWssIp = beego.AppConfig.DefaultString("bridge_wss_ip", BridgeIp)
	BridgePort = beego.AppConfig.DefaultInt("bridge_port", beego.AppConfig.DefaultInt("bridge_tcp_port", 0))
	BridgeTcpPort = beego.AppConfig.DefaultInt("bridge_tcp_port", BridgePort)
	BridgeKcpPort = beego.AppConfig.DefaultInt("bridge_kcp_port", BridgePort)
	BridgeQuicPort = beego.AppConfig.DefaultInt("bridge_quic_port", 0)
	BridgeTlsPort = beego.AppConfig.DefaultInt("bridge_tls_port", beego.AppConfig.DefaultInt("tls_bridge_port", 0))
	BridgeWsPort = beego.AppConfig.DefaultInt("bridge_ws_port", 0)
	BridgeWssPort = beego.AppConfig.DefaultInt("bridge_wss_port", 0)
	BridgePath = beego.AppConfig.DefaultString("bridge_path", "/ws")
	BridgeTrustedIps = beego.AppConfig.String("bridge_trusted_ips")
	BridgeRealIpHeader = beego.AppConfig.String("bridge_real_ip_header")
	HttpIp = beego.AppConfig.DefaultString("http_proxy_ip", "0.0.0.0")
	HttpPort = beego.AppConfig.DefaultInt("http_proxy_port", 0)
	HttpsPort = beego.AppConfig.DefaultInt("https_proxy_port", 0)
	Http3Port = beego.AppConfig.DefaultInt("http3_proxy_port", HttpsPort)
	WebIp = beego.AppConfig.DefaultString("web_ip", "0.0.0.0")
	WebPort = beego.AppConfig.DefaultInt("web_port", 0)
	P2pIp = beego.AppConfig.DefaultString("p2p_ip", "0.0.0.0")
	P2pPort = beego.AppConfig.DefaultInt("p2p_port", 0)
	quicAlpnList := beego.AppConfig.DefaultString("quic_alpn", "nps")
	QuicAlpn = strings.Split(quicAlpnList, ",")
	QuicKeepAliveSec = beego.AppConfig.DefaultInt("quic_keep_alive_period", 10)
	QuicIdleTimeoutSec = beego.AppConfig.DefaultInt("quic_max_idle_timeout", 30)
	QuicMaxStreams = beego.AppConfig.DefaultInt64("quic_max_incoming_streams", 100000)
	MuxPingIntervalSec = beego.AppConfig.DefaultInt("mux_ping_interval", 5)
	mux.PingInterval = time.Duration(MuxPingIntervalSec) * time.Second

	if BridgePort != 0 && (HttpPort == BridgePort || HttpsPort == BridgePort || WebPort == BridgePort || BridgeTlsPort == BridgePort) {
		if BridgePort <= 0 || BridgePort > 65535 {
			logs.Error("Invalid bridge port %d", BridgePort)
			return
		}
		pMux = pmux.NewPortMux(BridgePort, beego.AppConfig.String("web_host"), beego.AppConfig.String("bridge_host"))
	}
}

func GetBridgeTcpListener() (net.Listener, error) {
	logs.Info("server start, the bridge type is tcp, the bridge port is %d", BridgeTcpPort)
	if BridgeTcpPort <= 0 || BridgeTcpPort > 65535 {
		return nil, fmt.Errorf("invalid tcp bridge port %d", BridgeTcpPort)
	}
	if pMux != nil && BridgeTcpPort == BridgePort {
		return pMux.GetClientListener(), nil
	}
	return net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(BridgeTcpIp), Port: BridgeTcpPort})
}

func GetBridgeTlsListener() (net.Listener, error) {
	logs.Info("server start, the bridge type is tls, the bridge port is %d", BridgeTlsPort)
	if BridgeTlsPort <= 0 || BridgeTlsPort > 65535 {
		return nil, fmt.Errorf("invalid tls bridge port %d", BridgeTlsPort)
	}
	if pMux != nil && BridgeTlsPort == BridgePort {
		return pMux.GetClientTlsListener(), nil
	}
	return net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(BridgeTlsIp), Port: BridgeTlsPort})
}

func GetBridgeWsListener() (net.Listener, error) {
	logs.Info("server start, the bridge type is ws, the bridge port is %d, the bridge path is %s", BridgeWsPort, BridgePath)
	if BridgeWsPort <= 0 || BridgeWsPort > 65535 {
		return nil, fmt.Errorf("invalid ws bridge port %d", BridgeWsPort)
	}
	if pMux != nil && BridgeWsPort == BridgePort {
		return pMux.GetClientWsListener(), nil
	}
	return net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(BridgeWsIp), Port: BridgeWsPort})
}

func GetBridgeWssListener() (net.Listener, error) {
	logs.Info("server start, the bridge type is wss, the bridge port is %d, the bridge path is %s", BridgeWssPort, BridgePath)
	if BridgeWssPort <= 0 || BridgeWssPort > 65535 {
		return nil, fmt.Errorf("invalid wss bridge port %d", BridgeWssPort)
	}
	if pMux != nil && BridgeWssPort == BridgePort {
		return pMux.GetClientWssListener(), nil
	}
	return net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(BridgeWssIp), Port: BridgeWssPort})
}

func GetHttpListener() (net.Listener, error) {
	if pMux != nil && HttpPort == BridgePort {
		logs.Info("start http listener, port is %d", BridgePort)
		return pMux.GetHttpListener(), nil
	}
	logs.Info("start http listener, port is %d", HttpPort)
	return getTcpListener(HttpIp, HttpPort)
}

func GetHttpsListener() (net.Listener, error) {
	if pMux != nil && HttpsPort == BridgePort {
		logs.Info("start https listener, port is %d", BridgePort)
		return pMux.GetHttpsListener(), nil
	}
	logs.Info("start https listener, port is %d", HttpsPort)
	return getTcpListener(HttpIp, HttpsPort)
}

func GetWebManagerListener() (net.Listener, error) {
	if pMux != nil && WebPort == BridgePort {
		logs.Info("Web management start, access port is %d", BridgePort)
		return pMux.GetManagerListener(), nil
	}
	logs.Info("web management start, access port is %d", WebPort)
	return getTcpListener(WebIp, WebPort)
}

func getTcpListener(ip string, port int) (net.Listener, error) {
	if port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid tcp port %d", port)
	}
	if ip == "" {
		ip = "0.0.0.0"
	}
	return net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(ip), Port: port})
}
