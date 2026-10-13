package utils

import (
	"net"
	"time"
)

// IPv6Info 本机 IPv6 检测结果
type IPv6Info struct {
	// GlobalAddresses 全局单播 IPv6 地址（排除 loopback/link-local/ULA）
	GlobalAddresses []string `json:"global_addresses"`
	// HasGlobal 是否存在公网可达的全局 IPv6 地址
	HasGlobal bool `json:"has_global"`
	// PublicReachable 通过外部端点实测 IPv6 出站可达性（无法实测时为 false）
	PublicReachable bool `json:"public_reachable"`
}

// GetIPv6Info 检测本机全局 IPv6 地址与出站可达性。
// 可达性实测走 https://api64.ipify.org（双栈域名，IPv6 优先），3 秒超时。
func GetIPv6Info() IPv6Info {
	info := IPv6Info{GlobalAddresses: []string{}}

	ifaces, err := net.Interfaces()
	if err != nil {
		return info
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := ipFromAddr(addr)
			if ip == nil || !isGlobalIPv6(ip) {
				continue
			}
			info.GlobalAddresses = append(info.GlobalAddresses, ip.String())
		}
	}
	info.HasGlobal = len(info.GlobalAddresses) > 0

	if info.HasGlobal {
		info.PublicReachable = probeIPv6Outbound()
	}
	return info
}

func ipFromAddr(addr net.Addr) net.IP {
	switch v := addr.(type) {
	case *net.IPNet:
		return v.IP
	case *net.IPAddr:
		return v.IP
	}
	return nil
}

// isGlobalIPv6 判断是否全局单播 IPv6（2000::/3，排除 fd00::/8 ULA 与 fe80::/10 链路本地）
func isGlobalIPv6(ip net.IP) bool {
	if ip.To4() != nil || ip.To16() == nil {
		return false
	}
	return ip.IsGlobalUnicast() && !ip.IsPrivate() && !ip.IsLinkLocalUnicast()
}

// probeIPv6Outbound 强制走 IPv6 拨号验证出站连通性
func probeIPv6Outbound() bool {
	d := net.Dialer{Timeout: 3 * time.Second}
	conn, err := d.Dial("tcp6", "api64.ipify.org:443")
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
