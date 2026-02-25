package ddns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CallbackFunc IP 变化时的回调函数类型
// taskID: DDNS 任务 ID, oldIP: 旧 IP, newIP: 新 IP
type CallbackFunc func(taskID uint, oldIP, newIP string)

type ddnsEntry struct {
	cancel context.CancelFunc
}

// Manager DDNS 管理器
type Manager struct {
	db         *gorm.DB
	log        *logrus.Logger
	entries    sync.Map // map[uint]*ddnsEntry
	callbackFn CallbackFunc
}

// NewManager 创建 DDNS 管理器
func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

// SetCallbackFunc 设置 IP 变化回调函数（由 callback manager 注入）
func (m *Manager) SetCallbackFunc(fn CallbackFunc) {
	m.callbackFn = fn
}

// StartAll 启动所有已启用的 DDNS 任务
func (m *Manager) StartAll() {
	var tasks []model.DDNSTask
	m.db.Where("enable = ?", true).Find(&tasks)
	for _, t := range tasks {
		if err := m.Start(t.ID); err != nil {
			m.log.Errorf("[DDNS][%s] 启动失败: %v", t.Name, err)
		}
	}
}

// StopAll 停止所有 DDNS 任务
func (m *Manager) StopAll() {
	m.entries.Range(func(key, value interface{}) bool {
		entry := value.(*ddnsEntry)
		entry.cancel()
		return true
	})
}

// Start 启动指定 DDNS 任务
func (m *Manager) Start(id uint) error {
	m.Stop(id)

	var task model.DDNSTask
	if err := m.db.First(&task, id).Error; err != nil {
		return fmt.Errorf("DDNS 任务不存在: %w", err)
	}
	if !task.Enable {
		return fmt.Errorf("DDNS 任务 [%s] 未启用", task.Name)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &ddnsEntry{cancel: cancel}
	m.entries.Store(id, entry)

	go m.runDDNS(ctx, id, &task)

	m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     "running",
		"last_error": "",
	})
	m.log.Infof("[DDNS][%s] 已启动，间隔 %ds", task.Name, task.Interval)
	return nil
}

// Stop 停止指定 DDNS 任务
func (m *Manager) Stop(id uint) {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*ddnsEntry)
		entry.cancel()
		m.entries.Delete(id)
	}
	m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("status", "stopped")
}

// GetStatus 获取任务状态
func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.entries.Load(id); ok {
		return "running"
	}
	return "stopped"
}

// RunNow 立即执行一次 DDNS 更新
func (m *Manager) RunNow(id uint) error {
	var task model.DDNSTask
	if err := m.db.First(&task, id).Error; err != nil {
		return fmt.Errorf("DDNS 任务不存在: %w", err)
	}
	go m.doUpdate(id, &task)
	return nil
}

// runDDNS 定时循环执行 DDNS 更新
func (m *Manager) runDDNS(ctx context.Context, id uint, task *model.DDNSTask) {
	defer func() {
		m.entries.Delete(id)
		m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("status", "stopped")
		m.log.Infof("[DDNS][%d] 已停止", id)
	}()

	interval := time.Duration(task.Interval) * time.Second
	if interval < 30*time.Second {
		interval = 300 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 立即执行一次
	m.doUpdate(id, task)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 重新读取最新配置（用户可能修改了间隔等参数）
			var latestTask model.DDNSTask
			if err := m.db.First(&latestTask, id).Error; err != nil {
				m.log.Errorf("[DDNS][%d] 读取配置失败: %v", id, err)
				continue
			}
			// 若间隔变化，重启定时器
			newInterval := time.Duration(latestTask.Interval) * time.Second
			if newInterval < 30*time.Second {
				newInterval = 300 * time.Second
			}
			if newInterval != interval {
				ticker.Reset(newInterval)
				interval = newInterval
			}
			m.doUpdate(id, &latestTask)
		}
	}
}

// doUpdate 执行一次 DDNS 更新
func (m *Manager) doUpdate(id uint, task *model.DDNSTask) {
	// 若配置了关联域名账号，优先从账号读取凭证
	accessID, accessSecret, err := m.resolveCredentials(task)
	if err != nil {
		m.log.Errorf("[DDNS][%d] 获取凭证失败: %v", id, err)
		m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("last_error", err.Error())
		return
	}

	// 获取当前 IP
	currentIP, err := m.getIP(task)
	if err != nil {
		m.log.Errorf("[DDNS][%d] 获取 IP 失败: %v", id, err)
		m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("last_error", err.Error())
		return
	}

	// 检查 IP 是否变化（从数据库读取最新值）
	var dbTask model.DDNSTask
	if err := m.db.First(&dbTask, id).Error; err != nil {
		return
	}
	oldIP := dbTask.CurrentIP
	if oldIP == currentIP {
		m.log.Debugf("[DDNS][%d] IP 未变化: %s，跳过更新", id, currentIP)
		return
	}

	m.log.Infof("[DDNS][%d] IP 变化: %s -> %s，开始更新 DNS", id, oldIP, currentIP)

	// 解析域名列表
	var domains []string
	if task.Domains == "" {
		m.log.Warnf("[DDNS][%d] 域名列表为空，跳过", id)
		return
	}
	if err := json.Unmarshal([]byte(task.Domains), &domains); err != nil {
		// 兼容单域名字符串
		domains = []string{task.Domains}
	}
	if len(domains) == 0 {
		m.log.Warnf("[DDNS][%d] 域名列表为空，跳过", id)
		return
	}

	// 创建 DNS 服务商实例
	provider := NewProvider(task.Provider, accessID, accessSecret)
	if provider == nil {
		errMsg := fmt.Sprintf("不支持的 DNS 服务商: %s", task.Provider)
		m.log.Errorf("[DDNS][%d] %s", id, errMsg)
		m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("last_error", errMsg)
		return
	}

	// 确定记录类型
	recordType := "A"
	if task.TaskType == "IPv6" {
		recordType = "AAAA"
	}

	// 逐个更新域名
	var lastErr string
	successCount := 0
	for _, domain := range domains {
		if domain == "" {
			continue
		}
		if err := provider.UpdateRecord(domain, recordType, currentIP, task.TTL); err != nil {
			m.log.Errorf("[DDNS][%d] 更新域名 %s 失败: %v", id, domain, err)
			lastErr = err.Error()
		} else {
			m.log.Infof("[DDNS][%d] 域名 %s 更新成功: %s", id, domain, currentIP)
			successCount++
		}
	}

	now := time.Now()
	updates := map[string]interface{}{
		"current_ip":       currentIP,
		"last_update_time": now,
		"last_error":       lastErr,
	}
	m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Updates(updates)

	// IP 变化时触发回调
	if successCount > 0 && m.callbackFn != nil {
		m.log.Debugf("[DDNS][%d] 触发 IP 变化回调: %s -> %s", id, oldIP, currentIP)
		go m.callbackFn(id, oldIP, currentIP)
	}
}

// resolveCredentials 解析凭证：优先使用关联域名账号，否则使用任务自身配置
func (m *Manager) resolveCredentials(task *model.DDNSTask) (accessID, accessSecret string, err error) {
	// 若配置了关联域名账号 ID，从账号表读取凭证
	if task.DomainAccountID > 0 {
		var account model.DomainAccount
		if err := m.db.First(&account, task.DomainAccountID).Error; err != nil {
			return "", "", fmt.Errorf("关联域名账号 [ID=%d] 不存在: %w", task.DomainAccountID, err)
		}
		// 若任务的 Provider 为空，使用账号的 Provider
		if task.Provider == "" {
			task.Provider = account.Provider
		}
		return account.AccessID, account.AccessSecret, nil
	}
	// 使用任务自身凭证
	return task.AccessID, task.AccessSecret, nil
}

// getIP 根据配置获取当前 IP 地址
func (m *Manager) getIP(task *model.DDNSTask) (string, error) {
	var ip string
	var err error

	switch task.IPGetType {
	case "custom":
		// 直接使用 NetInterface 字段存储的自定义 IP
		ip = task.NetInterface
		if ip == "" {
			return "", fmt.Errorf("自定义 IP 为空")
		}
	case "interface":
		ip, err = getIPFromInterface(task.NetInterface, task.TaskType)
		if err != nil {
			return "", err
		}
	default: // "url" 或空
		ip, err = getIPFromURL(task)
		if err != nil {
			return "", err
		}
	}

	// 应用自定义正则过滤（用户可指定从响应中提取特定格式的 IP）
	if task.IPRegex != "" && ip != "" {
		re, err := regexp.Compile(task.IPRegex)
		if err != nil {
			m.log.Warnf("[DDNS] 自定义 IP 正则无效 [%s]: %v，跳过过滤", task.IPRegex, err)
		} else {
			matched := re.FindString(ip)
			if matched == "" {
				return "", fmt.Errorf("IP [%s] 不匹配自定义正则 [%s]", ip, task.IPRegex)
			}
			ip = matched
		}
	}

	return ip, nil
}

// getIPFromURL 从 URL 获取 IP
func getIPFromURL(task *model.DDNSTask) (string, error) {
	var urls []string
	if task.IPGetURLs != "" {
		if err := json.Unmarshal([]byte(task.IPGetURLs), &urls); err != nil {
			// 兼容单 URL 字符串
			urls = []string{task.IPGetURLs}
		}
	}

	// 使用默认 IP 查询接口
	if len(urls) == 0 {
		if task.TaskType == "IPv6" {
			urls = []string{
				"https://6.ipw.cn",
				"https://ipv6.ddnspod.com",
				"https://v6.ident.me",
			}
		} else {
			urls = []string{
				"https://4.ipw.cn",
				"https://ip.3322.net",
				"https://myip4.ipip.net",
				"https://v4.ident.me",
			}
		}
	}

	// 优先使用用户自定义正则，否则使用默认 IP 正则
	var ipRegexStr string
	if task.IPRegex != "" {
		ipRegexStr = task.IPRegex
	} else if task.TaskType == "IPv6" {
		ipRegexStr = `([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}`
	} else {
		ipRegexStr = `(25[0-5]|2[0-4]\d|1\d{2}|[1-9]\d|\d)(\.(25[0-5]|2[0-4]\d|1\d{2}|[1-9]\d|\d)){3}`
	}
	re, err := regexp.Compile(ipRegexStr)
	if err != nil {
		return "", fmt.Errorf("IP 正则编译失败: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	var lastErr error
	for _, rawURL := range urls {
		resp, err := client.Get(rawURL)
		if err != nil {
			lastErr = err
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		ip := re.FindString(string(body))
		if ip != "" {
			return ip, nil
		}
	}

	if lastErr != nil {
		return "", fmt.Errorf("所有 IP 查询接口均失败，最后错误: %w", lastErr)
	}
	return "", fmt.Errorf("所有 IP 查询接口均未返回有效 IP")
}

// getIPFromInterface 从网络接口获取 IP
func getIPFromInterface(ifaceName, ipType string) (string, error) {
	if ifaceName == "" {
		return "", fmt.Errorf("网络接口名称为空")
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		return "", fmt.Errorf("找不到网络接口 [%s]: %w", ifaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return "", fmt.Errorf("获取接口 [%s] 地址失败: %w", ifaceName, err)
	}

	for _, addr := range addrs {
		var ip net.IP
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue
		}
		if ipType == "IPv6" {
			// 排除 IPv4 映射地址，只取纯 IPv6
			if ip.To4() == nil && ip.To16() != nil {
				return ip.String(), nil
			}
		} else {
			if ip.To4() != nil {
				return ip.String(), nil
			}
		}
	}
	return "", fmt.Errorf("接口 [%s] 上未找到 %s 地址", ifaceName, ipType)
}