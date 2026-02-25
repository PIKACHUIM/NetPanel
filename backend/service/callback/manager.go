package callback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// TriggerEvent 触发事件
type TriggerEvent struct {
	Type     string // "stun_ip_change" / "frp_reconnect" 等
	SourceID uint
	NewIP    string
	NewPort  int
	OldIP    string
	OldPort  int
}

// triggerTypePrefix 事件类型前缀映射到任务 trigger_type
// 例如 "stun_ip_change" 匹配 trigger_type = "stun"
var triggerTypePrefix = map[string]string{
	"stun_ip_change": "stun",
	"frp_reconnect":  "frp",
	"et_reconnect":   "easytier",
}

// Manager 回调任务管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	eventCh chan TriggerEvent
	stopCh  chan struct{}
	once    sync.Once
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{
		db:      db,
		log:     log,
		eventCh: make(chan TriggerEvent, 64),
		stopCh:  make(chan struct{}),
	}
}

func (m *Manager) Start() {
	m.once.Do(func() {
		go m.processEvents()
	})
}

func (m *Manager) Stop() {
	close(m.stopCh)
}

// Trigger 触发回调事件
func (m *Manager) Trigger(event TriggerEvent) {
	select {
	case m.eventCh <- event:
	default:
		m.log.Warn("[回调] 事件队列已满，丢弃事件")
	}
}

func (m *Manager) processEvents() {
	for {
		select {
		case <-m.stopCh:
			return
		case event := <-m.eventCh:
			m.handleEvent(event)
		}
	}
}

func (m *Manager) handleEvent(event TriggerEvent) {
	// 将事件类型映射到任务 trigger_type
	// 例如 "stun_ip_change" → "stun"
	triggerType := event.Type
	if mapped, ok := triggerTypePrefix[event.Type]; ok {
		triggerType = mapped
	}

	var tasks []model.CallbackTask
	m.db.Where("enable = ? AND trigger_type = ?", true, triggerType).Find(&tasks)

	for _, task := range tasks {
		if task.TriggerSourceID != 0 && task.TriggerSourceID != event.SourceID {
			continue
		}
		go m.executeTask(&task, &event)
	}
}

func (m *Manager) executeTask(task *model.CallbackTask, event *TriggerEvent) {
	m.log.Infof("[回调] 执行任务 [%s]，触发事件: %s", task.Name, event.Type)

	var account model.CallbackAccount
	if err := m.db.First(&account, task.AccountID).Error; err != nil {
		m.log.Errorf("[回调] 账号不存在: %v", err)
		return
	}

	var err error
	switch account.Type {
	case "webhook":
		err = m.executeWebhook(&account, task, event)
	case "cf_origin":
		err = m.executeCFOrigin(&account, task, event)
	case "ali_esa":
		err = m.executeAliESA(&account, task, event)
	case "tencent_eo":
		err = m.executeTencentEO(&account, task, event)
	default:
		err = fmt.Errorf("不支持的账号类型: %s", account.Type)
	}

	if err != nil {
		m.log.Errorf("[回调] 任务 [%s] 执行失败: %v", task.Name, err)
		m.db.Model(&model.CallbackTask{}).Where("id = ?", task.ID).Update("last_error", err.Error())
	} else {
		m.log.Infof("[回调] 任务 [%s] 执行成功", task.Name)
		m.db.Model(&model.CallbackTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
			"last_run_time": time.Now(),
			"last_error":    "",
		})
	}
}

// executeWebhook 执行 Webhook 回调
func (m *Manager) executeWebhook(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(account.Config), &cfg); err != nil {
		return fmt.Errorf("解析账号配置失败: %w", err)
	}

	url := cfg["url"]
	if url == "" {
		return fmt.Errorf("Webhook URL 未配置")
	}
	method := cfg["method"]
	if method == "" {
		method = "POST"
	}

	payload := map[string]interface{}{
		"event":    event.Type,
		"new_ip":   event.NewIP,
		"new_port": event.NewPort,
		"old_ip":   event.OldIP,
		"old_port": event.OldPort,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := cfg["token"]; token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Webhook 返回错误状态码: %d", resp.StatusCode)
	}
	return nil
}

// executeCFOrigin 更新 Cloudflare 回源端口
// 配置字段：api_token, zone_id, rule_id, origin_port（可选，默认使用 event.NewPort）
func (m *Manager) executeCFOrigin(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(account.Config), &cfg); err != nil {
		return fmt.Errorf("解析账号配置失败: %w", err)
	}

	apiToken := cfg["api_token"]
	zoneID := cfg["zone_id"]
	ruleID := cfg["rule_id"]
	if apiToken == "" || zoneID == "" || ruleID == "" {
		return fmt.Errorf("CF 回源配置不完整（需要 api_token、zone_id、rule_id）")
	}

	// 目标端口：优先使用配置中的固定端口，否则使用事件中的新端口
	targetPort := event.NewPort
	if p := cfg["origin_port"]; p != "" {
		fmt.Sscanf(p, "%d", &targetPort)
	}

	// 先获取当前规则内容
	getURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/rulesets/phases/http_request_origin/entrypoint/rules/%s", zoneID, ruleID)
	req, err := http.NewRequest("GET", getURL, nil)
	if err != nil {
		return fmt.Errorf("创建 CF 请求失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("CF API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var ruleResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&ruleResp); err != nil {
		return fmt.Errorf("解析 CF 响应失败: %w", err)
	}

	// 构建更新请求：修改 origin 规则中的端口
	// CF Rules API: PATCH /zones/{zone_id}/rulesets/phases/http_request_origin/entrypoint/rules/{rule_id}
	patchURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/rulesets/phases/http_request_origin/entrypoint/rules/%s", zoneID, ruleID)
	patchBody := map[string]interface{}{
		"action": "route",
		"action_parameters": map[string]interface{}{
			"origin": map[string]interface{}{
				"port": targetPort,
			},
		},
	}

	bodyBytes, _ := json.Marshal(patchBody)
	patchReq, err := http.NewRequest("PATCH", patchURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建 CF PATCH 请求失败: %w", err)
	}
	patchReq.Header.Set("Authorization", "Bearer "+apiToken)
	patchReq.Header.Set("Content-Type", "application/json")

	patchResp, err := client.Do(patchReq)
	if err != nil {
		return fmt.Errorf("CF PATCH 请求失败: %w", err)
	}
	defer patchResp.Body.Close()

	respBody, _ := io.ReadAll(patchResp.Body)
	if patchResp.StatusCode >= 400 {
		return fmt.Errorf("CF API 返回错误 %d: %s", patchResp.StatusCode, string(respBody))
	}

	m.log.Infof("[回调][CF] 回源端口已更新为 %d，规则 %s", targetPort, ruleID)
	return nil
}

// executeAliESA 更新阿里云 ESA（边缘安全加速）回源端口
// 配置字段：access_key_id, access_key_secret, site_id, rule_id
func (m *Manager) executeAliESA(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(account.Config), &cfg); err != nil {
		return fmt.Errorf("解析账号配置失败: %w", err)
	}

	accessKeyID := cfg["access_key_id"]
	accessKeySecret := cfg["access_key_secret"]
	siteID := cfg["site_id"]
	ruleID := cfg["rule_id"]
	if accessKeyID == "" || accessKeySecret == "" || siteID == "" {
		return fmt.Errorf("阿里云 ESA 配置不完整（需要 access_key_id、access_key_secret、site_id）")
	}

	targetPort := event.NewPort
	if p := cfg["origin_port"]; p != "" {
		fmt.Sscanf(p, "%d", &targetPort)
	}

	// 调用阿里云 ESA OpenAPI 更新回源规则
	// API: https://esa.aliyuncs.com/ UpdateOriginPool 或 UpdateRoutineRelatedRecord
	// 使用阿里云 OpenAPI 签名 V4
	apiURL := "https://esa.aliyuncs.com/"
	params := map[string]string{
		"Action":          "UpdateOriginPool",
		"Version":         "2024-09-10",
		"SiteId":          siteID,
		"Format":          "JSON",
		"AccessKeyId":     accessKeyID,
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"Timestamp":       time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()),
	}
	if ruleID != "" {
		params["Id"] = ruleID
	}

	// 构建请求体
	reqBody := map[string]interface{}{
		"SiteId": siteID,
		"Origin": fmt.Sprintf("%s:%d", event.NewIP, targetPort),
	}
	if ruleID != "" {
		reqBody["Id"] = ruleID
	}

	bodyBytes, _ := json.Marshal(reqBody)

	// 使用简单的 POST 请求（实际生产中需要完整的阿里云签名）
	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建阿里云 ESA 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range params {
		req.Header.Set("x-acs-"+strings.ToLower(k), v)
	}
	req.Header.Set("x-acs-accesskeyid", accessKeyID)
	req.Header.Set("x-acs-action", "UpdateOriginPool")
	req.Header.Set("x-acs-version", "2024-09-10")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("阿里云 ESA API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("阿里云 ESA API 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	m.log.Infof("[回调][阿里ESA] 回源已更新为 %s:%d，站点 %s", event.NewIP, targetPort, siteID)
	return nil
}

// executeTencentEO 更新腾讯云 EO（EdgeOne）回源端口
// 配置字段：secret_id, secret_key, zone_id, rule_id
func (m *Manager) executeTencentEO(account *model.CallbackAccount, task *model.CallbackTask, event *TriggerEvent) error {
	var cfg map[string]string
	if err := json.Unmarshal([]byte(account.Config), &cfg); err != nil {
		return fmt.Errorf("解析账号配置失败: %w", err)
	}

	secretID := cfg["secret_id"]
	secretKey := cfg["secret_key"]
	zoneID := cfg["zone_id"]
	ruleID := cfg["rule_id"]
	if secretID == "" || secretKey == "" || zoneID == "" {
		return fmt.Errorf("腾讯云 EO 配置不完整（需要 secret_id、secret_key、zone_id）")
	}

	targetPort := event.NewPort
	if p := cfg["origin_port"]; p != "" {
		fmt.Sscanf(p, "%d", &targetPort)
	}

	// 腾讯云 EO API：ModifyOriginGroup 更新回源组
	// API 文档：https://cloud.tencent.com/document/product/1552/80698
	apiURL := "https://teo.tencentcloudapi.com/"
	timestamp := time.Now().Unix()

	reqBody := map[string]interface{}{
		"ZoneId": zoneID,
		"OriginGroupId": ruleID,
		"Origins": []map[string]interface{}{
			{
				"OriginId":     "origin-1",
				"Origin":       event.NewIP,
				"OriginPort":   fmt.Sprintf("%d", targetPort),
				"Weight":       100,
				"Private":      false,
			},
		},
	}

	bodyBytes, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("创建腾讯云 EO 请求失败: %w", err)
	}

	// 腾讯云 API 3.0 签名
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TC-Action", "ModifyOriginGroup")
	req.Header.Set("X-TC-Version", "2022-09-01")
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", "")

	// 简化签名（实际生产中需要完整的 TC3-HMAC-SHA256 签名）
	authHeader := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/teo/tc3_request, SignedHeaders=content-type;host, Signature=placeholder",
		secretID, time.Now().UTC().Format("2006-01-02"))
	req.Header.Set("Authorization", authHeader)
	_ = secretKey // 实际签名时使用

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("腾讯云 EO API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("腾讯云 EO API 返回错误 %d: %s", resp.StatusCode, string(respBody))
	}

	// 检查业务错误
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err == nil {
		if response, ok := result["Response"].(map[string]interface{}); ok {
			if errInfo, ok := response["Error"].(map[string]interface{}); ok {
				return fmt.Errorf("腾讯云 EO 业务错误: %v - %v", errInfo["Code"], errInfo["Message"])
			}
		}
	}

	m.log.Infof("[回调][腾讯EO] 回源已更新为 %s:%d，Zone %s", event.NewIP, targetPort, zoneID)
	return nil
}

// TestAccount 测试回调账号连通性
func (m *Manager) TestAccount(id uint) error {
	var account model.CallbackAccount
	if err := m.db.First(&account, id).Error; err != nil {
		return fmt.Errorf("账号不存在: %w", err)
	}

	testEvent := &TriggerEvent{
		Type:    "test",
		NewIP:   "1.2.3.4",
		NewPort: 12345,
		OldIP:   "0.0.0.0",
		OldPort: 0,
	}

	switch account.Type {
	case "webhook":
		return m.executeWebhook(&account, nil, testEvent)
	case "cf_origin":
		return m.executeCFOrigin(&account, nil, testEvent)
	case "ali_esa":
		return m.executeAliESA(&account, nil, testEvent)
	case "tencent_eo":
		return m.executeTencentEO(&account, nil, testEvent)
	default:
		return fmt.Errorf("账号类型 %s 暂不支持测试", account.Type)
	}
}
