package monitor

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
)

// ProbeEngine 服务探测引擎
type ProbeEngine struct {
	db      *gorm.DB
	manager *Manager
	
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// 探测任务
	probeTickers map[uint]*time.Ticker
	mu           sync.RWMutex
}

// NewProbeEngine 创建探测引擎
func NewProbeEngine(db *gorm.DB, manager *Manager) *ProbeEngine {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ProbeEngine{
		db:           db,
		manager:      manager,
		ctx:          ctx,
		cancel:       cancel,
		probeTickers: make(map[uint]*time.Ticker),
	}
}

// Start 启动探测引擎
func (p *ProbeEngine) Start() {
	log.Println("[ProbeEngine] 启动探测引擎...")
	
	// 加载所有启用的探测配置
	var probes []model.MonitorProbe
	p.db.Where("enable = ?", true).Find(&probes)
	
	for _, probe := range probes {
		p.StartProbe(probe)
	}
	
	log.Printf("[ProbeEngine] 探测引擎启动完成，加载了 %d 个探测任务\n", len(probes))
}

// Stop 停止探测引擎
func (p *ProbeEngine) Stop() {
	log.Println("[ProbeEngine] 停止探测引擎...")
	
	p.cancel()
	
	// 停止所有探测任务
	p.mu.Lock()
	for _, ticker := range p.probeTickers {
		ticker.Stop()
	}
	p.probeTickers = make(map[uint]*time.Ticker)
	p.mu.Unlock()
	
	p.wg.Wait()
	log.Println("[ProbeEngine] 探测引擎已停止")
}

// StartProbe 启动单个探测任务
func (p *ProbeEngine) StartProbe(probe model.MonitorProbe) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	// 如果已存在，先停止
	if ticker, ok := p.probeTickers[probe.ID]; ok {
		ticker.Stop()
	}
	
	// 创建定时器
	ticker := time.NewTicker(time.Duration(probe.Interval) * time.Second)
	p.probeTickers[probe.ID] = ticker
	
	// 启动探测协程
	p.wg.Add(1)
	go func(probe model.MonitorProbe) {
		defer p.wg.Done()
		
		// 立即执行一次
		p.executeProbe(probe)
		
		for {
			select {
			case <-p.ctx.Done():
				return
			case <-ticker.C:
				p.executeProbe(probe)
			}
		}
	}(probe)
	
	log.Printf("[ProbeEngine] 启动探测任务: %s (间隔 %d 秒)\n", probe.Name, probe.Interval)
}

// StopProbe 停止单个探测任务
func (p *ProbeEngine) StopProbe(probeID uint) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if ticker, ok := p.probeTickers[probeID]; ok {
		ticker.Stop()
		delete(p.probeTickers, probeID)
		log.Printf("[ProbeEngine] 停止探测任务: %d\n", probeID)
	}
}

// executeProbe 执行探测
func (p *ProbeEngine) executeProbe(probe model.MonitorProbe) {
	// 解析执行探测的服务器列表
	var serverIDs []uint
	if err := json.Unmarshal([]byte(probe.ServerIDs), &serverIDs); err != nil {
		log.Printf("[ProbeEngine] 解析服务器列表失败: %v\n", err)
		return
	}
	
	// 在每个服务器上执行探测
	for _, serverID := range serverIDs {
		go p.probeOnServer(probe, serverID)
	}
}

// probeOnServer 在指定服务器上执行探测
func (p *ProbeEngine) probeOnServer(probe model.MonitorProbe, serverID uint) {
	result := &model.MonitorProbeResult{
		ProbeID:   probe.ID,
		ServerID:  serverID,
		Timestamp: time.Now(),
	}
	
	var success bool
	var responseTime int64
	var statusCode int
	var err error
	
	// 根据探测类型执行
	switch probe.ProbeType {
	case "tcp":
		success, responseTime, err = p.manager.Collector.ProbeTCP(probe.TargetAddr, probe.TargetPort, probe.Timeout)
	case "udp":
		success, responseTime, err = p.manager.Collector.ProbeUDP(probe.TargetAddr, probe.TargetPort, probe.Timeout)
	case "http", "https":
		url := probe.TargetAddr
		if probe.HTTPPath != "" {
			url = url + probe.HTTPPath
		}
		if probe.ProbeType == "https" && !contains(url, "https://") {
			url = "https://" + url
		} else if probe.ProbeType == "http" && !contains(url, "http://") {
			url = "http://" + url
		}
		success, responseTime, statusCode, err = p.manager.Collector.ProbeHTTP(url, probe.Timeout)
	case "icmp":
		// TODO: ICMP Ping 探测
		log.Printf("[ProbeEngine] ICMP 探测暂未实现\n")
		return
	default:
		log.Printf("[ProbeEngine] 不支持的探测类型: %s\n", probe.ProbeType)
		return
	}
	
	result.Success = success
	result.ResponseTime = responseTime
	result.StatusCode = statusCode
	
	if err != nil {
		result.ErrorMsg = err.Error()
	}
	
	// 保存探测结果
	if err := p.db.Create(result).Error; err != nil {
		log.Printf("[ProbeEngine] 保存探测结果失败: %v\n", err)
	}
	
	// 检查是否需要触发告警
	p.checkProbeAlert(probe, serverID, success)
}

// checkProbeAlert 检查探测告警
func (p *ProbeEngine) checkProbeAlert(probe model.MonitorProbe, serverID uint, success bool) {
	// 获取最近的探测结果
	var recentResults []model.MonitorProbeResult
	p.db.Where("probe_id = ? AND server_id = ?", probe.ID, serverID).
		Order("timestamp DESC").
		Limit(probe.FailThreshold).
		Find(&recentResults)
	
	if len(recentResults) < probe.FailThreshold {
		return
	}
	
	// 检查是否连续失败
	allFailed := true
	for _, r := range recentResults {
		if r.Success {
			allFailed = false
			break
		}
	}
	
	if allFailed {
		// 触发告警
		log.Printf("[ProbeEngine] 探测 %s 连续失败 %d 次，触发告警\n", probe.Name, probe.FailThreshold)
		// TODO: 调用告警引擎
	}
}

// ListProbes 列出探测配置
func (p *ProbeEngine) ListProbes(enable *bool) ([]model.MonitorProbe, error) {
	query := p.db.Model(&model.MonitorProbe{})
	
	if enable != nil {
		query = query.Where("enable = ?", *enable)
	}
	
	var probes []model.MonitorProbe
	err := query.Order("id ASC").Find(&probes).Error
	return probes, err
}

// CreateProbe 创建探测配置
func (p *ProbeEngine) CreateProbe(probe *model.MonitorProbe) error {
	if err := p.db.Create(probe).Error; err != nil {
		return err
	}
	
	if probe.Enable {
		p.StartProbe(*probe)
	}
	
	return nil
}

// UpdateProbe 更新探测配置
func (p *ProbeEngine) UpdateProbe(probe *model.MonitorProbe) error {
	if err := p.db.Save(probe).Error; err != nil {
		return err
	}
	
	// 重启探测任务
	p.StopProbe(probe.ID)
	if probe.Enable {
		p.StartProbe(*probe)
	}
	
	return nil
}

// DeleteProbe 删除探测配置
func (p *ProbeEngine) DeleteProbe(id uint) error {
	// 停止探测任务
	p.StopProbe(id)
	
	return p.db.Transaction(func(tx *gorm.DB) error {
		// 删除探测配置
		if err := tx.Delete(&model.MonitorProbe{}, id).Error; err != nil {
			return err
		}
		
		// 删除探测结果
		tx.Where("probe_id = ?", id).Delete(&model.MonitorProbeResult{})
		
		return nil
	})
}

// GetProbeResults 获取探测结果
func (p *ProbeEngine) GetProbeResults(probeID, serverID uint, start, end time.Time) ([]model.MonitorProbeResult, error) {
	query := p.db.Where("probe_id = ?", probeID)
	
	if serverID > 0 {
		query = query.Where("server_id = ?", serverID)
	}
	
	if !start.IsZero() && !end.IsZero() {
		query = query.Where("timestamp BETWEEN ? AND ?", start, end)
	}
	
	var results []model.MonitorProbeResult
	err := query.Order("timestamp ASC").Find(&results).Error
	return results, err
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s[:len(substr)] == substr || len(s) > len(substr) && s[len(s)-len(substr):] == substr)
}
