package ddns

import (
	"fmt"
	"strings"
)

// DNSProvider DNS 服务商接口
type DNSProvider interface {
	UpdateRecord(domain, recordType, ip, ttl string) error
}

// NewProvider 创建 DNS 服务商实例
func NewProvider(name, accessID, accessSecret string) DNSProvider {
	switch strings.ToLower(name) {
	case "alidns", "aliyun":
		return &AliDNSProvider{AccessKeyID: accessID, AccessKeySecret: accessSecret}
	case "cloudflare", "cf":
		return &CloudflareProvider{APIToken: accessSecret}
	case "dnspod":
		return &DnspodProvider{SecretID: accessID, SecretKey: accessSecret}
	case "callback", "webhook":
		return &WebhookProvider{URL: accessID, Method: accessSecret}
	default:
		return nil
	}
}

// ===== 阿里云 DNS =====

type AliDNSProvider struct {
	AccessKeyID     string
	AccessKeySecret string
}

func (p *AliDNSProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	// TODO: 调用阿里云 DNS API
	// 参考 ddns-go 的 alidns.go 实现
	return fmt.Errorf("阿里云 DNS 更新待实现: %s -> %s", domain, ip)
}

// ===== Cloudflare =====

type CloudflareProvider struct {
	APIToken string
}

func (p *CloudflareProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	// TODO: 调用 Cloudflare API
	return fmt.Errorf("Cloudflare DNS 更新待实现: %s -> %s", domain, ip)
}

// ===== DNSPod =====

type DnspodProvider struct {
	SecretID  string
	SecretKey string
}

func (p *DnspodProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	// TODO: 调用 DNSPod API
	return fmt.Errorf("DNSPod DNS 更新待实现: %s -> %s", domain, ip)
}

// ===== Webhook =====

type WebhookProvider struct {
	URL    string
	Method string
}

func (p *WebhookProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	// TODO: 发送 Webhook 请求
	return fmt.Errorf("Webhook DNS 更新待实现: %s -> %s", domain, ip)
}
