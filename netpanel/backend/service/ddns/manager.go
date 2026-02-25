package ddns

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ddnsEntry struct {
	cancel context.CancelFunc
}

// Manager DDNS 管理器
type Manager struct {
	db      *gorm.DB
	log     *logrus.Logger
	entries sync.Map // map[uint]*ddnsEntry
}

func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	return &Manager{db: db, log: log}
}

func (m *Manager) StartAll() {
	var tasks []model.DDNSTask
	m.db.Where("enable = ?", true).Find(&tasks)
	for _, t := range tasks {
		if err := m.Start(t.ID); err != nil {
			m.log.Errorf("DDNS [%s] 启动失败: %v", t.Name, err)
		}
	}
}

func (m *Manager) StopAll() {
	m.entries.Range(func(key, value interface{}) bool {
		entry := value.(*ddnsEntry)
		entry.cancel()
		return true
	})
}

func (m *Manager) Start(id uint) error {
	m.Stop(id)

	var task model.DDNSTask
	if err := m.db.First(&task, id).Error; err != nil {
		return fmt.Errorf("DDNS 任务不存在: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	entry := &ddnsEntry{cancel: cancel}
	m.entries.Store(id, entry)

	go m.runDDNS(ctx, id, &task)

	m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("status", "running")
	return nil
}

func (m *Manager) Stop(id uint) {
	if val, ok := m.entries.Load(id); ok {
		entry := val.(*ddnsEntry)
		entry.cancel()
		m.entries.Delete(id)
	}
	m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("status", "stopped")
}

func (m *Manager) GetStatus(id uint) string {
	if _, ok := m.entries.Load(id); ok {
		return "running"
	}
	return "stopped"
}

func (m *Manager) RunNow(id uint) error {
	var task model.DDNSTask
	if err := m.db.First(&task, id).Error; err != nil {
		return fmt.Errorf("DDNS 任务不存在: %w", err)
	}
	go m.doUpdate(id, &task)
	return nil
}

func (m *Manager) runDDNS(ctx context.Context, id uint, task *model.DDNSTask) {
	interval := time.Duration(task.Interval) * time.Second
	if interval <= 0 {
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
			// 重新读取最新配置
			var latestTask model.DDNSTask
			if err := m.db.First(&latestTask, id).Error; err == nil {
				m.doUpdate(id, &latestTask)
			}
		}
	}
}

func (m *Manager) doUpdate(id uint, task *model.DDNSTask) {
	// 获取当前 IP
	currentIP, err := m.getIP(task)
	if err != nil {
		m.log.Errorf("[DDNS][%d] 获取 IP 失败: %v", id, err)
		m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("last_error", err.Error())
		return
	}

	// 检查 IP 是否变化
	var dbTask model.DDNSTask
	m.db.First(&dbTask, id)
	if dbTask.CurrentIP == currentIP {
		m.log.Debugf("[DDNS][%d] IP 未变化: %s", id, currentIP)
		return
	}

	m.log.Infof("[DDNS][%d] IP 变化: %s -> %s", id, dbTask.CurrentIP, currentIP)

	// 解析域名列表
	var domains []string
	if err := json.Unmarshal([]byte(task.Domains), &domains); err != nil {
		m.log.Errorf("[DDNS][%d] 解析域名列表失败: %v", id, err)
		return
	}

	// 更新 DNS 记录
	provider := NewProvider(task.Provider, task.AccessID, task.AccessSecret)
	if provider == nil {
		m.log.Errorf("[DDNS][%d] 不支持的 DNS 服务商: %s", id, task.Provider)
		return
	}

	for _, domain := range domains {
		if err := provider.UpdateRecord(domain, task.TaskType, currentIP, task.TTL); err != nil {
			m.log.Errorf("[DDNS][%d] 更新域名 %s 失败: %v", id, domain, err)
			m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Update("last_error", err.Error())
			return
		}
		m.log.Infof("[DDNS][%d] 域名 %s 更新成功: %s", id, domain, currentIP)
	}

	now := time.Now()
	m.db.Model(&model.DDNSTask{}).Where("id = ?", id).Updates(map[string]interface{}{
		"current_ip":       currentIP,
		"last_update_time": now,
		"last_error":       "",
	})
}

// getIP 获取当前 IP 地址
func (m *Manager) getIP(task *model.DDNSTask) (string, error) {
	switch task.IPGetType {
	case "custom":
		return task.NetInterface, nil // 直接使用自定义 IP
	case "interface":
		return getIPFromInterface(task.NetInterface, task.TaskType)
	default: // url
		return getIPFromURL(task)
	}
}

// getIPFromURL 从 URL 获取 IP
func getIPFromURL(task *model.DDNSTask) (string, error) {
	var urls []string
	if task.IPGetURLs != "" {
		json.Unmarshal([]byte(task.IPGetURLs), &urls)
	}

	if len(urls) == 0 {
		if task.TaskType == "IPv6" {
			urls = []string{"https://6.ipw.cn", "https://ipv6.ddnspod.com"}
		} else {
			urls = []string{"https://4.ipw.cn", "https://ip.3322.net", "https://myip4.ipip.net"}
		}
	}

	var ipRegex string
	if task.TaskType == "IPv6" {
		ipRegex = `([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}`
	} else {
		ipRegex = `((25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])\.){3,3}(25[0-5]|(2[0-4]|1{0,1}[0-9]){0,1}[0-9])`
	}
	re := regexp.MustCompile(ipRegex)

	client := &http.Client{Timeout: 10 * time.Second}
	for _, url := range urls {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		ip := re.FindString(string(body))
		if ip != "" {
			return ip, nil
		}
	}
	return "", fmt.Errorf("所有 IP 查询接口均失败")
}

// getIPFromInterface 从网络接口获取 IP
func getIPFromInterface(ifaceName, ipType string) (string, error) {
	// TODO: 实现从网络接口获取 IP
	return "", fmt.Errorf("从网络接口获取 IP 功能待实现")
}
