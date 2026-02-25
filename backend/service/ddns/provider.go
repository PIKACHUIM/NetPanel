package ddns

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DNSProvider DNS 服务商接口
type DNSProvider interface {
	// UpdateRecord 更新或创建 DNS 记录
	// domain: 完整域名（如 home.example.com）
	// recordType: A 或 AAAA
	// ip: 新的 IP 地址
	// ttl: TTL 字符串（如 "600"）
	UpdateRecord(domain, recordType, ip, ttl string) error
}

// NewProvider 创建 DNS 服务商实例
func NewProvider(name, accessID, accessSecret string) DNSProvider {
	switch strings.ToLower(name) {
	case "alidns", "aliyun":
		return &AliDNSProvider{AccessKeyID: accessID, AccessKeySecret: accessSecret}
	case "cloudflare", "cf":
		return &CloudflareProvider{APIToken: accessSecret, ZoneID: accessID}
	case "dnspod":
		return &DnspodProvider{SecretID: accessID, SecretKey: accessSecret}
	case "webhook":
		return &WebhookProvider{URL: accessID, Method: accessSecret}
	default:
		return nil
	}
}

// splitDomain 将完整域名拆分为主机记录和根域名
// 例如 home.example.com -> ("home", "example.com")
// 例如 example.com -> ("@", "example.com")
func splitDomain(fullDomain string) (rr, domain string) {
	parts := strings.Split(fullDomain, ".")
	if len(parts) <= 2 {
		return "@", fullDomain
	}
	return strings.Join(parts[:len(parts)-2], "."), strings.Join(parts[len(parts)-2:], ".")
}

// httpGet 发送 GET 请求并返回响应体
func httpGet(reqURL string, headers map[string]string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// httpJSON 发送 JSON 请求
func httpJSON(method, reqURL string, headers map[string]string, payload interface{}) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// ===== 阿里云 DNS =====

// AliDNSProvider 阿里云 DNS 服务商
type AliDNSProvider struct {
	AccessKeyID     string
	AccessKeySecret string
}

const aliDNSEndpoint = "https://alidns.aliyuncs.com/"

// aliSign 生成阿里云 API 签名
func (p *AliDNSProvider) aliSign(params map[string]string) string {
	// 排序参数
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建规范化查询字符串
	var parts []string
	for _, k := range keys {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(params[k]))
	}
	canonicalized := strings.Join(parts, "&")

	// 构建待签名字符串
	stringToSign := "GET&%2F&" + url.QueryEscape(canonicalized)

	// HMAC-SHA1 签名
	mac := hmac.New(sha1.New, []byte(p.AccessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// aliRequest 发送阿里云 DNS API 请求
func (p *AliDNSProvider) aliRequest(action string, params map[string]string) ([]byte, error) {
	params["Action"] = action
	params["AccessKeyId"] = p.AccessKeyID
	params["Format"] = "JSON"
	params["Version"] = "2015-01-09"
	params["SignatureMethod"] = "HMAC-SHA1"
	params["SignatureVersion"] = "1.0"
	params["SignatureNonce"] = strconv.FormatInt(time.Now().UnixNano(), 10)
	params["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	params["Signature"] = p.aliSign(params)

	reqURL := aliDNSEndpoint + "?" + buildQuery(params)
	return httpGet(reqURL, nil)
}

func buildQuery(params map[string]string) string {
	var parts []string
	for k, v := range params {
		parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
	}
	return strings.Join(parts, "&")
}

// UpdateRecord 更新阿里云 DNS 记录
func (p *AliDNSProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	rr, rootDomain := splitDomain(domain)

	// 查询现有记录
	listResp, err := p.aliRequest("DescribeDomainRecords", map[string]string{
		"DomainName": rootDomain,
		"RRKeyWord":  rr,
		"Type":       recordType,
	})
	if err != nil {
		return fmt.Errorf("查询阿里云 DNS 记录失败: %w", err)
	}

	var listResult struct {
		DomainRecords struct {
			Record []struct {
				RecordId string `json:"RecordId"`
				Value    string `json:"Value"`
			} `json:"Record"`
		} `json:"DomainRecords"`
	}
	if err := json.Unmarshal(listResp, &listResult); err != nil {
		return fmt.Errorf("解析阿里云 DNS 响应失败: %w", err)
	}

	ttlInt := "600"
	if ttl != "" {
		ttlInt = ttl
	}

	if len(listResult.DomainRecords.Record) > 0 {
		// 更新现有记录
		record := listResult.DomainRecords.Record[0]
		if record.Value == ip {
			return nil // IP 未变化
		}
		_, err = p.aliRequest("UpdateDomainRecord", map[string]string{
			"RecordId": record.RecordId,
			"RR":       rr,
			"Type":     recordType,
			"Value":    ip,
			"TTL":      ttlInt,
		})
		if err != nil {
			return fmt.Errorf("更新阿里云 DNS 记录失败: %w", err)
		}
	} else {
		// 新增记录
		_, err = p.aliRequest("AddDomainRecord", map[string]string{
			"DomainName": rootDomain,
			"RR":         rr,
			"Type":       recordType,
			"Value":      ip,
			"TTL":        ttlInt,
		})
		if err != nil {
			return fmt.Errorf("新增阿里云 DNS 记录失败: %w", err)
		}
	}
	return nil
}

// ===== Cloudflare =====

// CloudflareProvider Cloudflare DNS 服务商
type CloudflareProvider struct {
	APIToken string
	ZoneID   string // Zone ID（可选，若为空则自动查询）
}

const cfAPIBase = "https://api.cloudflare.com/client/v4"

func (p *CloudflareProvider) cfHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + p.APIToken,
		"Content-Type":  "application/json",
	}
}

// getZoneID 根据域名获取 Zone ID
func (p *CloudflareProvider) getZoneID(rootDomain string) (string, error) {
	if p.ZoneID != "" {
		return p.ZoneID, nil
	}
	body, err := httpGet(fmt.Sprintf("%s/zones?name=%s", cfAPIBase, rootDomain), p.cfHeaders())
	if err != nil {
		return "", fmt.Errorf("查询 Cloudflare Zone 失败: %w", err)
	}
	var result struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &result); err != nil || !result.Success || len(result.Result) == 0 {
		return "", fmt.Errorf("未找到域名 %s 对应的 Cloudflare Zone", rootDomain)
	}
	return result.Result[0].ID, nil
}

// UpdateRecord 更新 Cloudflare DNS 记录
func (p *CloudflareProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	_, rootDomain := splitDomain(domain)
	zoneID, err := p.getZoneID(rootDomain)
	if err != nil {
		return err
	}

	ttlInt := 1 // 1 = auto
	if ttl != "" {
		if v, err := strconv.Atoi(ttl); err == nil && v > 0 {
			ttlInt = v
		}
	}

	// 查询现有记录
	body, err := httpGet(
		fmt.Sprintf("%s/zones/%s/dns_records?type=%s&name=%s", cfAPIBase, zoneID, recordType, domain),
		p.cfHeaders(),
	)
	if err != nil {
		return fmt.Errorf("查询 Cloudflare DNS 记录失败: %w", err)
	}

	var listResult struct {
		Result []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"result"`
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(body, &listResult); err != nil {
		return fmt.Errorf("解析 Cloudflare DNS 响应失败: %w", err)
	}

	payload := map[string]interface{}{
		"type":    recordType,
		"name":    domain,
		"content": ip,
		"ttl":     ttlInt,
		"proxied": false,
	}

	if len(listResult.Result) > 0 {
		record := listResult.Result[0]
		if record.Content == ip {
			return nil // IP 未变化
		}
		// 更新记录
		_, err = httpJSON("PUT",
			fmt.Sprintf("%s/zones/%s/dns_records/%s", cfAPIBase, zoneID, record.ID),
			p.cfHeaders(), payload,
		)
		if err != nil {
			return fmt.Errorf("更新 Cloudflare DNS 记录失败: %w", err)
		}
	} else {
		// 新增记录
		_, err = httpJSON("POST",
			fmt.Sprintf("%s/zones/%s/dns_records", cfAPIBase, zoneID),
			p.cfHeaders(), payload,
		)
		if err != nil {
			return fmt.Errorf("新增 Cloudflare DNS 记录失败: %w", err)
		}
	}
	return nil
}

// ===== DNSPod（腾讯云）=====

// DnspodProvider DNSPod 服务商
type DnspodProvider struct {
	SecretID  string
	SecretKey string
}

const dnspodEndpoint = "https://dnspod.tencentcloudapi.com"

// tcSign 生成腾讯云 API v3 签名
func (p *DnspodProvider) tcSign(service, host, payload string, timestamp int64) (string, string) {
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// Step 1: 规范请求串
	canonicalRequest := strings.Join([]string{
		"POST",
		"/",
		"",
		"content-type:application/json; charset=utf-8\nhost:" + host + "\n",
		"content-type;host",
		hashSHA256(payload),
	}, "\n")

	// Step 2: 待签名字符串
	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		strconv.FormatInt(timestamp, 10),
		credentialScope,
		hashSHA256(canonicalRequest),
	}, "\n")

	// Step 3: 计算签名
	secretDate := hmacSHA256([]byte("TC3"+p.SecretKey), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	// Step 4: 构建 Authorization
	authorization := fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host, Signature=%s",
		p.SecretID, credentialScope, signature,
	)
	return authorization, date
}

func hashSHA256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

// dnspodRequest 发送 DNSPod API 请求
func (p *DnspodProvider) dnspodRequest(action string, payload interface{}) ([]byte, error) {
	host := "dnspod.tencentcloudapi.com"
	timestamp := time.Now().Unix()

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadStr := string(payloadBytes)

	authorization, _ := p.tcSign("dnspod", host, payloadStr, timestamp)

	headers := map[string]string{
		"Authorization":  authorization,
		"Content-Type":   "application/json; charset=utf-8",
		"Host":           host,
		"X-TC-Action":    action,
		"X-TC-Timestamp": strconv.FormatInt(timestamp, 10),
		"X-TC-Version":   "2021-03-23",
	}

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", dnspodEndpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// UpdateRecord 更新 DNSPod DNS 记录
func (p *DnspodProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	rr, rootDomain := splitDomain(domain)

	ttlInt := 600
	if ttl != "" {
		if v, err := strconv.Atoi(ttl); err == nil && v > 0 {
			ttlInt = v
		}
	}

	// 查询现有记录
	listBody, err := p.dnspodRequest("DescribeRecordList", map[string]interface{}{
		"Domain":     rootDomain,
		"Subdomain":  rr,
		"RecordType": recordType,
	})
	if err != nil {
		return fmt.Errorf("查询 DNSPod 记录失败: %w", err)
	}

	var listResult struct {
		Response struct {
			RecordList []struct {
				RecordId uint   `json:"RecordId"`
				Value    string `json:"Value"`
			} `json:"RecordList"`
			Error *struct {
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(listBody, &listResult); err != nil {
		return fmt.Errorf("解析 DNSPod 响应失败: %w", err)
	}
	if listResult.Response.Error != nil {
		return fmt.Errorf("DNSPod API 错误: %s", listResult.Response.Error.Message)
	}

	if len(listResult.Response.RecordList) > 0 {
		record := listResult.Response.RecordList[0]
		if record.Value == ip {
			return nil // IP 未变化
		}
		// 更新记录
		_, err = p.dnspodRequest("ModifyRecord", map[string]interface{}{
			"Domain":     rootDomain,
			"RecordId":   record.RecordId,
			"SubDomain":  rr,
			"RecordType": recordType,
			"RecordLine": "默认",
			"Value":      ip,
			"TTL":        ttlInt,
		})
		if err != nil {
			return fmt.Errorf("更新 DNSPod 记录失败: %w", err)
		}
	} else {
		// 新增记录
		_, err = p.dnspodRequest("CreateRecord", map[string]interface{}{
			"Domain":     rootDomain,
			"SubDomain":  rr,
			"RecordType": recordType,
			"RecordLine": "默认",
			"Value":      ip,
			"TTL":        ttlInt,
		})
		if err != nil {
			return fmt.Errorf("新增 DNSPod 记录失败: %w", err)
		}
	}
	return nil
}

// ===== Webhook =====

// WebhookProvider Webhook 服务商
type WebhookProvider struct {
	URL    string // Webhook URL，支持变量替换：{ip} {domain} {type}
	Method string // HTTP 方法，默认 GET
}

// UpdateRecord 发送 Webhook 请求
func (p *WebhookProvider) UpdateRecord(domain, recordType, ip, ttl string) error {
	method := strings.ToUpper(p.Method)
	if method == "" {
		method = "GET"
	}

	// 替换 URL 中的变量
	reqURL := p.URL
	reqURL = strings.ReplaceAll(reqURL, "{ip}", url.QueryEscape(ip))
	reqURL = strings.ReplaceAll(reqURL, "{domain}", url.QueryEscape(domain))
	reqURL = strings.ReplaceAll(reqURL, "{type}", url.QueryEscape(recordType))

	client := &http.Client{Timeout: 15 * time.Second}
	var req *http.Request
	var err error

	if method == "POST" || method == "PUT" {
		payload := map[string]string{
			"ip":     ip,
			"domain": domain,
			"type":   recordType,
			"ttl":    ttl,
		}
		data, _ := json.Marshal(payload)
		req, err = http.NewRequest(method, reqURL, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("创建 Webhook 请求失败: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, err = http.NewRequest(method, reqURL, nil)
		if err != nil {
			return fmt.Errorf("创建 Webhook 请求失败: %w", err)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("发送 Webhook 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Webhook 响应错误 HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// ===== MD5 工具（备用）=====

func md5Hash(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}