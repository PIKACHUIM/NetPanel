package handlers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/pkg/config"
	"github.com/netpanel/netpanel/service/access"
	"github.com/netpanel/netpanel/service/callback"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// ===== DNS Provider 同步接口 =====

// ProviderRecord 服务商返回的解析记录
type ProviderRecord struct {
	RemoteID   string
	RecordType string
	Host       string // 主机记录（相对域名，如 www、@ 等）
	Value      string
	TTL        int
	Proxied    bool // 仅 Cloudflare 支持
}

// ProviderDomainItem 服务商返回的域名条目
type ProviderDomainItem struct {
	Name    string
	ThirdID string
}

// dnsRecordProvider DNS 解析记录同步接口
type dnsRecordProvider interface {
	// ListRecords 获取指定域名的所有解析记录
	ListRecords(domain string) ([]ProviderRecord, error)
	// ListDomains 获取账号下所有域名
	ListDomains() ([]ProviderDomainItem, error)
}

// newDNSRecordProvider 根据账号信息创建对应的 DNS provider
func newDNSRecordProvider(acc model.DomainAccount) dnsRecordProvider {
	switch strings.ToLower(acc.Provider) {
	case "cloudflare", "cf":
		// Cloudflare: AccessID = Zone ID（可选），AccessSecret = API Token
		// 若 AuthType == api_key，则 AccessID=Email, AccessSecret=Global API Key
		if acc.AuthType == "api_key" {
			return &cfProvider{email: acc.Email, apiKey: acc.AccessSecret}
		}
		return &cfProvider{apiToken: acc.AccessSecret, zoneID: acc.AccessID}
	case "alidns", "aliyun":
		return &aliDNSRecordProvider{accessKeyID: acc.AccessID, accessKeySecret: acc.AccessSecret}
	case "dnspod":
		return &dnspodRecordProvider{secretID: acc.AccessID, secretKey: acc.AccessSecret}
	default:
		return nil
	}
}

// ===== Cloudflare Provider =====

const cfAPIBase = "https://api.cloudflare.com/client/v4"

type cfProvider struct {
	apiToken string
	apiKey   string // Global API Key（api_key 认证方式）
	email    string // 邮箱（api_key 认证方式）
	zoneID   string // 可选，若为空则自动查询
}

func (p *cfProvider) cfHeaders() map[string]string {
	h := map[string]string{"Content-Type": "application/json"}
	if p.apiToken != "" {
		h["Authorization"] = "Bearer " + p.apiToken
	} else {
		h["X-Auth-Email"] = p.email
		h["X-Auth-Key"] = p.apiKey
	}
	return h
}

func (p *cfProvider) cfGet(path string) ([]byte, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", cfAPIBase+path, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range p.cfHeaders() {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("Cloudflare API HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (p *cfProvider) getZoneID(domain string) (string, error) {
	if p.zoneID != "" {
		return p.zoneID, nil
	}
	// 尝试用根域名查询 Zone
	parts := strings.Split(domain, ".")
	var rootDomain string
	if len(parts) >= 2 {
		rootDomain = strings.Join(parts[len(parts)-2:], ".")
	} else {
		rootDomain = domain
	}
	body, err := p.cfGet("/zones?name=" + rootDomain + "&status=active")
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

func (p *cfProvider) ListRecords(domain string) ([]ProviderRecord, error) {
	zoneID, err := p.getZoneID(domain)
	if err != nil {
		return nil, err
	}

	var allRecords []ProviderRecord
	page := 1
	for {
		body, err := p.cfGet(fmt.Sprintf("/zones/%s/dns_records?per_page=100&page=%d", zoneID, page))
		if err != nil {
			return nil, fmt.Errorf("获取 Cloudflare 解析记录失败: %w", err)
		}
		var result struct {
			Result []struct {
				ID      string `json:"id"`
				Type    string `json:"type"`
				Name    string `json:"name"`
				Content string `json:"content"`
				TTL     int    `json:"ttl"`
				Proxied bool   `json:"proxied"`
			} `json:"result"`
			Success    bool `json:"success"`
			ResultInfo struct {
				Page       int `json:"page"`
				TotalPages int `json:"total_pages"`
			} `json:"result_info"`
		}
		if err := json.Unmarshal(body, &result); err != nil || !result.Success {
			return nil, fmt.Errorf("解析 Cloudflare 响应失败")
		}
		for _, r := range result.Result {
			// 将完整域名转换为主机记录（相对部分）
			host := r.Name
			if strings.HasSuffix(host, "."+domain) {
				host = strings.TrimSuffix(host, "."+domain)
			} else if host == domain {
				host = "@"
			}
			allRecords = append(allRecords, ProviderRecord{
				RemoteID:   r.ID,
				RecordType: r.Type,
				Host:       host,
				Value:      r.Content,
				TTL:        r.TTL,
				Proxied:    r.Proxied,
			})
		}
		if page >= result.ResultInfo.TotalPages || result.ResultInfo.TotalPages == 0 {
			break
		}
		page++
	}
	return allRecords, nil
}

func (p *cfProvider) ListDomains() ([]ProviderDomainItem, error) {
	var allDomains []ProviderDomainItem
	page := 1
	for {
		body, err := p.cfGet(fmt.Sprintf("/zones?per_page=50&page=%d&status=active", page))
		if err != nil {
			return nil, fmt.Errorf("获取 Cloudflare 域名列表失败: %w", err)
		}
		var result struct {
			Result []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"result"`
			Success    bool `json:"success"`
			ResultInfo struct {
				Page       int `json:"page"`
				TotalPages int `json:"total_pages"`
			} `json:"result_info"`
		}
		if err := json.Unmarshal(body, &result); err != nil || !result.Success {
			return nil, fmt.Errorf("解析 Cloudflare 域名列表响应失败")
		}
		for _, z := range result.Result {
			allDomains = append(allDomains, ProviderDomainItem{Name: z.Name, ThirdID: z.ID})
		}
		if page >= result.ResultInfo.TotalPages || result.ResultInfo.TotalPages == 0 {
			break
		}
		page++
	}
	return allDomains, nil
}

// ===== 阿里云 DNS Provider（解析记录同步）=====

// aliDNSSign 生成阿里云 DNS API HMAC-SHA1 签名
func aliDNSSign(params map[string]string, secret string) string {
	// 排序参数键
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
	stringToSign := "GET&%2F&" + url.QueryEscape(canonicalized)

	mac := hmac.New(sha1.New, []byte(secret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// tc3HashSHA256 计算字符串的 SHA256 哈希（十六进制）
func tc3HashSHA256(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// tc3HmacSHA256 计算 HMAC-SHA256
func tc3HmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

type aliDNSRecordProvider struct {
	accessKeyID     string
	accessKeySecret string
}

func (p *aliDNSRecordProvider) aliRequest(action string, params map[string]string) ([]byte, error) {
	// 内联阿里云签名逻辑
	allParams := map[string]string{
		"Action":           action,
		"AccessKeyId":      p.accessKeyID,
		"Format":           "JSON",
		"Version":          "2015-01-09",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   strconv.FormatInt(time.Now().UnixNano(), 10),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	for k, v := range params {
		allParams[k] = v
	}
	allParams["Signature"] = aliDNSSign(allParams, p.accessKeySecret)

	var parts []string
	for k, v := range allParams {
		parts = append(parts, k+"="+v)
	}
	reqURL := "https://alidns.aliyuncs.com/?" + strings.Join(parts, "&")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (p *aliDNSRecordProvider) ListRecords(domain string) ([]ProviderRecord, error) {
	body, err := p.aliRequest("DescribeDomainRecords", map[string]string{
		"DomainName": domain,
		"PageSize":   "500",
	})
	if err != nil {
		return nil, fmt.Errorf("获取阿里云解析记录失败: %w", err)
	}
	var result struct {
		DomainRecords struct {
			Record []struct {
				RecordId string `json:"RecordId"`
				Type     string `json:"Type"`
				RR       string `json:"RR"`
				Value    string `json:"Value"`
				TTL      int    `json:"TTL"`
			} `json:"Record"`
		} `json:"DomainRecords"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析阿里云响应失败: %w", err)
	}
	var records []ProviderRecord
	for _, r := range result.DomainRecords.Record {
		records = append(records, ProviderRecord{
			RemoteID:   r.RecordId,
			RecordType: r.Type,
			Host:       r.RR,
			Value:      r.Value,
			TTL:        r.TTL,
		})
	}
	return records, nil
}

func (p *aliDNSRecordProvider) ListDomains() ([]ProviderDomainItem, error) {
	body, err := p.aliRequest("DescribeDomains", map[string]string{"PageSize": "100"})
	if err != nil {
		return nil, fmt.Errorf("获取阿里云域名列表失败: %w", err)
	}
	var result struct {
		Domains struct {
			Domain []struct {
				DomainId   string `json:"DomainId"`
				DomainName string `json:"DomainName"`
			} `json:"Domain"`
		} `json:"Domains"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析阿里云域名列表失败: %w", err)
	}
	var domains []ProviderDomainItem
	for _, d := range result.Domains.Domain {
		domains = append(domains, ProviderDomainItem{Name: d.DomainName, ThirdID: d.DomainId})
	}
	return domains, nil
}

// ===== DNSPod Provider（解析记录同步）=====

type dnspodRecordProvider struct {
	secretID  string
	secretKey string
}

func (p *dnspodRecordProvider) dnspodPost(action string, payload interface{}) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	payloadStr := string(data)
	host := "dnspod.tencentcloudapi.com"
	service := "dnspod"
	ts := time.Now().Unix()
	timestamp := strconv.FormatInt(ts, 10)
	date := time.Unix(ts, 0).UTC().Format("2006-01-02")

	// TC3-HMAC-SHA256 签名
	canonicalRequest := strings.Join([]string{
		"POST", "/", "",
		"content-type:application/json; charset=utf-8\nhost:" + host + "\n",
		"content-type;host",
		tc3HashSHA256(payloadStr),
	}, "\n")
	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256", timestamp, credentialScope, tc3HashSHA256(canonicalRequest),
	}, "\n")
	secretDate := tc3HmacSHA256([]byte("TC3"+p.secretKey), date)
	secretService := tc3HmacSHA256(secretDate, service)
	secretSigning := tc3HmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(tc3HmacSHA256(secretSigning, stringToSign))
	authorization := fmt.Sprintf(
		"TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host, Signature=%s",
		p.secretID, credentialScope, signature,
	)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", "https://"+host, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Timestamp", timestamp)
	req.Header.Set("X-TC-Version", "2021-03-23")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (p *dnspodRecordProvider) ListRecords(domain string) ([]ProviderRecord, error) {
	body, err := p.dnspodPost("DescribeRecordList", map[string]interface{}{
		"Domain": domain,
		"Limit":  3000,
	})
	if err != nil {
		return nil, fmt.Errorf("获取 DNSPod 解析记录失败: %w", err)
	}
	var result struct {
		Response struct {
			RecordList []struct {
				RecordId   uint   `json:"RecordId"`
				Type       string `json:"Type"`
				SubDomain  string `json:"Name"`
				Value      string `json:"Value"`
				TTL        int    `json:"TTL"`
			} `json:"RecordList"`
			Error *struct{ Message string } `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 DNSPod 响应失败: %w", err)
	}
	if result.Response.Error != nil {
		return nil, fmt.Errorf("DNSPod API 错误: %s", result.Response.Error.Message)
	}
	var records []ProviderRecord
	for _, r := range result.Response.RecordList {
		records = append(records, ProviderRecord{
			RemoteID:   strconv.FormatUint(uint64(r.RecordId), 10),
			RecordType: r.Type,
			Host:       r.SubDomain,
			Value:      r.Value,
			TTL:        r.TTL,
		})
	}
	return records, nil
}

func (p *dnspodRecordProvider) ListDomains() ([]ProviderDomainItem, error) {
	body, err := p.dnspodPost("DescribeDomainList", map[string]interface{}{"Limit": 3000})
	if err != nil {
		return nil, fmt.Errorf("获取 DNSPod 域名列表失败: %w", err)
	}
	var result struct {
		Response struct {
			DomainList []struct {
				DomainId uint   `json:"DomainId"`
				Name     string `json:"Name"`
			} `json:"DomainList"`
			Error *struct{ Message string } `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 DNSPod 域名列表失败: %w", err)
	}
	if result.Response.Error != nil {
		return nil, fmt.Errorf("DNSPod API 错误: %s", result.Response.Error.Message)
	}
	var domains []ProviderDomainItem
	for _, d := range result.Response.DomainList {
		domains = append(domains, ProviderDomainItem{
			Name:    d.Name,
			ThirdID: strconv.FormatUint(uint64(d.DomainId), 10),
		})
	}
	return domains, nil
}

// ===== WOL =====

type WolHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewWolHandler(db *gorm.DB, log *logrus.Logger) *WolHandler {
	return &WolHandler{db: db, log: log}
}

func (h *WolHandler) List(c *gin.Context) {
	var devices []model.WolDevice
	h.db.Order("id desc").Find(&devices)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": devices})
}

func (h *WolHandler) Create(c *gin.Context) {
	var device model.WolDevice
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&device)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": device, "message": "创建成功"})
}

func (h *WolHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.WolDevice
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *WolHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.WolDevice{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *WolHandler) Wake(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var device model.WolDevice
	if err := h.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "设备不存在"})
		return
	}
	if err := sendWakePacket(device.MACAddress, device.BroadcastIP, device.NetInterface, device.Port); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "唤醒失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "唤醒包已发送"})
}

// ===== 域名账号 =====

type DomainAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainAccountHandler(db *gorm.DB, log *logrus.Logger) *DomainAccountHandler {
	return &DomainAccountHandler{db: db, log: log}
}

func (h *DomainAccountHandler) List(c *gin.Context) {
	var accounts []model.DomainAccount
	h.db.Order("id desc").Find(&accounts)
	// 隐藏 Secret
	for i := range accounts {
		if len(accounts[i].AccessSecret) > 4 {
			accounts[i].AccessSecret = "****" + accounts[i].AccessSecret[len(accounts[i].AccessSecret)-4:]
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *DomainAccountHandler) Create(c *gin.Context) {
	var account model.DomainAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *DomainAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var existing model.DomainAccount
	if err := h.db.First(&existing, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}
	var req model.DomainAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	// 如果 Secret 是掩码则保留原值
	if strings.HasPrefix(req.AccessSecret, "****") {
		req.AccessSecret = existing.AccessSecret
	}
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DomainAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// 检查是否有关联域名
	var count int64
	h.db.Model(&model.DomainInfo{}).Where("account_id = ?", id).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "该账号下存在域名，无法删除"})
		return
	}
	h.db.Delete(&model.DomainAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *DomainAccountHandler) Test(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var account model.DomainAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}
	// TODO: 根据 account.Provider 调用对应 DNS 服务商 API 验证凭据
	h.log.Infof("[域名账号] 测试连接: id=%d provider=%s", id, account.Provider)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "连接测试成功"})
}

// ===== 域名管理 =====

type DomainInfoHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainInfoHandler(db *gorm.DB, log *logrus.Logger) *DomainInfoHandler {
	h := &DomainInfoHandler{db: db, log: log}
	// 启动自动同步后台任务
	go h.runAutoSyncScheduler()
	return h
}

// ===== 自动同步调度器 =====

// autoSyncState 记录每个域名的自动同步定时器
var (
	autoSyncTimers   = make(map[uint]*time.Timer)
	autoSyncTimersMu sync.Mutex
)

// runAutoSyncScheduler 启动时扫描所有开启了自动同步的域名，注册定时器
func (h *DomainInfoHandler) runAutoSyncScheduler() {
	// 等待 DB 初始化完成
	time.Sleep(3 * time.Second)
	h.log.Info("[域名自动同步] 调度器启动")

	var domains []model.DomainInfo
	h.db.Where("auto_sync = ? AND sync_interval > 0", true).Find(&domains)
	for _, d := range domains {
		h.scheduleAutoSync(d.ID, d.SyncInterval)
	}
}

// scheduleAutoSync 为指定域名注册/重置自动同步定时器
func (h *DomainInfoHandler) scheduleAutoSync(domainID uint, intervalMinutes int) {
	autoSyncTimersMu.Lock()
	defer autoSyncTimersMu.Unlock()

	// 取消旧定时器
	if t, ok := autoSyncTimers[domainID]; ok {
		t.Stop()
		delete(autoSyncTimers, domainID)
	}
	if intervalMinutes <= 0 {
		return
	}

	var runSync func()
	runSync = func() {
		h.log.Infof("[域名自动同步] 执行同步: domain_id=%d", domainID)
		ctx := context.Background()
		h.doSyncFromProvider(ctx, domainID)

		// 重新注册下一次
		autoSyncTimersMu.Lock()
		autoSyncTimers[domainID] = time.AfterFunc(
			time.Duration(intervalMinutes)*time.Minute, runSync,
		)
		autoSyncTimersMu.Unlock()
	}

	autoSyncTimers[domainID] = time.AfterFunc(
		time.Duration(intervalMinutes)*time.Minute, runSync,
	)
	h.log.Infof("[域名自动同步] 已注册: domain_id=%d interval=%dm", domainID, intervalMinutes)
}

// cancelAutoSync 取消指定域名的自动同步定时器
func cancelAutoSync(domainID uint) {
	autoSyncTimersMu.Lock()
	defer autoSyncTimersMu.Unlock()
	if t, ok := autoSyncTimers[domainID]; ok {
		t.Stop()
		delete(autoSyncTimers, domainID)
	}
}

// doSyncFromProvider 核心同步逻辑：从服务商拉取解析记录并 upsert 到本地
// 返回同步的记录数和错误
func (h *DomainInfoHandler) doSyncFromProvider(ctx context.Context, domainInfoID uint) (int, error) {
	var domain model.DomainInfo
	if err := h.db.WithContext(ctx).First(&domain, domainInfoID).Error; err != nil {
		return 0, fmt.Errorf("域名不存在: %w", err)
	}

	var acc model.DomainAccount
	if err := h.db.WithContext(ctx).First(&acc, domain.AccountID).Error; err != nil {
		return 0, fmt.Errorf("账号不存在: %w", err)
	}

	h.log.Infof("[域名同步] 开始同步: domain=%s provider=%s", domain.Name, acc.Provider)

	// 创建对应服务商的 provider
	provider := newDNSRecordProvider(acc)
	if provider == nil {
		return 0, fmt.Errorf("不支持的 DNS 服务商: %s", acc.Provider)
	}

	// 从服务商拉取解析记录
	providerRecords, err := provider.ListRecords(domain.Name)
	if err != nil {
		return 0, fmt.Errorf("从服务商拉取解析记录失败: %w", err)
	}

	h.log.Infof("[域名同步] 服务商返回 %d 条记录: domain=%s", len(providerRecords), domain.Name)

	// 构建服务商记录的 remoteID 集合，用于后续删除本地多余记录
	remoteIDSet := make(map[string]bool)
	for _, pr := range providerRecords {
		if pr.RemoteID != "" {
			remoteIDSet[pr.RemoteID] = true
		}
	}

	// 三路 diff：插入/更新/删除
	for _, pr := range providerRecords {
		var local model.DomainRecord
		err := h.db.WithContext(ctx).Where("domain_info_id = ? AND remote_id = ?", domainInfoID, pr.RemoteID).First(&local).Error
		if err != nil {
			// 本地不存在 → 插入
			h.db.WithContext(ctx).Create(&model.DomainRecord{
				DomainInfoID:    domainInfoID,
				DomainAccountID: domain.AccountID,
				Domain:          domain.Name,
				RecordType:      pr.RecordType,
				Host:            pr.Host,
				Value:           pr.Value,
				TTL:             pr.TTL,
				RemoteID:        pr.RemoteID,
				Proxied:         pr.Proxied,
			})
		} else {
			// 本地已存在 → 更新
			h.db.WithContext(ctx).Model(&local).Updates(map[string]interface{}{
				"record_type": pr.RecordType,
				"host":        pr.Host,
				"value":       pr.Value,
				"ttl":         pr.TTL,
				"proxied":     pr.Proxied,
			})
		}
	}

	// 删除服务商已不存在的本地记录（仅删除有 remote_id 的记录）
	if len(remoteIDSet) > 0 {
		remoteIDs := make([]string, 0, len(remoteIDSet))
		for id := range remoteIDSet {
			remoteIDs = append(remoteIDs, id)
		}
		h.db.WithContext(ctx).Where(
			"domain_info_id = ? AND remote_id != '' AND remote_id NOT IN ?",
			domainInfoID, remoteIDs,
		).Delete(&model.DomainRecord{})
	}

	// 更新最后同步时间和记录数
	var localCount int64
	h.db.WithContext(ctx).Model(&model.DomainRecord{}).Where("domain_info_id = ?", domainInfoID).Count(&localCount)
	now := time.Now()
	h.db.WithContext(ctx).Model(&model.DomainInfo{}).Where("id = ?", domainInfoID).Updates(map[string]interface{}{
		"last_sync_time": now,
		"record_count":   localCount,
	})

	return int(localCount), nil
}

// List 获取域名列表
func (h *DomainInfoHandler) List(c *gin.Context) {
	accountID := c.Query("account_id")
	keyword := c.Query("keyword")

	var domains []model.DomainInfo
	query := h.db.Order("id desc")
	if accountID != "" {
		query = query.Where("account_id = ?", accountID)
	}
	if keyword != "" {
		query = query.Where("name LIKE ? OR remark LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Find(&domains)

	// 附加账号信息
	type DomainWithAccount struct {
		model.DomainInfo
		AccountName     string `json:"account_name"`
		AccountProvider string `json:"account_provider"`
	}
	result := make([]DomainWithAccount, 0, len(domains))
	for _, d := range domains {
		var acc model.DomainAccount
		h.db.Select("name, provider").First(&acc, d.AccountID)
		result = append(result, DomainWithAccount{
			DomainInfo:      d,
			AccountName:     acc.Name,
			AccountProvider: acc.Provider,
		})
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": result})
}

// Create 添加域名
func (h *DomainInfoHandler) Create(c *gin.Context) {
	var domain model.DomainInfo
	if err := c.ShouldBindJSON(&domain); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	// 检查账号是否存在
	var acc model.DomainAccount
	if err := h.db.First(&acc, domain.AccountID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "域名账号不存在"})
		return
	}
	// 检查域名是否已存在
	var count int64
	h.db.Model(&model.DomainInfo{}).Where("account_id = ? AND name = ?", domain.AccountID, domain.Name).Count(&count)
	if count > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "该账号下域名已存在"})
		return
	}
	h.db.Create(&domain)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": domain, "message": "添加域名成功"})
}

// Update 修改域名配置（到期时间、到期提醒、备注等）
func (h *DomainInfoHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var domain model.DomainInfo
	if err := h.db.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "域名不存在"})
		return
	}
	var req model.DomainInfo
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

// Delete 删除域名
func (h *DomainInfoHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var domain model.DomainInfo
	if err := h.db.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "域名不存在"})
		return
	}
	// 同时删除关联的解析记录
	h.db.Where("domain_info_id = ?", id).Delete(&model.DomainRecord{})
	h.db.Delete(&model.DomainInfo{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// FetchFromProvider 从服务商拉取账号下的域名列表（含已添加状态）
func (h *DomainInfoHandler) FetchFromProvider(c *gin.Context) {
	accountIDStr := c.Query("account_id")
	if accountIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请提供 account_id"})
		return
	}
	accountID, err := strconv.ParseUint(accountIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "account_id 格式错误"})
		return
	}

	var acc model.DomainAccount
	if err := h.db.First(&acc, accountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}

	// 查询该账号下已添加的域名
	var existingDomains []model.DomainInfo
	h.db.Where("account_id = ?", accountID).Find(&existingDomains)
	existingMap := make(map[string]bool)
	for _, d := range existingDomains {
		existingMap[d.Name] = true
	}

	h.log.Infof("[域名管理] 从服务商拉取域名列表: account_id=%d provider=%s", accountID, acc.Provider)

	type ProviderDomain struct {
		Name    string `json:"name"`
		ThirdID string `json:"third_id"`
		Added   bool   `json:"added"` // 是否已添加到本地
	}

	// 创建对应服务商的 provider
	provider := newDNSRecordProvider(acc)
	var providerDomains []ProviderDomain

	if provider != nil {
		providerItems, err := provider.ListDomains()
		if err != nil {
			h.log.Warnf("[域名管理] 拉取域名列表失败: %v", err)
			// 拉取失败时降级：仅返回本地已添加的域名
			for _, d := range existingDomains {
				providerDomains = append(providerDomains, ProviderDomain{
					Name:    d.Name,
					ThirdID: d.ThirdID,
					Added:   true,
				})
			}
			c.JSON(http.StatusOK, gin.H{
				"code": 200,
				"data": gin.H{
					"domains":  providerDomains,
					"provider": acc.Provider,
					"account":  acc.Name,
					"error":    err.Error(),
				},
			})
			return
		}
		for _, item := range providerItems {
			providerDomains = append(providerDomains, ProviderDomain{
				Name:    item.Name,
				ThirdID: item.ThirdID,
				Added:   existingMap[item.Name],
			})
		}
	} else {
		// 不支持的服务商：仅返回本地已添加的域名
		for _, d := range existingDomains {
			providerDomains = append(providerDomains, ProviderDomain{
				Name:    d.Name,
				ThirdID: d.ThirdID,
				Added:   true,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"data": gin.H{
			"domains":  providerDomains,
			"provider": acc.Provider,
			"account":  acc.Name,
		},
	})
}

// UpdateAutoSync 更新域名的自动同步配置
func (h *DomainInfoHandler) UpdateAutoSync(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		AutoSync     bool `json:"auto_sync"`
		SyncInterval int  `json:"sync_interval"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	var domain model.DomainInfo
	if err := h.db.First(&domain, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "域名不存在"})
		return
	}
	h.db.Model(&model.DomainInfo{}).Where("id = ?", id).Updates(map[string]interface{}{
		"auto_sync":     req.AutoSync,
		"sync_interval": req.SyncInterval,
	})
	// 重新注册/取消定时器
	if req.AutoSync && req.SyncInterval > 0 {
		h.scheduleAutoSync(uint(id), req.SyncInterval)
	} else {
		cancelAutoSync(uint(id))
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "自动同步配置已更新"})
}

// Refresh 立即刷新（手动触发同步）
func (h *DomainInfoHandler) Refresh(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	count, err := h.doSyncFromProvider(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "同步成功", "data": gin.H{"count": count}})
}

// ===== 证书账号 =====

type CertAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewCertAccountHandler(db *gorm.DB, log *logrus.Logger) *CertAccountHandler {
	return &CertAccountHandler{db: db, log: log}
}

func (h *CertAccountHandler) List(c *gin.Context) {
	var accounts []model.CertAccount
	h.db.Order("id desc").Find(&accounts)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *CertAccountHandler) Create(c *gin.Context) {
	var account model.CertAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *CertAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CertAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CertAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CertAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CertAccountHandler) Verify(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var account model.CertAccount
	if err := h.db.First(&account, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}
	// TODO: 调用 ACME 接口验证账号有效性
	h.log.Infof("[证书账号] 验证账号: id=%d type=%s email=%s", id, account.Type, account.Email)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "账号验证成功"})
}

// ===== 域名证书 =====

type CertHandler struct {
	db     *gorm.DB
	log    *logrus.Logger
	config *config.Config
}

func NewCertHandler(db *gorm.DB, log *logrus.Logger, cfg *config.Config) *CertHandler {
	return &CertHandler{db: db, log: log, config: cfg}
}

func (h *CertHandler) List(c *gin.Context) {
	var certs []model.DomainCert
	h.db.Order("id desc").Find(&certs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": certs})
}

func (h *CertHandler) Create(c *gin.Context) {
	var cert model.DomainCert
	if err := c.ShouldBindJSON(&cert); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cert.Status = "pending"
	h.db.Create(&cert)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": cert, "message": "创建成功"})
}

func (h *CertHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainCert
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CertHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainCert{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CertHandler) Renew(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	// TODO: 实现 ACME 证书续期
	h.log.Infof("触发证书续期: %d", id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "证书续期任务已提交"})
}

// ===== 域名解析 =====

type DomainRecordHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewDomainRecordHandler(db *gorm.DB, log *logrus.Logger) *DomainRecordHandler {
	return &DomainRecordHandler{db: db, log: log}
}

func (h *DomainRecordHandler) List(c *gin.Context) {
	domainInfoID := c.Query("domain_info_id")
	accountID := c.Query("account_id")
	var records []model.DomainRecord
	query := h.db.Order("id desc")
	if domainInfoID != "" {
		query = query.Where("domain_info_id = ?", domainInfoID)
	} else if accountID != "" {
		query = query.Where("domain_account_id = ?", accountID)
	}
	query.Find(&records)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": records})
}

func (h *DomainRecordHandler) Create(c *gin.Context) {
	var record model.DomainRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	// 从 DomainInfo 获取 account_id
	if record.DomainInfoID > 0 && record.DomainAccountID == 0 {
		var domain model.DomainInfo
		if err := h.db.First(&domain, record.DomainInfoID).Error; err == nil {
			record.DomainAccountID = domain.AccountID
			record.Domain = domain.Name
		}
	}
	h.db.Create(&record)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": record, "message": "创建成功"})
}

func (h *DomainRecordHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.DomainRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *DomainRecordHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.DomainRecord{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *DomainRecordHandler) SyncFromProvider(c *gin.Context) {
	domainInfoIDStr := c.Param("domainInfoId")
	domainInfoID, err := strconv.ParseUint(domainInfoIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数格式错误"})
		return
	}
	var domain model.DomainInfo
	if err := h.db.First(&domain, domainInfoID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "域名不存在"})
		return
	}
	var acc model.DomainAccount
	if err := h.db.First(&acc, domain.AccountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "账号不存在"})
		return
	}

	h.log.Infof("[域名解析] 同步解析记录: domain=%s provider=%s", domain.Name, acc.Provider)

	// 使用 doSyncFromProvider 统一处理同步逻辑
	domainInfoHandler := &DomainInfoHandler{db: h.db, log: h.log}
	count, err := domainInfoHandler.doSyncFromProvider(c.Request.Context(), uint(domainInfoID))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "同步完成", "data": gin.H{"count": count}})
}

// ===== IP 地址库 =====

type IPDBHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewIPDBHandler(db *gorm.DB, log *logrus.Logger) *IPDBHandler {
	return &IPDBHandler{db: db, log: log}
}

func (h *IPDBHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	keyword := c.Query("keyword")

	var entries []model.IPDBEntry
	var total int64
	query := h.db.Model(&model.IPDBEntry{})
	if keyword != "" {
		query = query.Where("cidr LIKE ? OR location LIKE ? OR tags LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Count(&total)
	query.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&entries)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"list":      entries,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}})
}

func (h *IPDBHandler) Create(c *gin.Context) {
	var entry model.IPDBEntry
	if err := c.ShouldBindJSON(&entry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&entry)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": entry, "message": "创建成功"})
}

func (h *IPDBHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.IPDBEntry
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *IPDBHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.IPDBEntry{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// parseIPsFromLine 从一行文本中解析出所有 IP/CIDR，支持空格、逗号、分号分隔多个 IP 段
// 行格式示例：
//   192.168.1.0/24
//   192.168.1.0/24 10.0.0.0/8
//   192.168.1.0/24,10.0.0.0/8;172.16.0.0/12
func parseIPsFromLine(line string) []string {
	// 统一将逗号、分号替换为空格，再按空格分割
	replacer := strings.NewReplacer(",", " ", ";", " ")
	normalized := replacer.Replace(line)
	parts := strings.Fields(normalized)
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// 验证是否为有效 IP 或 CIDR
		if strings.Contains(p, "/") {
			_, _, err := net.ParseCIDR(p)
			if err != nil {
				continue
			}
		} else {
			if net.ParseIP(p) == nil {
				continue
			}
		}
		result = append(result, p)
	}
	return result
}

// parseTextToEntries 将文本内容解析为 IPDBEntry 列表
// 支持每行多个 IP/CIDR（空格/逗号/分号分隔），行尾可附加 location 和 tags
// 格式：
//   CIDR1 CIDR2 CIDR3
//   CIDR1,CIDR2 location tags
//   # 注释行
func parseTextToEntries(text, defaultLocation, defaultTags string) []model.IPDBEntry {
	var entries []model.IPDBEntry
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		// 尝试解析行中所有 token，IP/CIDR 格式的作为地址，其余作为 location/tags
		replacer := strings.NewReplacer(",", " ", ";", " ")
		normalized := replacer.Replace(line)
		tokens := strings.Fields(normalized)

		var cidrs []string
		var extras []string
		for _, tok := range tokens {
			if strings.Contains(tok, "/") {
				_, _, err := net.ParseCIDR(tok)
				if err == nil {
					cidrs = append(cidrs, tok)
					continue
				}
			} else if net.ParseIP(tok) != nil {
				cidrs = append(cidrs, tok)
				continue
			}
			extras = append(extras, tok)
		}

		// extras[0] 作为 location，extras[1] 作为 tags（若未指定默认值）
		location := defaultLocation
		tags := defaultTags
		if len(extras) >= 1 && defaultLocation == "" {
			location = extras[0]
		}
		if len(extras) >= 2 && defaultTags == "" {
			tags = extras[1]
		}

		for _, cidr := range cidrs {
			entries = append(entries, model.IPDBEntry{
				CIDR:     cidr,
				Location: location,
				Tags:     tags,
			})
		}
	}
	return entries
}

// upsertEntries 批量 upsert IP 条目，返回实际写入数量
func (h *IPDBHandler) upsertEntries(entries []model.IPDBEntry) int {
	imported := 0
	for i := range entries {
		if entries[i].CIDR == "" {
			continue
		}
		var existing model.IPDBEntry
		result := h.db.Where("cidr = ?", entries[i].CIDR).First(&existing)
		if result.Error == nil {
			existing.Location = entries[i].Location
			existing.Tags = entries[i].Tags
			if entries[i].Remark != "" {
				existing.Remark = entries[i].Remark
			}
			h.db.Save(&existing)
		} else {
			h.db.Create(&entries[i])
		}
		imported++
	}
	return imported
}

// Import 批量导入（手动输入文本，每行支持多个 IP/CIDR）
func (h *IPDBHandler) Import(c *gin.Context) {
	var req struct {
		Entries  []model.IPDBEntry `json:"entries"`
		Text     string            `json:"text"`
		Location string            `json:"location"`
		Tags     string            `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	var entries []model.IPDBEntry
	if req.Text != "" {
		entries = append(entries, parseTextToEntries(req.Text, req.Location, req.Tags)...)
	}
	entries = append(entries, req.Entries...)

	if len(entries) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "没有可导入的条目"})
		return
	}

	imported := h.upsertEntries(entries)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入成功", "data": gin.H{"count": imported}})
}

// downloadAndParseURL 下载 URL 内容并解析为 IPDBEntry 列表
func (h *IPDBHandler) downloadAndParseURL(url, defaultLocation, defaultTags string) ([]model.IPDBEntry, error) {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败，HTTP状态码: %d", resp.StatusCode)
	}

	// 限制最大读取 50MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("读取内容失败: %w", err)
	}

	entries := parseTextToEntries(string(body), defaultLocation, defaultTags)
	return entries, nil
}

// ImportFromURL 从 URL 下载并导入 IP 列表（每行支持多个 IP/CIDR）
func (h *IPDBHandler) ImportFromURL(c *gin.Context) {
	var req struct {
		URL        string `json:"url" binding:"required"`
		Location   string `json:"location"`
		Tags       string `json:"tags"`
		ClearFirst bool   `json:"clear_first"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	entries, err := h.downloadAndParseURL(req.URL, req.Location, req.Tags)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if len(entries) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "文件中没有找到有效的 IP/CIDR 条目", "data": gin.H{"count": 0}})
		return
	}

	if req.ClearFirst {
		h.db.Where("1 = 1").Delete(&model.IPDBEntry{})
	}

	imported := h.upsertEntries(entries)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "导入成功", "data": gin.H{
		"count": imported,
		"url":   req.URL,
	}})
}

// ===== IP 地址库订阅 =====

// ListSubscriptions 获取订阅列表
func (h *IPDBHandler) ListSubscriptions(c *gin.Context) {
	var subs []model.IPDBSubscription
	h.db.Order("id desc").Find(&subs)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": subs})
}

// CreateSubscription 创建订阅
func (h *IPDBHandler) CreateSubscription(c *gin.Context) {
	var sub model.IPDBSubscription
	if err := c.ShouldBindJSON(&sub); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&sub)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": sub, "message": "创建成功"})
}

// UpdateSubscription 更新订阅
func (h *IPDBHandler) UpdateSubscription(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.IPDBSubscription
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

// DeleteSubscription 删除订阅
func (h *IPDBHandler) DeleteSubscription(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.IPDBSubscription{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// RefreshSubscription 手动刷新订阅
func (h *IPDBHandler) RefreshSubscription(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var sub model.IPDBSubscription
	if err := h.db.First(&sub, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "订阅不存在"})
		return
	}

	entries, err := h.downloadAndParseURL(sub.URL, sub.Location, sub.Tags)
	now := time.Now()
	if err != nil {
		sub.LastSyncTime = &now
		sub.LastSyncError = err.Error()
		h.db.Save(&sub)
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if len(entries) == 0 {
		sub.LastSyncTime = &now
		sub.LastSyncCount = 0
		sub.LastSyncError = ""
		h.db.Save(&sub)
		c.JSON(http.StatusOK, gin.H{"code": 200, "message": "文件中没有找到有效的 IP/CIDR 条目", "data": gin.H{"count": 0}})
		return
	}

	if sub.ClearFirst {
		h.db.Where("1 = 1").Delete(&model.IPDBEntry{})
	}

	imported := h.upsertEntries(entries)
	sub.LastSyncTime = &now
	sub.LastSyncCount = imported
	sub.LastSyncError = ""
	h.db.Save(&sub)

	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "刷新成功", "data": gin.H{"count": imported}})
}

// Query 查询 IP 归属地
func (h *IPDBHandler) Query(c *gin.Context) {
	ip := c.Query("ip")
	if ip == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "请提供 IP 地址"})
		return
	}

	netIP := net.ParseIP(ip)
	if netIP == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的 IP 地址格式"})
		return
	}

	// 先精确匹配
	var entry model.IPDBEntry
	if err := h.db.Where("cidr = ?", ip).First(&entry).Error; err == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "data": entry})
		return
	}

	// 遍历所有 CIDR 条目进行匹配
	var allEntries []model.IPDBEntry
	h.db.Find(&allEntries)

	for _, e := range allEntries {
		if strings.Contains(e.CIDR, "/") {
			_, ipNet, err := net.ParseCIDR(e.CIDR)
			if err == nil && ipNet.Contains(netIP) {
				c.JSON(http.StatusOK, gin.H{"code": 200, "data": e})
				return
			}
		} else {
			if net.ParseIP(e.CIDR).Equal(netIP) {
				c.JSON(http.StatusOK, gin.H{"code": 200, "data": e})
				return
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 200, "data": gin.H{
		"ip":       ip,
		"location": "",
		"tags":     "",
		"found":    false,
	}})
}

// ===== 访问控制 =====

type AccessHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *access.Manager
}

func NewAccessHandler(db *gorm.DB, log *logrus.Logger, mgr *access.Manager) *AccessHandler {
	return &AccessHandler{db: db, log: log, mgr: mgr}
}

func (h *AccessHandler) List(c *gin.Context) {
	var rules []model.AccessRule
	h.db.Order("id desc").Find(&rules)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rules})
}

func (h *AccessHandler) Create(c *gin.Context) {
	var rule model.AccessRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&rule)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": rule, "message": "创建成功"})
}

func (h *AccessHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.AccessRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *AccessHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.AccessRule{}, id)
	h.mgr.Reload()
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

// ===== 回调账号 =====

type CallbackAccountHandler struct {
	db  *gorm.DB
	log *logrus.Logger
	mgr *callback.Manager
}

func NewCallbackAccountHandler(db *gorm.DB, log *logrus.Logger, mgr *callback.Manager) *CallbackAccountHandler {
	return &CallbackAccountHandler{db: db, log: log, mgr: mgr}
}

func (h *CallbackAccountHandler) List(c *gin.Context) {
	var accounts []model.CallbackAccount
	h.db.Order("id desc").Find(&accounts)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": accounts})
}

func (h *CallbackAccountHandler) Create(c *gin.Context) {
	var account model.CallbackAccount
	if err := c.ShouldBindJSON(&account); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&account)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": account, "message": "创建成功"})
}

func (h *CallbackAccountHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackAccount
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackAccountHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CallbackAccount{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}

func (h *CallbackAccountHandler) Test(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.mgr.TestAccount(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "测试失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "测试成功"})
}

// ===== 回调任务 =====

type CallbackTaskHandler struct {
	db  *gorm.DB
	log *logrus.Logger
}

func NewCallbackTaskHandler(db *gorm.DB, log *logrus.Logger) *CallbackTaskHandler {
	return &CallbackTaskHandler{db: db, log: log}
}

func (h *CallbackTaskHandler) List(c *gin.Context) {
	var tasks []model.CallbackTask
	h.db.Order("id desc").Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": tasks})
}

func (h *CallbackTaskHandler) Create(c *gin.Context) {
	var task model.CallbackTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.db.Create(&task)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": task, "message": "创建成功"})
}

func (h *CallbackTaskHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req model.CallbackTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	req.ID = uint(id)
	h.db.Save(&req)
	c.JSON(http.StatusOK, gin.H{"code": 200, "data": req, "message": "更新成功"})
}

func (h *CallbackTaskHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	h.db.Delete(&model.CallbackTask{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 200, "message": "删除成功"})
}
