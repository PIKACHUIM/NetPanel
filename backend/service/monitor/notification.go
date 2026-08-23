package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// NotificationManager 通知管理器
type NotificationManager struct {
	db *gorm.DB
}

// NewNotificationManager 创建通知管理器
func NewNotificationManager(db *gorm.DB) *NotificationManager {
	return &NotificationManager{
		db: db,
	}
}

// Send 发送通知
func (n *NotificationManager) Send(accountID uint, title, message string) error {
	var account model.CallbackAccount
	if err := n.db.First(&account, accountID).Error; err != nil {
		return fmt.Errorf("通知渠道不存在: %w", err)
	}
	
	// 根据类型发送通知
	switch account.Type {
	case "webhook":
		return n.sendWebhook(account, title, message)
	case "email":
		return n.sendEmail(account, title, message)
	case "wechat_work":
		return n.sendWechatWork(account, title, message)
	case "dingtalk":
		return n.sendDingtalk(account, title, message)
	case "telegram":
		return n.sendTelegram(account, title, message)
	case "discord":
		return n.sendDiscord(account, title, message)
	default:
		return fmt.Errorf("不支持的通知类型: %s", account.Type)
	}
}

// sendWebhook 发送 Webhook 通知
func (n *NotificationManager) sendWebhook(account model.CallbackAccount, title, message string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(account.Config), &config); err != nil {
		return err
	}
	
	url, ok := config["url"].(string)
	if !ok {
		return fmt.Errorf("Webhook URL 未配置")
	}
	
	payload := map[string]interface{}{
		"title":   title,
		"message": message,
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	}
	
	return n.sendHTTPPost(url, payload)
}

// sendEmail 发送邮件通知
func (n *NotificationManager) sendEmail(account model.CallbackAccount, title, message string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(account.Config), &config); err != nil {
		return err
	}

	host, _ := config["smtp_host"].(string)
	if host == "" {
		return fmt.Errorf("SMTP 服务器地址未配置")
	}
	port := 587
	if p, ok := config["smtp_port"].(float64); ok && p > 0 {
		port = int(p)
	}
	user, _ := config["smtp_user"].(string)
	password, _ := config["smtp_password"].(string)
	from, _ := config["smtp_from"].(string)
	if from == "" {
		return fmt.Errorf("发件人地址未配置")
	}
	toEmails := parseEmailList(config["to_emails"])
	if len(toEmails) == 0 {
		return fmt.Errorf("收件人地址未配置")
	}

	msg := buildEmailMessage(from, toEmails, title, message)
	addr := fmt.Sprintf("%s:%d", host, port)

	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, password, host)
	}

	if err := smtp.SendMail(addr, auth, from, toEmails, msg); err != nil {
		return fmt.Errorf("SMTP 发送失败: %w", err)
	}
	return nil
}

// parseEmailList 解析逗号分隔的收件人列表并去空
func parseEmailList(raw interface{}) []string {
	s, _ := raw.(string)
	if s == "" {
		return nil
	}
	var emails []string
	for _, part := range strings.Split(s, ",") {
		if e := strings.TrimSpace(part); e != "" {
			emails = append(emails, e)
		}
	}
	return emails
}

// buildEmailMessage 构造 RFC 5322 邮件内容
func buildEmailMessage(from string, to []string, title, message string) []byte {
	var buf bytes.Buffer
	header := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nDate: %s\r\n\r\n",
		from, strings.Join(to, ", "), mime.QEncoding.Encode("UTF-8", title), time.Now().Format(time.RFC1123Z))
	buf.WriteString(header)
	buf.WriteString(message)
	buf.WriteString("\r\n")
	return buf.Bytes()
}

// sendWechatWork 发送企业微信通知
func (n *NotificationManager) sendWechatWork(account model.CallbackAccount, title, message string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(account.Config), &config); err != nil {
		return err
	}
	
	webhookURL, ok := config["webhook_url"].(string)
	if !ok {
		return fmt.Errorf("企业微信 Webhook URL 未配置")
	}
	
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"content": fmt.Sprintf("## %s\n\n%s", title, message),
		},
	}
	
	return n.sendHTTPPost(webhookURL, payload)
}

// sendDingtalk 发送钉钉通知
func (n *NotificationManager) sendDingtalk(account model.CallbackAccount, title, message string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(account.Config), &config); err != nil {
		return err
	}
	
	webhookURL, ok := config["webhook_url"].(string)
	if !ok {
		return fmt.Errorf("钉钉 Webhook URL 未配置")
	}
	
	payload := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]interface{}{
			"title": title,
			"text":  message,
		},
	}
	
	return n.sendHTTPPost(webhookURL, payload)
}

// sendTelegram 发送 Telegram 通知
func (n *NotificationManager) sendTelegram(account model.CallbackAccount, title, message string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(account.Config), &config); err != nil {
		return err
	}
	
	botToken, ok := config["bot_token"].(string)
	if !ok {
		return fmt.Errorf("Telegram Bot Token 未配置")
	}
	
	chatID, ok := config["chat_id"].(string)
	if !ok {
		return fmt.Errorf("Telegram Chat ID 未配置")
	}
	
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text":    fmt.Sprintf("*%s*\n\n%s", title, message),
		"parse_mode": "Markdown",
	}
	
	return n.sendHTTPPost(url, payload)
}

// sendDiscord 发送 Discord 通知
func (n *NotificationManager) sendDiscord(account model.CallbackAccount, title, message string) error {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(account.Config), &config); err != nil {
		return err
	}
	
	webhookURL, ok := config["webhook_url"].(string)
	if !ok {
		return fmt.Errorf("Discord Webhook URL 未配置")
	}
	
	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       title,
				"description": message,
				"color":       15158332, // 红色
				"timestamp":   time.Now().Format(time.RFC3339),
			},
		},
	}
	
	return n.sendHTTPPost(webhookURL, payload)
}

// sendHTTPPost 发送 HTTP POST 请求
func (n *NotificationManager) sendHTTPPost(url string, payload interface{}) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP 请求失败 (%d): %s", resp.StatusCode, string(body))
	}
	
	return nil
}

// SendNotification 发送通知（别名方法，供外部调用）
func (n *NotificationManager) SendNotification(channelID uint, title, message string) error {
	return n.Send(channelID, title, message)
}

// TestNotification 测试通知
func (n *NotificationManager) TestNotification(accountID uint) error {
	return n.Send(accountID, "测试通知", "这是一条来自 NetPanel 监控系统的测试通知。")
}
