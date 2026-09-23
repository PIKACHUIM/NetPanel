// Package safehttp 提供带 SSRF 防护的 HTTP 客户端，
// 用于所有「由用户提供 URL、服务端代为请求」的场景。
//
// 未加防护时，攻击者可让面板去访问 127.0.0.1、内网主机或云厂商元数据服务
// （169.254.169.254），借面板的网络位置读取本不可达的内容。
package safehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// blockedRanges 禁止访问的地址段：回环、私网、链路本地、
// 云元数据、保留段与 NAT64/文档用途段。
var blockedRanges []*net.IPNet

func init() {
	for _, cidr := range []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::1/128",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
		"64:ff9b::/96",
	} {
		if _, n, err := net.ParseCIDR(cidr); err == nil {
			blockedRanges = append(blockedRanges, n)
		}
	}
}

// IsBlockedIP 判断目标 IP 是否属于禁止访问的范围。
func IsBlockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}
	// IPv4-mapped IPv6（::ffff:127.0.0.1）需要按 IPv4 再判一次
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	for _, n := range blockedRanges {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// ValidateURL 校验用户提供的 URL 协议与主机是否可接受。
func ValidateURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("URL 格式无效: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("仅支持 http/https 协议")
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("URL 缺少主机名")
	}
	// 字面量 IP 可在发起连接前直接判定
	if ip := net.ParseIP(u.Hostname()); ip != nil && IsBlockedIP(ip) {
		return nil, fmt.Errorf("禁止访问内网或保留地址: %s", u.Hostname())
	}
	return u, nil
}

// NewClient 返回带 SSRF 防护的 HTTP 客户端。
//
// 校验放在 Control 回调里，作用于 DNS 解析之后、真正建立连接之前，
// 因此能挡住 DNS rebinding 与解析到内网的域名——只做 URL 层面的检查是不够的。
func NewClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout: 10 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			if ip := net.ParseIP(host); ip != nil && IsBlockedIP(ip) {
				return fmt.Errorf("禁止访问内网或保留地址: %s", host)
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		},
		// 重定向同样需要逐跳校验，否则 302 到 127.0.0.1 可绕过首跳检查
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("重定向次数过多")
			}
			if _, err := ValidateURL(req.URL.String()); err != nil {
				return err
			}
			return nil
		},
	}
}

// Get 校验 URL 后发起 GET 请求。
func Get(ctx context.Context, rawURL string, timeout time.Duration) (*http.Response, error) {
	if _, err := ValidateURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return NewClient(timeout).Do(req)
}
