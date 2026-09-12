package ai

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/caddy"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/linereg"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/selector"
	"github.com/netpanel/netpanel/service/tunservice"
	"github.com/netpanel/netpanel/service/wireguard"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Configurator AI 配置执行器：接收自然语言指令，调用内部工具执行配置操作。
// 与 MCP Server 共享相同的 Manager 引用，避免重复的网络往返。
type Configurator struct {
	db             *gorm.DB
	log            *logrus.Logger
	tunserviceMgr  *tunservice.Manager
	lineregMgr     *linereg.Manager
	frpMgr         *frp.Manager
	npsMgr         *nps.Manager
	easytierMgr    *easytier.Manager
	wireguardMgr   *wireguard.Manager
	caddyMgr       *caddy.Manager
}

// ConfigRequest AI 生成的配置请求。
type ConfigRequest struct {
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
}

// ProbeConfigSnapshot 探测配置快照。
type ProbeConfigSnapshot struct {
	IntervalSec      int    `json:"interval_sec"`
	FailureThreshold int    `json:"failure_threshold"`
	ToleranceMs      int    `json:"tolerance_ms"`
	MaxConcurrent    int    `json:"max_concurrent"`
	ToolFilter       string `json:"tool_filter"`
	RebindMode       string `json:"rebind_mode"`
}

// NewConfigurator 创建配置执行器。
func NewConfigurator(
	db *gorm.DB,
	log *logrus.Logger,
	tunserviceMgr *tunservice.Manager,
	lineregMgr *linereg.Manager,
	frpMgr *frp.Manager,
	npsMgr *nps.Manager,
	easytierMgr *easytier.Manager,
	wireguardMgr *wireguard.Manager,
	caddyMgr *caddy.Manager,
) *Configurator {
	return &Configurator{
		db:             db,
		log:            log,
		tunserviceMgr:  tunserviceMgr,
		lineregMgr:     lineregMgr,
		frpMgr:         frpMgr,
		npsMgr:         npsMgr,
		easytierMgr:    easytierMgr,
		wireguardMgr:   wireguardMgr,
		caddyMgr:       caddyMgr,
	}
}

// ContextSnapshot 返回当前穿透环境的完整快照，用于注入到 AI system prompt。
func (c *Configurator) ContextSnapshot() string {
	var sb strings.Builder

	// 穿透服务列表
	services, err := c.tunserviceMgr.List()
	if err != nil {
		sb.WriteString(fmt.Sprintf("获取服务列表失败: %v\n", err))
	} else {
		sb.WriteString(fmt.Sprintf("## 穿透服务（共 %d 个）\n", len(services)))
		for _, svc := range services {
			sb.WriteString(fmt.Sprintf("- [%d] %s (启用=%v, 线路=%d)\n",
				svc.ID, svc.Name, svc.Enable, len(svc.Lines)))
			for _, line := range svc.Lines {
				sb.WriteString(fmt.Sprintf("  - 线路 %s: %s (状态=%s)\n",
					line.ID, line.Name, line.Status))
			}
		}
	}

	// 选线快照
	snapshot := c.lineregMgr.Selector().Snapshot()
	sb.WriteString(fmt.Sprintf("\n## 选线状态\n"))
	sb.WriteString(fmt.Sprintf("- 当前选中: %s\n", snapshot.Current))
	sb.WriteString(fmt.Sprintf("- 锁定线路: %s\n", snapshot.Locked))
	sb.WriteString(fmt.Sprintf("- 线路数: %d\n", len(snapshot.Lines)))

	// 探测配置
	config := c.probeConfigSnapshot()
	sb.WriteString(fmt.Sprintf("\n## 探测配置\n"))
	sb.WriteString(fmt.Sprintf("- 间隔: %ds\n", config.IntervalSec))
	sb.WriteString(fmt.Sprintf("- 失败阈值: %d\n", config.FailureThreshold))
	sb.WriteString(fmt.Sprintf("- 容差: %dms\n", config.ToleranceMs))
	sb.WriteString(fmt.Sprintf("- 并发上限: %d\n", config.MaxConcurrent))
	sb.WriteString(fmt.Sprintf("- 工具过滤: %s\n", config.ToolFilter))
	sb.WriteString(fmt.Sprintf("- 重绑模式: %s\n", config.RebindMode))

	// 端口转发规则
	var rules []model.PortForwardRule
	c.db.Order("id desc").Find(&rules)
	sb.WriteString(fmt.Sprintf("\n## 端口转发规则（共 %d 个）\n", len(rules)))
	for _, rule := range rules {
		sb.WriteString(fmt.Sprintf("- [%d] %s: %s:%d -> %s:%d (启用=%v, 状态=%s)\n",
			rule.ID, rule.Name, rule.ListenIP, rule.ListenPort,
			rule.TargetAddress, rule.TargetPort, rule.Enable, rule.Status))
	}

	return sb.String()
}

// probeConfigSnapshot 读取探测策略配置（通过系统配置表读取）。
func (c *Configurator) probeConfigSnapshot() ProbeConfigSnapshot {
	return ProbeConfigSnapshot{
		IntervalSec:      c.cfgInt("probe_interval_sec", 60),
		FailureThreshold: c.cfgInt("probe_failure_threshold", 2),
		ToleranceMs:      c.cfgInt("probe_tolerance_ms", 50),
		MaxConcurrent:    c.cfgInt("probe_max_concurrent", 8),
		ToolFilter:       c.cfgStr("probe_tool_filter", ""),
		RebindMode:       c.cfgStr("port_rebind_mode", "auto"),
	}
}

// cfgInt 从 SystemConfig 读取整数配置。
func (c *Configurator) cfgInt(key string, def int) int {
	var cfg model.SystemConfig
	if err := c.db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return def
	}
	var n int
	fmt.Sscanf(cfg.Value, "%d", &n)
	if n <= 0 {
		return def
	}
	return n
}

// cfgStr 从 SystemConfig 读取字符串配置。
func (c *Configurator) cfgStr(key, def string) string {
	var cfg model.SystemConfig
	if err := c.db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return def
	}
	return cfg.Value
}

// setCfgInt 写入（或更新）一条 SystemConfig 整数配置。
func (c *Configurator) setCfgInt(key string, value int) {
	var cfg model.SystemConfig
	if err := c.db.Where("key = ?", key).First(&cfg).Error; err == nil {
		cfg.Value = fmt.Sprintf("%d", value)
		c.db.Save(&cfg)
		return
	}
	c.db.Create(&model.SystemConfig{Key: key, Value: fmt.Sprintf("%d", value)})
}

// setCfgStr 写入（或更新）一条 SystemConfig 字符串配置。
func (c *Configurator) setCfgStr(key, value string) {
	var cfg model.SystemConfig
	if err := c.db.Where("key = ?", key).First(&cfg).Error; err == nil {
		cfg.Value = value
		c.db.Save(&cfg)
		return
	}
	c.db.Create(&model.SystemConfig{Key: key, Value: value})
}

// ExecuteInstruction 执行 AI 生成的配置指令。
func (c *Configurator) ExecuteInstruction(instruction string) (string, error) {
	var req ConfigRequest
	if err := json.Unmarshal([]byte(instruction), &req); err != nil {
		return "", fmt.Errorf("解析指令失败: %w", err)
	}

	c.log.Infof("[AI Configurator] 执行指令: %s", req.Action)

	switch req.Action {
	case "list_services":
		return c.handleListServices()
	case "list_lines":
		return c.handleListLines()
	case "create_service":
		return c.handleCreateService(req.Params)
	case "start_service":
		return c.handleStartService(req.Params)
	case "stop_service":
		return c.handleStopService(req.Params)
	case "create_frpc":
		return c.handleCreateFrpClient(req.Params)
	case "create_easytier":
		return c.handleCreateEasyTier(req.Params)
	case "create_wireguard":
		return c.handleCreateWireGuard(req.Params)
	case "set_probe_config":
		return c.handleSetProbeConfig(req.Params)
	case "describe":
		// 返回环境摘要供 AI 上下文使用
		return c.ContextSnapshot(), nil
	default:
		return "", fmt.Errorf("未知指令动作: %s", req.Action)
	}
}

// handleListServices 列出所有穿透服务。
func (c *Configurator) handleListServices() (string, error) {
	services, err := c.tunserviceMgr.List()
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, svc := range services {
		sb.WriteString(fmt.Sprintf("[%d] %s (启用=%v, 线路=%d)\n",
			svc.ID, svc.Name, svc.Enable, len(svc.Lines)))
		for _, line := range svc.Lines {
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", line.ID, line.Name, line.Status))
		}
	}
	return fmt.Sprintf("共有 %d 个穿透服务:\n%s", len(services), sb.String()), nil
}

// handleListLines 列出所有可用线路。
func (c *Configurator) handleListLines() (string, error) {
	lines := linereg.BuildLines(c.db)
	return fmt.Sprintf("共有 %d 条可用线路:\n%s", len(lines), formatLineList(lines)), nil
}

// handleCreateService 创建穿透服务。
func (c *Configurator) handleCreateService(params map[string]interface{}) (string, error) {
	name, _ := params["name"].(string)
	if name == "" {
		return "", fmt.Errorf("缺少服务名称")
	}

	svc := &model.TunService{
		Name:     name,
		Enable:   true,
		LineRefs: "[]",
	}
	if err := c.db.Create(svc).Error; err != nil {
		return "", fmt.Errorf("创建服务失败: %w", err)
	}

	return fmt.Sprintf("已创建穿透服务 [%d] %s", svc.ID, svc.Name), nil
}

// handleStartService 启动穿透服务。
func (c *Configurator) handleStartService(params map[string]interface{}) (string, error) {
	id, err := toUint(params["id"])
	if err != nil {
		return "", fmt.Errorf("缺少服务 ID")
	}
	if err := c.tunserviceMgr.Start(id); err != nil {
		return "", fmt.Errorf("启动服务失败: %w", err)
	}
	return fmt.Sprintf("已启动服务 [%d]", id), nil
}

// handleStopService 停止穿透服务。
func (c *Configurator) handleStopService(params map[string]interface{}) (string, error) {
	id, err := toUint(params["id"])
	if err != nil {
		return "", fmt.Errorf("缺少服务 ID")
	}
	c.tunserviceMgr.Stop(id)
	return fmt.Sprintf("已停止服务 [%d]", id), nil
}

// handleCreateFrpClient 创建 FRP 客户端配置。
func (c *Configurator) handleCreateFrpClient(params map[string]interface{}) (string, error) {
	name, _ := params["name"].(string)
	serverAddr, _ := params["server_addr"].(string)
	serverPort, _ := params["server_port"].(float64)
	proxyName, _ := params["proxy_name"].(string)
	proxyType, _ := params["proxy_type"].(string)
	localIP, _ := params["local_ip"].(string)
	localPort, _ := params["local_port"].(float64)
	remotePort, _ := params["remote_port"].(float64)

	if name == "" || serverAddr == "" || proxyName == "" {
		return "", fmt.Errorf("缺少必要参数: name, server_addr, proxy_name")
	}

	// 创建 frpc 配置
	frpc := &model.FrpcConfig{
		Name:       name,
		ServerAddr: serverAddr,
		ServerPort: int(serverPort),
		Enable:     true,
	}
	if err := c.db.Create(frpc).Error; err != nil {
		return "", fmt.Errorf("创建 FRP 配置失败: %w", err)
	}

	// 创建代理
	proxy := model.FrpcProxy{
		FrpcID:    frpc.ID,
		Name:      proxyName,
		Type:      proxyType,
		LocalIP:   localIP,
		LocalPort: int(localPort),
		RemotePort: int(remotePort),
		Enable:    true,
	}
	if err := c.db.Create(&proxy).Error; err != nil {
		return "", fmt.Errorf("创建代理失败: %w", err)
	}

	return fmt.Sprintf("已创建 FRP 客户端 [%d] %s，代理 [%d] %s",
		frpc.ID, frpc.Name, proxy.ID, proxy.Name), nil
}

// handleCreateEasyTier 创建 EasyTier 配置。
func (c *Configurator) handleCreateEasyTier(params map[string]interface{}) (string, error) {
	name, _ := params["name"].(string)
	serverAddr, _ := params["server_addr"].(string)

	if name == "" || serverAddr == "" {
		return "", fmt.Errorf("缺少必要参数: name, server_addr")
	}

	cfg := &model.EasytierClient{
		Name:       name,
		ServerAddr: serverAddr,
		Enable:     true,
	}
	if err := c.db.Create(cfg).Error; err != nil {
		return "", fmt.Errorf("创建 EasyTier 客户端失败: %w", err)
	}

	return fmt.Sprintf("已创建 EasyTier 客户端 [%d] %s: %s", cfg.ID, cfg.Name, serverAddr), nil
}

// handleCreateWireGuard 创建 WireGuard 配置。
func (c *Configurator) handleCreateWireGuard(params map[string]interface{}) (string, error) {
	name, _ := params["name"].(string)
	endpoint, _ := params["endpoint"].(string)

	if name == "" || endpoint == "" {
		return "", fmt.Errorf("缺少必要参数: name, endpoint")
	}

	cfg := &model.WireguardConfig{
		Name:   name,
		Enable: true,
	}
	if err := c.db.Create(cfg).Error; err != nil {
		return "", fmt.Errorf("创建 WireGuard 配置失败: %w", err)
	}

	peer := model.WireguardPeer{
		WireguardID: cfg.ID,
		Name:        name + "-peer",
		Endpoint:    endpoint,
		Enable:      true,
	}
	if err := c.db.Create(&peer).Error; err != nil {
		return "", fmt.Errorf("创建对端失败: %w", err)
	}

	return fmt.Sprintf("已创建 WireGuard 配置 [%d] %s: %s", cfg.ID, cfg.Name, endpoint), nil
}

// handleSetProbeConfig 设置探测配置。
func (c *Configurator) handleSetProbeConfig(params map[string]interface{}) (string, error) {
intervalSec := c.cfgInt("probe_interval_sec", 60)
failureThreshold := c.cfgInt("probe_failure_threshold", 2)
toleranceMs := c.cfgInt("probe_tolerance_ms", 50)
maxConcurrent := c.cfgInt("probe_max_concurrent", 8)
toolFilter := c.cfgStr("probe_tool_filter", "")
rebindMode := c.cfgStr("port_rebind_mode", "auto")

	// 覆盖传入字段（带范围校验）
	if v, ok := params["interval_sec"]; ok {
		if n, ok := v.(float64); ok && n >= 5 && n <= 3600 {
			intervalSec = int(n)
		}
	}
	if v, ok := params["failure_threshold"]; ok {
		if n, ok := v.(float64); ok && n >= 1 && n <= 10 {
			failureThreshold = int(n)
		}
	}
	if v, ok := params["tolerance_ms"]; ok {
		if n, ok := v.(float64); ok && n >= 0 && n <= 5000 {
			toleranceMs = int(n)
		}
	}
	if v, ok := params["max_concurrent"]; ok {
		if n, ok := v.(float64); ok && n >= 1 && n <= 64 {
			maxConcurrent = int(n)
		}
	}
	if v, ok := params["tool_filter"]; ok {
		if s, ok := v.(string); ok {
			toolFilter = s
		}
	}
	if v, ok := params["rebind_mode"]; ok {
		if s, ok := v.(string); ok && (s == "auto" || s == "manual" || s == "off") {
			rebindMode = s
		}
	}

	// 持久化
	c.setCfgInt("probe_interval_sec", intervalSec)
	c.setCfgInt("probe_failure_threshold", failureThreshold)
	c.setCfgInt("probe_tolerance_ms", toleranceMs)
	c.setCfgInt("probe_max_concurrent", maxConcurrent)
	c.setCfgStr("probe_tool_filter", toolFilter)
	c.setCfgStr("port_rebind_mode", rebindMode)

	// 应用
	c.lineregMgr.SetInterval(time.Duration(intervalSec) * time.Second)
	c.lineregMgr.SetFailureThreshold(failureThreshold)
	c.lineregMgr.SetTolerance(time.Duration(toleranceMs) * time.Millisecond)
	c.lineregMgr.SetMaxConcurrent(maxConcurrent)
	c.lineregMgr.SetToolFilter(toolFilter)
	c.lineregMgr.SetRebindMode(rebindMode)

	return fmt.Sprintf("已更新探测配置: interval=%ds, threshold=%d, tolerance=%dms, concurrent=%d, filter=%s, rebind=%s",
		int(intervalSec), failureThreshold, int(toleranceMs), maxConcurrent, toolFilter, rebindMode), nil
}

// formatLineList 格式化线路列表输出。
func formatLineList(lines []selector.Line) string {
	var sb strings.Builder
	for _, line := range lines {
		sb.WriteString(fmt.Sprintf("- %s: %s (%s:%s)\n", line.ID, line.Name, line.Tool, line.Address))
	}
	return sb.String()
}

// toUint 将 interface{} 转换为 uint。
func toUint(v interface{}) (uint, error) {
	switch t := v.(type) {
	case float64:
		return uint(t), nil
	case int:
		return uint(t), nil
	case string:
		var n uint
		fmt.Sscanf(t, "%d", &n)
		return n, nil
	default:
		return 0, fmt.Errorf("无法转换为 uint: %v", v)
	}
}
