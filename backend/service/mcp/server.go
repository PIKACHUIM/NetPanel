// Package mcp 提供 NetPanel 的 MCP（Model Context Protocol）服务端：
// 以 HTTP JSON-RPC 2.0 形式暴露基础工具调用接口（POST /mcp），
// 供 Claude Desktop 等 MCP 客户端发现并调用穿透服务管理、选线快照、
// 探测历史、工具日志与端口转发等能力。
//
// 本服务仅监听 127.0.0.1，避免将控制接口暴露到公网。
package mcp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/cftunnel"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/linereg"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/portforward"
	"github.com/netpanel/netpanel/service/tunservice"
	"github.com/netpanel/netpanel/service/wireguard"
)

// Server MCP 服务端：持有各业务管理器的引用，对外提供 JSON-RPC 2.0 接口。
type Server struct {
	db             *gorm.DB
	log            *logrus.Logger
	tunserviceMgr  *tunservice.Manager
	lineregMgr     *linereg.Manager
	frpMgr         *frp.Manager
	npsMgr         *nps.Manager
	easytierMgr    *easytier.Manager
	wireguardMgr   *wireguard.Manager
	cftunnelMgr    *cftunnel.Manager
	portforwardMgr *portforward.Manager
	addr           string
	token          string // 访问令牌（Authorization: Bearer）
	httpSrv        *http.Server
}

// NewServer 创建 MCP 服务端。addr 为监听地址（如 ":18090"），
// 实际绑定时会强制为 127.0.0.1 本地回环。token 为访问令牌，
// 客户端需在请求头携带 Authorization: Bearer <token>。
func NewServer(
	db *gorm.DB,
	log *logrus.Logger,
	tunserviceMgr *tunservice.Manager,
	lineregMgr *linereg.Manager,
	frpMgr *frp.Manager,
	npsMgr *nps.Manager,
	easytierMgr *easytier.Manager,
	wireguardMgr *wireguard.Manager,
	cftunnelMgr *cftunnel.Manager,
	portforwardMgr *portforward.Manager,
	addr string,
	token string,
) *Server {
	return &Server{
		db:             db,
		log:            log,
		tunserviceMgr:  tunserviceMgr,
		lineregMgr:     lineregMgr,
		frpMgr:         frpMgr,
		npsMgr:         npsMgr,
		easytierMgr:    easytierMgr,
		wireguardMgr:   wireguardMgr,
		cftunnelMgr:    cftunnelMgr,
		portforwardMgr: portforwardMgr,
		addr:           addr,
		token:          token,
	}
}

// Start 启动 HTTP 服务。仅监听 127.0.0.1：addr 传 ":18090" 会被强制改为
// "127.0.0.1:18090"。同步绑定端口以暴露监听错误，随后在 goroutine 中
// 运行 HTTP 服务。
func (s *Server) Start() error {
	if s.httpSrv != nil {
		return fmt.Errorf("MCP 服务已启动")
	}
	bindAddr := s.addr
	if strings.HasPrefix(bindAddr, ":") {
		bindAddr = "127.0.0.1" + bindAddr
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", s.handler)
	s.httpSrv = &http.Server{
		Addr:    bindAddr,
		Handler: mux,
	}
	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		s.httpSrv = nil
		return fmt.Errorf("MCP 服务监听失败: %w", err)
	}
	s.log.Infof("[MCP] JSON-RPC 服务已启动，监听 http://%s/mcp", bindAddr)
	go func() {
		if serveErr := s.httpSrv.Serve(ln); serveErr != nil && serveErr != http.ErrServerClosed {
			s.log.Errorf("[MCP] HTTP 服务异常退出: %v", serveErr)
		}
	}()
	return nil
}

// Stop 优雅关闭 HTTP 服务。
func (s *Server) Stop() error {
	if s.httpSrv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.httpSrv.Shutdown(ctx)
	s.httpSrv = nil
	return err
}

// authorized 校验请求头 Authorization: Bearer <token>。未配置 token 时放行
// （兼容旧行为）。
func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) == 1
}

// ===== JSON-RPC 2.0 请求处理 =====

// rpcRequest JSON-RPC 2.0 请求（id 保留原始字节，便于原样回写）。
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// handler 处理 POST /mcp 的 JSON-RPC 2.0 请求。
func (s *Server) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      nil,
			"error":   map[string]interface{}{"code": -32000, "message": "仅支持 POST 请求"},
		})
		return
	}

	// 令牌认证：防止本地其他进程或意外暴露的端口被未授权调用
	if s.token != "" && !s.authorized(r) {
		s.log.Warnf("[MCP] 未授权访问被拒绝: %s %s", r.RemoteAddr, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      nil,
			"error":   map[string]interface{}{"code": -32001, "message": "Unauthorized"},
		})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.writeRPCError(w, nil, -32700, "读取请求体失败: "+err.Error())
		return
	}
	var req rpcRequest
	if err := json.Unmarshal(body, &req); err != nil {
		s.writeRPCError(w, nil, -32700, "无效的 JSON 请求: "+err.Error())
		return
	}
	s.log.Debugf("[MCP] 收到请求: method=%s", req.Method)

	switch req.Method {
	case "initialize":
		s.writeResult(w, req.ID, map[string]interface{}{
			"protocolVersion": "2025-03-26",
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{}},
			"serverInfo": map[string]interface{}{
				"name":    "netpanel-mcp",
				"version": "0.1.0",
			},
		})
	case "notifications/initialized":
		// 通知：不返回任何响应体（无 id）
		return
	case "tools/list":
		s.writeResult(w, req.ID, map[string]interface{}{"tools": s.toolsList()})
	case "tools/call":
		s.writeResult(w, req.ID, s.handleToolCall(req.Params))
	default:
		s.writeRPCError(w, req.ID, -32601, fmt.Sprintf("未知方法: %s", req.Method))
	}
}

// authorized 校验请求是否携带有效令牌（恒定时间比较，防时序攻击）。
// 未配置 token 时放行（兼容旧行为）。
func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		return false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	return subtle.ConstantTimeCompare([]byte(token), []byte(s.token)) == 1
}

// writeResult 写 JSON-RPC 成功响应。
func (s *Server) writeResult(w http.ResponseWriter, id json.RawMessage, result interface{}) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"result":  result,
	}
	if len(id) > 0 {
		resp["id"] = json.RawMessage(id)
	}
	s.writeJSON(w, resp)
}

// writeRPCError 写 JSON-RPC 错误响应。
func (s *Server) writeRPCError(w http.ResponseWriter, id json.RawMessage, code int, message string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	if len(id) > 0 {
		resp["id"] = json.RawMessage(id)
	}
	s.writeJSON(w, resp)
}

// writeJSON 统一编码 JSON 响应。
func (s *Server) writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Warnf("[MCP] 响应编码失败: %v", err)
	}
}

// ===== tools/list：工具清单 =====

// toolsList 返回 MCP tools/list 结果中的工具数组。
func (s *Server) toolsList() []map[string]interface{} {
	str := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "string", "description": desc}
	}
	intg := func(desc string) map[string]interface{} {
		return map[string]interface{}{"type": "integer", "description": desc}
	}
	schema := func(properties map[string]interface{}, required []string) map[string]interface{} {
		if properties == nil {
			properties = map[string]interface{}{}
		}
		return map[string]interface{}{
			"type":       "object",
			"properties": properties,
			"required":   required,
		}
	}
	return []map[string]interface{}{
		{
			"name":        "tunservice_list",
			"description": "列出全部穿透服务及其线路（含实时状态）",
			"inputSchema": schema(nil, nil),
		},
		{
			"name":        "tunservice_start",
			"description": "启动指定穿透服务关联的全部线路",
			"inputSchema": schema(map[string]interface{}{"id": intg("穿透服务 ID")}, []string{"id"}),
		},
		{
			"name":        "tunservice_stop",
			"description": "停止指定穿透服务关联的全部线路",
			"inputSchema": schema(map[string]interface{}{"id": intg("穿透服务 ID")}, []string{"id"}),
		},
		{
			"name":        "line_snapshot",
			"description": "返回线路自动选线器当前状态快照（Current/Lines/Results/Locked）",
			"inputSchema": schema(nil, nil),
		},
		{
			"name":        "probe_history",
			"description": "返回指定穿透服务的线路探测历史（按线路分组，最近 limit 条）",
			"inputSchema": schema(map[string]interface{}{
				"id":    intg("穿透服务 ID"),
				"limit": intg("返回条数，默认 100，范围 1-500"),
			}, []string{"id"}),
		},
		{
			"name":        "tool_logs",
			"description": "返回指定工具实例的实时日志（tool 取值 easytier/cftunnel；frp/nps/wg 暂不支持）",
			"inputSchema": schema(map[string]interface{}{
				"tool": str("工具名：easytier / cftunnel / frp / nps / wg"),
				"id":   intg("实例 ID"),
			}, []string{"tool", "id"}),
		},
		{
			"name":        "portforward_list",
			"description": "列出全部端口转发规则",
			"inputSchema": schema(nil, nil),
		},
		{
			"name":        "portforward_start",
			"description": "启动指定端口转发规则",
			"inputSchema": schema(map[string]interface{}{"id": intg("端口转发规则 ID")}, []string{"id"}),
		},
		{
			"name":        "portforward_stop",
			"description": "停止指定端口转发规则",
			"inputSchema": schema(map[string]interface{}{"id": intg("端口转发规则 ID")}, []string{"id"}),
		},
		{
			"name":        "probe_config_get",
			"description": "读取线路自动选线探测策略（间隔/失败阈值/容差/并发/工具过滤/重绑模式）",
			"inputSchema": schema(nil, nil),
		},
		{
			"name":        "probe_config_set",
			"description": "更新线路自动选线探测策略（可只传要修改的字段；未传字段保持当前值）",
			"inputSchema": schema(map[string]interface{}{
				"interval_sec":      intg("探测间隔（秒），范围 5-3600"),
				"failure_threshold": intg("连续失败阈值（次），范围 1-10"),
				"tolerance_ms":      intg("切换防抖容差（毫秒），范围 0-5000"),
				"max_concurrent":    intg("单轮探测并发上限，范围 1-64"),
				"tool_filter":       str("参与自动选线的工具（逗号分隔，如 wireguard；空=全部）"),
				"rebind_mode":       str("端口层重绑模式：auto / manual / off"),
			}, nil),
		},
		{
			"name":        "tunservice_diag",
			"description": "异常诊断：列出全部穿透服务的状态与最近错误，并附带选线快照与待重绑清单",
			"inputSchema": schema(nil, nil),
		},
	}
}

// ===== tools/call：工具分发 =====

// callParams tools/call 请求参数。
type callParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// handleToolCall 分发 tools/call 请求到具体工具，返回 MCP 工具结果
// （成功：content[].text 放 JSON 字符串；失败：isError=true）。
func (s *Server) handleToolCall(raw json.RawMessage) map[string]interface{} {
	var params callParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return s.textError("解析 tools/call 参数失败: " + err.Error())
	}
	if params.Arguments == nil {
		params.Arguments = map[string]interface{}{}
	}

	switch params.Name {
	case "tunservice_list":
		views, err := s.tunserviceMgr.List()
		if err != nil {
			return s.textError("获取穿透服务列表失败: " + err.Error())
		}
		return s.textResult(views)

	case "tunservice_start":
		id, err := s.argUint(params.Arguments, "id")
		if err != nil {
			return s.textError(err.Error())
		}
		if err := s.tunserviceMgr.Start(id); err != nil {
			return s.textError("启动穿透服务失败: " + err.Error())
		}
		return s.textResult(map[string]interface{}{"ok": true, "id": id})

	case "tunservice_stop":
		id, err := s.argUint(params.Arguments, "id")
		if err != nil {
			return s.textError(err.Error())
		}
		s.tunserviceMgr.Stop(id)
		return s.textResult(map[string]interface{}{"ok": true, "id": id})

	case "line_snapshot":
		// Snapshot 返回 selector.State（Current/Lines/Results/Locked），
		// 字段均为导出且线程安全拷贝，可直接序列化。
		return s.textResult(s.lineregMgr.Selector().Snapshot())

	case "probe_history":
		id, err := s.argUint(params.Arguments, "id")
		if err != nil {
			return s.textError(err.Error())
		}
		limit := 100
		if v, ok := params.Arguments["limit"]; ok {
			n, err := toInt(v)
			if err != nil {
				return s.textError("limit 参数非法: " + err.Error())
			}
			limit = n
		}
		// 钳制 1-500
		if limit < 1 {
			limit = 1
		}
		if limit > 500 {
			limit = 500
		}
		hist, err := s.tunserviceMgr.History(id, limit)
		if err != nil {
			return s.textError("获取探测历史失败: " + err.Error())
		}
		return s.textResult(hist)

	case "tool_logs":
		return s.handleToolLogs(params.Arguments)

	case "portforward_list":
		var rules []model.PortForwardRule
		if err := s.db.Order("id desc").Find(&rules).Error; err != nil {
			return s.textError("获取端口转发列表失败: " + err.Error())
		}
		return s.textResult(rules)

	case "portforward_start":
		id, err := s.argUint(params.Arguments, "id")
		if err != nil {
			return s.textError(err.Error())
		}
		if err := s.portforwardMgr.Start(id); err != nil {
			return s.textError("启动端口转发失败: " + err.Error())
		}
		return s.textResult(map[string]interface{}{"ok": true, "id": id})

	case "portforward_stop":
		id, err := s.argUint(params.Arguments, "id")
		if err != nil {
			return s.textError(err.Error())
		}
		s.portforwardMgr.Stop(id)
		return s.textResult(map[string]interface{}{"ok": true, "id": id})

	case "probe_config_get":
		return s.textResult(s.probeConfigSnapshot())

	case "probe_config_set":
		return s.handleProbeConfigSet(params.Arguments)

	case "tunservice_diag":
		return s.handleTunserviceDiag()

	default:
		return s.textError(fmt.Sprintf("未知工具: %s", params.Name))
	}
}

// probeConfigSnapshot 读取探测策略配置（与 GET /v1/linereg/config 一致）。
func (s *Server) probeConfigSnapshot() map[string]interface{} {
	return map[string]interface{}{
		"interval_sec":      s.cfgInt("probe_interval_sec", 60),
		"failure_threshold": s.cfgInt("probe_failure_threshold", 2),
		"tolerance_ms":      s.cfgInt("probe_tolerance_ms", 50),
		"max_concurrent":    s.cfgInt("probe_max_concurrent", 8),
		"tool_filter":       s.cfgStr("probe_tool_filter", ""),
		"rebind_mode":       s.lineregMgr.RebindMode(),
	}
}

// handleProbeConfigSet 更新探测策略（支持部分字段；未传字段保持当前值）。
func (s *Server) handleProbeConfigSet(args map[string]interface{}) map[string]interface{} {
	intervalSec := s.cfgInt("probe_interval_sec", 60)
	failureThreshold := s.cfgInt("probe_failure_threshold", 2)
	toleranceMs := s.cfgInt("probe_tolerance_ms", 50)
	maxConcurrent := s.cfgInt("probe_max_concurrent", 8)
	toolFilter := s.cfgStr("probe_tool_filter", "")
	rebindMode := s.lineregMgr.RebindMode()

	// 覆盖传入字段（带范围校验）
	fields := []struct {
		key    string
		min,   max int
		dst    *int
	}{
		{"interval_sec", 5, 3600, &intervalSec},
		{"failure_threshold", 1, 10, &failureThreshold},
		{"tolerance_ms", 0, 5000, &toleranceMs},
		{"max_concurrent", 1, 64, &maxConcurrent},
	}
	for _, f := range fields {
		v, ok := args[f.key]
		if !ok {
			continue
		}
		n, err := toInt(v)
		if err != nil {
			return s.textError(f.key + " 参数非法: " + err.Error())
		}
		if n < f.min || n > f.max {
			return s.textError(fmt.Sprintf("%s 需在 %d-%d 范围内", f.key, f.min, f.max))
		}
		*f.dst = n
	}
	if v, ok := args["tool_filter"]; ok {
		if str, isStr := v.(string); isStr {
			toolFilter = str
		} else {
			return s.textError("tool_filter 参数需为字符串")
		}
	}
	if v, ok := args["rebind_mode"]; ok {
		str, isStr := v.(string)
		if !isStr || (str != "auto" && str != "manual" && str != "off") {
			return s.textError("rebind_mode 需为 auto/manual/off")
		}
		rebindMode = str
	}

	// 持久化
	s.setCfgInt("probe_interval_sec", intervalSec)
	s.setCfgInt("probe_failure_threshold", failureThreshold)
	s.setCfgInt("probe_tolerance_ms", toleranceMs)
	s.setCfgInt("probe_max_concurrent", maxConcurrent)
	s.setCfgStr("probe_tool_filter", toolFilter)
	s.setCfgStr("port_rebind_mode", rebindMode)

	// 应用
	s.lineregMgr.SetInterval(time.Duration(intervalSec) * time.Second)
	s.lineregMgr.SetFailureThreshold(failureThreshold)
	s.lineregMgr.SetTolerance(time.Duration(toleranceMs) * time.Millisecond)
	s.lineregMgr.SetMaxConcurrent(maxConcurrent)
	s.lineregMgr.SetToolFilter(toolFilter)
	s.lineregMgr.SetRebindMode(rebindMode)
	return s.textResult(map[string]interface{}{"ok": true, "config": s.probeConfigSnapshot()})
}

// handleTunserviceDiag 异常诊断：服务状态 + 最近错误 + 选线快照 + 待重绑清单。
func (s *Server) handleTunserviceDiag() map[string]interface{} {
	views, err := s.tunserviceMgr.List()
	if err != nil {
		return s.textError("获取穿透服务列表失败: " + err.Error())
	}
	return s.textResult(map[string]interface{}{
		"services":         views,
		"snapshot":         s.lineregMgr.Selector().Snapshot(),
		"rebind_mode":      s.lineregMgr.RebindMode(),
		"pending_rebinds":  s.lineregMgr.PendingRebinds(),
	})
}

// cfgInt 从 SystemConfig 读取整数配置；缺失或非法时返回 def。
func (s *Server) cfgInt(key string, def int) int {
	var cfg model.SystemConfig
	if err := s.db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return def
	}
	n, err := strconv.Atoi(cfg.Value)
	if err != nil {
		return def
	}
	return n
}

// cfgStr 从 SystemConfig 读取字符串配置；缺失时返回 def。
func (s *Server) cfgStr(key, def string) string {
	var cfg model.SystemConfig
	if err := s.db.Where("key = ?", key).First(&cfg).Error; err != nil {
		return def
	}
	return cfg.Value
}

// setCfgInt 写入（或更新）一条 SystemConfig 整数配置。
func (s *Server) setCfgInt(key string, value int) {
	var cfg model.SystemConfig
	if err := s.db.Where("key = ?", key).First(&cfg).Error; err == nil {
		cfg.Value = strconv.Itoa(value)
		s.db.Save(&cfg)
		return
	}
	s.db.Create(&model.SystemConfig{Key: key, Value: strconv.Itoa(value)})
}

// setCfgStr 写入（或更新）一条 SystemConfig 字符串配置。
func (s *Server) setCfgStr(key, value string) {
	var cfg model.SystemConfig
	if err := s.db.Where("key = ?", key).First(&cfg).Error; err == nil {
		cfg.Value = value
		s.db.Save(&cfg)
		return
	}
	s.db.Create(&model.SystemConfig{Key: key, Value: value})
}

// handleToolLogs 实现 tool_logs 工具：返回指定工具实例的实时日志。
// 经核对：easytier 提供 GetClientLogs、cftunnel 提供 GetLogs；
// frp / nps / wireguard 当前没有日志读取接口，返回明确错误。
func (s *Server) handleToolLogs(args map[string]interface{}) map[string]interface{} {
	tool, _ := args["tool"].(string)
	id, err := s.argUint(args, "id")
	if err != nil {
		return s.textError(err.Error())
	}
	// tail 限制返回行数（默认 100，最大 1000），避免超大日志拖垮 AI 上下文
	tail := 100
	if t, ok := args["tail"]; ok {
		if n, err := toInt(t); err != nil || n < 1 {
			return s.textError("tail 参数非法（需为 >=1 的整数）")
		} else if n > 1000 {
			tail = 1000
		} else {
			tail = n
		}
	}

	lines := func(logs []string) map[string]interface{} {
		if len(logs) > tail {
			logs = logs[len(logs)-tail:]
		}
		return s.textResult(map[string]interface{}{"lines": logs, "count": len(logs)})
	}

	switch tool {
	case "easytier":
		if s.easytierMgr == nil {
			return s.textError("easytier 管理器未初始化")
		}
		return lines(s.easytierMgr.GetClientLogs(id))
	case "cftunnel":
		if s.cftunnelMgr == nil {
			return s.textError("cftunnel 管理器未初始化")
		}
		return lines(s.cftunnelMgr.GetLogs(id))
	case "frp", "nps", "wg":
		return s.textError(fmt.Sprintf("工具 %s 暂不支持日志查询（未实现 GetLogs 接口）", tool))
	default:
		return s.textError(fmt.Sprintf("未知工具: %s（可选值：easytier / cftunnel / frp / nps / wg）", tool))
	}
}

// ===== 工具结果与参数解析 =====

// textResult 构造 MCP 工具成功结果：text 字段中放入 JSON 序列化后的数据。
func (s *Server) textResult(payload interface{}) map[string]interface{} {
	b, err := json.Marshal(payload)
	if err != nil {
		return s.textError("序列化结果失败: " + err.Error())
	}
	return map[string]interface{}{
		"content": []map[string]interface{}{
			{"type": "text", "text": string(b)},
		},
	}
}

// textError 构造 MCP 工具失败结果。
func (s *Server) textError(msg string) map[string]interface{} {
	return map[string]interface{}{
		"isError": true,
		"content": []map[string]interface{}{
			{"type": "text", "text": msg},
		},
	}
}

// argUint 从 arguments 中读取 uint 型参数（支持 JSON 数字或字符串形式）。
func (s *Server) argUint(args map[string]interface{}, key string) (uint, error) {
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("缺少参数: %s", key)
	}
	return toUint(v)
}

// toUint 将 JSON 参数值转换为 uint。
func toUint(v interface{}) (uint, error) {
	switch t := v.(type) {
	case float64:
		if t < 0 || t != math.Trunc(t) || t > math.MaxUint32 {
			return 0, fmt.Errorf("参数值非法（需为非负整数）: %v", t)
		}
		return uint(t), nil
	case json.Number:
		n, err := t.Int64()
		if err != nil || n < 0 {
			return 0, fmt.Errorf("参数值非法（需为非负整数）: %v", t)
		}
		return uint(n), nil
	case string:
		n, err := strconv.ParseUint(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("参数值非法（需为非负整数）: %q", t)
		}
		return uint(n), nil
	default:
		return 0, fmt.Errorf("参数类型错误（需为数字或字符串）: %v", v)
	}
}

// toInt 将 JSON 参数值转换为 int（limit 等用途）。
func toInt(v interface{}) (int, error) {
	switch t := v.(type) {
	case float64:
		if t != math.Trunc(t) {
			return 0, fmt.Errorf("参数值非法（需为整数）: %v", t)
		}
		return int(t), nil
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0, fmt.Errorf("参数值非法（需为整数）: %v", t)
		}
		return int(n), nil
	case string:
		n, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("参数值非法（需为整数）: %q", t)
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("参数类型错误（需为数字或字符串）: %v", v)
	}
}
