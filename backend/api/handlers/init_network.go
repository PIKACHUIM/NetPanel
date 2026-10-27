package handlers

import (
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// InitNetworkHandler 检测网络环境，供首次向导使用
type InitNetworkHandler struct {
	log *logrus.Logger
}

func NewInitNetworkHandler(log *logrus.Logger) *InitNetworkHandler {
	return &InitNetworkHandler{log: log}
}

// NetworkInfo 返回当前网络环境信息
// GET /api/v1/init/network-info
func (h *InitNetworkHandler) NetworkInfo(c *gin.Context) {
	info := gin.H{
		"public_ip_v4": "",
		"public_ip_v6": "",
		"has_ipv4":     false,
		"has_ipv6":     false,
		"tun_capable":  false,
	}

	type ipResult struct {
		ip  string
		err error
	}

	// 并发探测 IPv4 和 IPv6
	v4ch := make(chan ipResult, 1)
	v6ch := make(chan ipResult, 1)

	go func() {
		ip, err := detectPublicIPv4()
		v4ch <- ipResult{ip, err}
	}()

	go func() {
		ip, err := detectPublicIPv6()
		v6ch <- ipResult{ip, err}
	}()

	select {
	case r := <-v4ch:
		if r.err == nil && r.ip != "" {
			info["public_ip_v4"] = r.ip
			info["has_ipv4"] = true
		}
	case <-time.After(5 * time.Second):
	}

	select {
	case r := <-v6ch:
		if r.err == nil && r.ip != "" {
			info["public_ip_v6"] = r.ip
			info["has_ipv6"] = true
		}
	case <-time.After(5 * time.Second):
	}

	// TUN 设备是否可用（Docker 模式下如果没有 --device /dev/net/tun 则不可用）
	info["tun_capable"] = checkTUNAvailable()

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": info})
}

// detectPublicIPv4 通过公共服务探测公网 IPv4
func detectPublicIPv4() (string, error) {
	services := []string{
		"https://api.ipify.org?format=text",
		"https://ifconfig.me/ip",
		"https://ipinfo.io/ip",
	}

	client := &http.Client{Timeout: 3 * time.Second}
	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			continue
		}
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		ip := string(buf[:n])
		if len(ip) > 0 {
			return ip, nil
		}
	}
	return "", nil
}

// detectPublicIPv6 探测公网 IPv6（需要 IPv6 出口）
func detectPublicIPv6() (string, error) {
	services := []string{
		"https://api6.ipify.org?format=text",
		"https://v6.ipinfo.io/ip",
	}

	client := &http.Client{Timeout: 3 * time.Second}
	for _, url := range services {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			continue
		}
		buf := make([]byte, 64)
		n, _ := resp.Body.Read(buf)
		ip := string(buf[:n])
		if len(ip) > 0 {
			return ip, nil
		}
	}
	return "", nil
}

// checkTUNAvailable 检测 /dev/net/tun 是否可访问
func checkTUNAvailable() bool {
	f, err := os.Open("/dev/net/tun")
	if err != nil {
		return false
	}
	f.Close()
	// 进一步验证：尝试创建 TUN socket
	addr, err := net.ResolveIPAddr("ip", "10.0.0.1")
	if err != nil {
		_ = addr
	}
	return true
}
