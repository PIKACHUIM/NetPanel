package meshnode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/netpanel/netpanel/model"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Manager 组网节点管理器
type Manager struct {
	db     *gorm.DB
	log    *logrus.Logger
	ctx    context.Context
	cancel context.CancelFunc
	mu     sync.RWMutex
	client *http.Client
}

// NewManager 创建组网节点管理器
func NewManager(db *gorm.DB, log *logrus.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		db:     db,
		log:    log,
		ctx:    ctx,
		cancel: cancel,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
	return m
}

// Start 启动定期心跳检测
func (m *Manager) Start() {
	go m.heartbeatLoop()
	m.log.Info("[组网节点] 心跳检测已启动")
}

// Stop 停止心跳检测
func (m *Manager) Stop() {
	m.cancel()
	m.log.Info("[组网节点] 心跳检测已停止")
}

// heartbeatLoop 定期检测所有节点的连通性
func (m *Manager) heartbeatLoop() {
	// 启动后立即执行一次
	m.checkAllNodes()

	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAllNodes()
		}
	}
}

// checkAllNodes 检测所有启用的节点
func (m *Manager) checkAllNodes() {
	var nodes []model.MeshNode
	if err := m.db.Where("enable = ?", true).Find(&nodes).Error; err != nil {
		m.log.Errorf("[组网节点] 查询节点列表失败: %v", err)
		return
	}

	var wg sync.WaitGroup
	for i := range nodes {
		wg.Add(1)
		go func(node model.MeshNode) {
			defer wg.Done()
			m.checkNode(&node)
		}(nodes[i])
	}
	wg.Wait()

	// 检测完所有节点后，收集各节点间的连通性
	m.checkPeerLatencies(nodes)
}

// checkNode 检测单个节点的连通性
func (m *Manager) checkNode(node *model.MeshNode) {
	wasOnline := node.IsOnline
	start := time.Now()

	// 尝试调用远程节点的系统信息接口
	_, err := m.callRemoteAPI(node, "GET", "/api/v1/system/info", nil)
	latency := int(time.Since(start).Milliseconds())

	now := time.Now()
	updates := map[string]interface{}{
		"last_heartbeat": now,
	}

	if err != nil {
		updates["is_online"] = false
		updates["latency"] = -1
		m.log.Debugf("[组网节点] 节点 [%d] %s 不可达: %v", node.ID, node.Name, err)

		// 状态变化：在线 → 离线
		if wasOnline {
			m.recordEvent(node.ID, node.Name, "offline", fmt.Sprintf("节点 %s 离线", node.Name))
		}
	} else {
		updates["is_online"] = true
		updates["latency"] = latency

		// 解析节点IP
		if nodeIP := extractIPFromURL(node.URL); nodeIP != "" {
			updates["node_ip"] = nodeIP
		}

		// 状态变化：离线 → 在线
		if !wasOnline {
			m.recordEvent(node.ID, node.Name, "online", fmt.Sprintf("节点 %s 上线（延迟 %dms）", node.Name, latency))
		}
	}

	m.db.Model(&model.MeshNode{}).Where("id = ?", node.ID).Updates(updates)
}

// checkPeerLatencies 检测节点之间的连通性
// 通过调用每个在线节点的 ping 接口来检测它与其他节点之间的延迟
func (m *Manager) checkPeerLatencies(nodes []model.MeshNode) {
	onlineNodes := make([]model.MeshNode, 0)
	for _, n := range nodes {
		// 重新从数据库读取最新状态
		var node model.MeshNode
		if err := m.db.First(&node, n.ID).Error; err == nil && node.IsOnline {
			onlineNodes = append(onlineNodes, node)
		}
	}

	if len(onlineNodes) < 2 {
		return
	}

	// 对每个在线节点，让它 ping 其他所有节点
	var wg sync.WaitGroup
	for i := range onlineNodes {
		wg.Add(1)
		go func(source model.MeshNode) {
			defer wg.Done()
			peerLatencies := make(map[string]int)

			for _, target := range onlineNodes {
				if target.ID == source.ID {
					continue
				}
				// 让 source 节点 ping target 节点
				latency := m.pingFromNode(&source, target.URL)
				peerLatencies[fmt.Sprintf("%d", target.ID)] = latency
			}

			// 保存连通性数据
			data, _ := json.Marshal(peerLatencies)
			m.db.Model(&model.MeshNode{}).Where("id = ?", source.ID).Update("peer_latencies", string(data))
		}(onlineNodes[i])
	}
	wg.Wait()
}

// pingFromNode 从指定节点 ping 目标URL，返回延迟（毫秒），-1 表示不可达
func (m *Manager) pingFromNode(source *model.MeshNode, targetURL string) int {
	// 调用源节点的 ping 接口
	body := map[string]string{"target_url": targetURL}
	data, _ := json.Marshal(body)

	resp, err := m.callRemoteAPI(source, "POST", "/api/v1/mesh/ping", data)
	if err != nil {
		return -1
	}

	var result struct {
		Code int `json:"code"`
		Data struct {
			Latency int `json:"latency"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &result); err != nil || result.Code != 200 {
		return -1
	}
	return result.Data.Latency
}

// callRemoteAPI 调用远程节点的API
func (m *Manager) callRemoteAPI(node *model.MeshNode, method, path string, body []byte) ([]byte, error) {
	apiURL := strings.TrimRight(node.URL, "/") + path

	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequestWithContext(m.ctx, method, apiURL, bytes.NewReader(body))
	} else {
		req, err = http.NewRequestWithContext(m.ctx, method, apiURL, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 使用节点的管理员凭证获取 token
	token, err := m.getNodeToken(node)
	if err != nil {
		return nil, fmt.Errorf("获取节点 token 失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// nodeTokenCache 节点 token 缓存
var nodeTokenCache sync.Map // map[uint]tokenEntry

type tokenEntry struct {
	Token     string
	ExpiresAt time.Time
}

// getNodeToken 获取远程节点的认证 token（带缓存）
func (m *Manager) getNodeToken(node *model.MeshNode) (string, error) {
	// 检查缓存
	if entry, ok := nodeTokenCache.Load(node.ID); ok {
		te := entry.(tokenEntry)
		if time.Now().Before(te.ExpiresAt) {
			return te.Token, nil
		}
	}

	// 登录获取 token
	loginURL := strings.TrimRight(node.URL, "/") + "/api/v1/auth/login"
	loginBody, _ := json.Marshal(map[string]string{
		"username": node.AdminUser,
		"password": node.AdminPassword.String(),
	})

	req, err := http.NewRequestWithContext(m.ctx, "POST", loginURL, bytes.NewReader(loginBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析登录响应失败: %w", err)
	}
	if result.Code != 200 || result.Data.Token == "" {
		return "", fmt.Errorf("登录失败: %s", result.Message)
	}

	// 缓存 token（有效期 20 分钟）
	nodeTokenCache.Store(node.ID, tokenEntry{
		Token:     result.Data.Token,
		ExpiresAt: time.Now().Add(20 * time.Minute),
	})

	return result.Data.Token, nil
}

// ProxyRequest 代理请求到远程节点
func (m *Manager) ProxyRequest(nodeID uint, method, path string, body []byte) ([]byte, error) {
	var node model.MeshNode
	if err := m.db.First(&node, nodeID).Error; err != nil {
		return nil, fmt.Errorf("节点不存在: %w", err)
	}
	if !node.Enable {
		return nil, fmt.Errorf("节点已禁用")
	}
	return m.callRemoteAPI(&node, method, path, body)
}

// GetTopology 获取拓扑关系数据
// 返回节点列表和边（隧道连接关系）
func (m *Manager) GetTopology() (map[string]interface{}, error) {
	// 获取所有节点（包括本机）
	var nodes []model.MeshNode
	if err := m.db.Find(&nodes).Error; err != nil {
		return nil, err
	}

	// 获取本机的隧道数据
	localTunnels := m.getLocalTunnels()

	// 获取远程节点的隧道数据
	allTunnels := make(map[uint][]tunnelInfo)
	allTunnels[0] = localTunnels // 0 表示本机

	for _, node := range nodes {
		if !node.IsOnline {
			continue
		}
		tunnels := m.getRemoteTunnels(&node)
		allTunnels[node.ID] = tunnels
	}

	// 计算边（连接关系）
	edges := m.calculateEdges(nodes, allTunnels)

	// 构建节点信息（包含隧道统计）
	nodeInfos := m.buildNodeInfos(nodes, allTunnels)

	return map[string]interface{}{
		"nodes": nodeInfos,
		"edges": edges,
	}, nil
}

// tunnelInfo 隧道信息
type tunnelInfo struct {
	Type        string `json:"type"`         // frpc/frps/nps_client/nps_server/easytier_client/easytier_server/port_forward/stun
	Name        string `json:"name"`
	ID          uint   `json:"id"`
	ServerAddr  string `json:"server_addr"`  // 连接的服务器地址
	ServerPort  int    `json:"server_port"`  // 连接的服务器端口
	ListenPort  int    `json:"listen_port"`  // 监听端口
	NetworkName string `json:"network_name"` // EasyTier 网络名称
	PeerAddrs   string `json:"peer_addrs"`   // EasyTier 对端地址
	Status      string `json:"status"`
}

// edgeInfo 边信息
type edgeInfo struct {
	Source       uint   `json:"source"`        // 源节点ID（0=本机）
	Target       uint   `json:"target"`        // 目标节点ID（0=本机）
	TunnelType   string `json:"tunnel_type"`   // frp/nps/easytier
	TunnelName   string `json:"tunnel_name"`   // 隧道名称
	Direction    string `json:"direction"`     // unidirectional/bidirectional/p2p
	SourceLabel  string `json:"source_label"`  // 源端标签（如"客户端"）
	TargetLabel  string `json:"target_label"`  // 目标端标签（如"服务端"）
}

// getLocalTunnels 获取本机的隧道数据
func (m *Manager) getLocalTunnels() []tunnelInfo {
	var tunnels []tunnelInfo

	// FRP 客户端
	var frpcs []model.FrpcConfig
	m.db.Find(&frpcs)
	for _, f := range frpcs {
		tunnels = append(tunnels, tunnelInfo{
			Type:       "frpc",
			Name:       f.Name,
			ID:         f.ID,
			ServerAddr: f.ServerAddr,
			ServerPort: f.ServerPort,
			Status:     f.Status,
		})
	}

	// FRP 服务端
	var frpss []model.FrpsConfig
	m.db.Find(&frpss)
	for _, f := range frpss {
		tunnels = append(tunnels, tunnelInfo{
			Type:       "frps",
			Name:       f.Name,
			ID:         f.ID,
			ListenPort: f.BindPort,
			Status:     f.Status,
		})
	}

	// NPS 客户端
	var npscs []model.NpsClientConfig
	m.db.Find(&npscs)
	for _, n := range npscs {
		tunnels = append(tunnels, tunnelInfo{
			Type:       "nps_client",
			Name:       n.Name,
			ID:         n.ID,
			ServerAddr: n.ServerAddr,
			ServerPort: n.ServerPort,
			Status:     n.Status,
		})
	}

	// NPS 服务端
	var npss []model.NpsServerConfig
	m.db.Find(&npss)
	for _, n := range npss {
		tunnels = append(tunnels, tunnelInfo{
			Type:       "nps_server",
			Name:       n.Name,
			ID:         n.ID,
			ListenPort: n.BridgePort,
			Status:     n.Status,
		})
	}

	// EasyTier 客户端
	var etcs []model.EasytierClient
	m.db.Find(&etcs)
	for _, e := range etcs {
		tunnels = append(tunnels, tunnelInfo{
			Type:        "easytier_client",
			Name:        e.Name,
			ID:          e.ID,
			NetworkName: e.NetworkName,
			PeerAddrs:   e.ServerAddr,
			Status:      e.Status,
		})
	}

	// EasyTier 服务端
	var etss []model.EasytierServer
	m.db.Find(&etss)
	for _, e := range etss {
		tunnels = append(tunnels, tunnelInfo{
			Type:        "easytier_server",
			Name:        e.Name,
			ID:          e.ID,
			NetworkName: e.NetworkName,
			ListenPort:  0, // 从 ListenPorts 解析
			Status:      e.Status,
		})
	}

	// 端口转发
	var pfs []model.PortForwardRule
	m.db.Find(&pfs)
	for _, p := range pfs {
		tunnels = append(tunnels, tunnelInfo{
			Type:       "port_forward",
			Name:       p.Name,
			ID:         p.ID,
			ListenPort: p.ListenPort,
			Status:     p.Status,
		})
	}

	// STUN
	var stuns []model.StunRule
	m.db.Find(&stuns)
	for _, s := range stuns {
		tunnels = append(tunnels, tunnelInfo{
			Type:       "stun",
			Name:       s.Name,
			ID:         s.ID,
			ListenPort: s.ListenPort,
			Status:     s.Status,
		})
	}

	return tunnels
}

// getRemoteTunnels 获取远程节点的隧道数据
func (m *Manager) getRemoteTunnels(node *model.MeshNode) []tunnelInfo {
	var tunnels []tunnelInfo

	// 获取各类隧道数据
	types := []struct {
		path string
		typ  string
	}{
		{"/api/v1/frpc", "frpc"},
		{"/api/v1/frps", "frps"},
		{"/api/v1/nps/client", "nps_client"},
		{"/api/v1/nps/server", "nps_server"},
		{"/api/v1/easytier/client", "easytier_client"},
		{"/api/v1/easytier/server", "easytier_server"},
		{"/api/v1/port-forward", "port_forward"},
		{"/api/v1/stun", "stun"},
	}

	for _, t := range types {
		resp, err := m.callRemoteAPI(node, "GET", t.path, nil)
		if err != nil {
			continue
		}
		var result struct {
			Code int               `json:"code"`
			Data []json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(resp, &result); err != nil || result.Code != 200 {
			continue
		}

		for _, raw := range result.Data {
			ti := parseTunnelFromJSON(t.typ, raw)
			if ti != nil {
				tunnels = append(tunnels, *ti)
			}
		}
	}

	return tunnels
}

// parseTunnelFromJSON 从JSON解析隧道信息
func parseTunnelFromJSON(typ string, raw json.RawMessage) *tunnelInfo {
	var generic map[string]interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}

	ti := &tunnelInfo{Type: typ}

	if v, ok := generic["name"].(string); ok {
		ti.Name = v
	}
	if v, ok := generic["id"].(float64); ok {
		ti.ID = uint(v)
	}
	if v, ok := generic["status"].(string); ok {
		ti.Status = v
	}

	switch typ {
	case "frpc":
		if v, ok := generic["server_addr"].(string); ok {
			ti.ServerAddr = v
		}
		if v, ok := generic["server_port"].(float64); ok {
			ti.ServerPort = int(v)
		}
	case "frps":
		if v, ok := generic["bind_port"].(float64); ok {
			ti.ListenPort = int(v)
		}
	case "nps_client":
		if v, ok := generic["server_addr"].(string); ok {
			ti.ServerAddr = v
		}
		if v, ok := generic["server_port"].(float64); ok {
			ti.ServerPort = int(v)
		}
	case "nps_server":
		if v, ok := generic["bridge_port"].(float64); ok {
			ti.ListenPort = int(v)
		}
	case "easytier_client":
		if v, ok := generic["network_name"].(string); ok {
			ti.NetworkName = v
		}
		if v, ok := generic["peers"].(string); ok {
			ti.PeerAddrs = v
		}
	case "easytier_server":
		if v, ok := generic["network_name"].(string); ok {
			ti.NetworkName = v
		}
	case "port_forward":
		if v, ok := generic["listen_port"].(float64); ok {
			ti.ListenPort = int(v)
		}
	case "stun":
		if v, ok := generic["listen_port"].(float64); ok {
			ti.ListenPort = int(v)
		}
	}

	return ti
}

// calculateEdges 计算节点之间的连接关系
func (m *Manager) calculateEdges(nodes []model.MeshNode, allTunnels map[uint][]tunnelInfo) []edgeInfo {
	var edges []edgeInfo

	// 构建节点IP到ID的映射
	nodeIPMap := make(map[string]uint) // IP -> nodeID
	for _, n := range nodes {
		if n.NodeIP != "" {
			nodeIPMap[n.NodeIP] = n.ID
		}
		// 也从URL中提取IP
		if ip := extractIPFromURL(n.URL); ip != "" {
			nodeIPMap[ip] = n.ID
		}
	}

	// 构建节点监听端口映射：nodeID -> {port -> tunnelType}
	nodeListenPorts := make(map[uint]map[int]string)
	for nodeID, tunnels := range allTunnels {
		ports := make(map[int]string)
		for _, t := range tunnels {
			if t.ListenPort > 0 {
				ports[t.ListenPort] = t.Type
			}
		}
		nodeListenPorts[nodeID] = ports
	}

	// 构建 EasyTier 网络名称映射：networkName -> []nodeID
	etNetworks := make(map[string][]uint)
	for nodeID, tunnels := range allTunnels {
		for _, t := range tunnels {
			if (t.Type == "easytier_client" || t.Type == "easytier_server") && t.NetworkName != "" {
				etNetworks[t.NetworkName] = append(etNetworks[t.NetworkName], nodeID)
			}
		}
	}

	edgeSet := make(map[string]bool) // 去重

	// 1. FRP 连接：客户端 → 服务端（单向）
	for sourceID, tunnels := range allTunnels {
		for _, t := range tunnels {
			if t.Type != "frpc" || t.ServerAddr == "" {
				continue
			}
			// 查找服务器IP对应的节点
			serverIP := resolveHost(t.ServerAddr)
			if targetID, ok := nodeIPMap[serverIP]; ok {
				key := fmt.Sprintf("frp_%d_%d", sourceID, targetID)
				if !edgeSet[key] {
					edgeSet[key] = true
					edges = append(edges, edgeInfo{
						Source:      sourceID,
						Target:      targetID,
						TunnelType:  "frp",
						TunnelName:  t.Name,
						Direction:   "unidirectional",
						SourceLabel: "FRP客户端",
						TargetLabel: "FRP服务端",
					})
				}
			}
		}
	}

	// 2. NPS 连接：客户端 ↔ 服务端（双向）
	for sourceID, tunnels := range allTunnels {
		for _, t := range tunnels {
			if t.Type != "nps_client" || t.ServerAddr == "" {
				continue
			}
			serverIP := resolveHost(t.ServerAddr)
			if targetID, ok := nodeIPMap[serverIP]; ok {
				key := fmt.Sprintf("nps_%d_%d", min(sourceID, targetID), max(sourceID, targetID))
				if !edgeSet[key] {
					edgeSet[key] = true
					edges = append(edges, edgeInfo{
						Source:      sourceID,
						Target:      targetID,
						TunnelType:  "nps",
						TunnelName:  t.Name,
						Direction:   "bidirectional",
						SourceLabel: "NPS客户端",
						TargetLabel: "NPS服务端",
					})
				}
			}
		}
	}

	// 3. EasyTier 连接
	// 3a. 同一网络名称 → P2P 互连
	for netName, nodeIDs := range etNetworks {
		uniqueIDs := uniqueUints(nodeIDs)
		for i := 0; i < len(uniqueIDs); i++ {
			for j := i + 1; j < len(uniqueIDs); j++ {
				key := fmt.Sprintf("et_p2p_%d_%d_%s", uniqueIDs[i], uniqueIDs[j], netName)
				if !edgeSet[key] {
					edgeSet[key] = true
					edges = append(edges, edgeInfo{
						Source:      uniqueIDs[i],
						Target:      uniqueIDs[j],
						TunnelType:  "easytier",
						TunnelName:  netName,
						Direction:   "p2p",
						SourceLabel: "ET节点",
						TargetLabel: "ET节点",
					})
				}
			}
		}
	}

	// 3b. EasyTier 直连对端IP
	for sourceID, tunnels := range allTunnels {
		for _, t := range tunnels {
			if t.Type != "easytier_client" || t.PeerAddrs == "" {
				continue
			}
			// 解析 peers 字段（逗号分隔的 addr:port）
			peers := strings.Split(t.PeerAddrs, ",")
			for _, peer := range peers {
				peer = strings.TrimSpace(peer)
				if peer == "" {
					continue
				}
				peerIP := extractIPFromAddr(peer)
				if targetID, ok := nodeIPMap[peerIP]; ok && targetID != sourceID {
					key := fmt.Sprintf("et_direct_%d_%d", min(sourceID, targetID), max(sourceID, targetID))
					if !edgeSet[key] {
						edgeSet[key] = true
						edges = append(edges, edgeInfo{
							Source:      sourceID,
							Target:      targetID,
							TunnelType:  "easytier",
							TunnelName:  t.Name,
							Direction:   "bidirectional",
							SourceLabel: "ET直连",
							TargetLabel: "ET直连",
						})
					}
				}
			}
		}
	}

	return edges
}

// buildNodeInfos 构建节点信息（包含隧道统计）
func (m *Manager) buildNodeInfos(nodes []model.MeshNode, allTunnels map[uint][]tunnelInfo) []map[string]interface{} {
	var infos []map[string]interface{}

	// 本机节点
	localTunnels := allTunnels[0]
	localInfo := map[string]interface{}{
		"id":        0,
		"name":      "本机",
		"is_local":  true,
		"is_online": true,
		"node_ip":   "127.0.0.1",
		"tunnels":   buildTunnelStats(localTunnels),
	}
	infos = append(infos, localInfo)

	// 远程节点
	for _, node := range nodes {
		tunnels := allTunnels[node.ID]
		info := map[string]interface{}{
			"id":             node.ID,
			"name":           node.Name,
			"is_local":       false,
			"is_online":      node.IsOnline,
			"node_ip":        node.NodeIP,
			"latency":        node.Latency,
			"peer_latencies": node.PeerLatencies,
			"tunnels":        buildTunnelStats(tunnels),
		}
		infos = append(infos, info)
	}

	return infos
}

// buildTunnelStats 构建隧道统计
func buildTunnelStats(tunnels []tunnelInfo) map[string]interface{} {
	stats := map[string]interface{}{
		"total":           len(tunnels),
		"port_forward":    0,
		"stun":            0,
		"frp":             0,
		"nps":             0,
		"easytier":        0,
		"details":         tunnels,
	}

	for _, t := range tunnels {
		switch {
		case t.Type == "port_forward":
			stats["port_forward"] = stats["port_forward"].(int) + 1
		case t.Type == "stun":
			stats["stun"] = stats["stun"].(int) + 1
		case strings.HasPrefix(t.Type, "frp"):
			stats["frp"] = stats["frp"].(int) + 1
		case strings.HasPrefix(t.Type, "nps"):
			stats["nps"] = stats["nps"].(int) + 1
		case strings.HasPrefix(t.Type, "easytier"):
			stats["easytier"] = stats["easytier"].(int) + 1
		}
	}

	return stats
}

// recordEvent 记录节点事件
func (m *Manager) recordEvent(nodeID uint, nodeName, eventType, message string) {
	event := model.MeshNodeEvent{
		NodeID:    nodeID,
		NodeName:  nodeName,
		EventType: eventType,
		Message:   message,
		EventTime: time.Now(),
	}
	if err := m.db.Create(&event).Error; err != nil {
		m.log.Errorf("[组网节点] 记录事件失败: %v", err)
	}
}

// RecordEvent 公开的记录事件方法（供 handler 调用）
func (m *Manager) RecordEvent(nodeID uint, nodeName, eventType, message string) {
	m.recordEvent(nodeID, nodeName, eventType, message)
}

// PingTarget 从本机 ping 目标URL，返回延迟（毫秒）
func (m *Manager) PingTarget(targetURL string) (int, error) {
	start := time.Now()
	pingURL := strings.TrimRight(targetURL, "/") + "/api/v1/system/info"

	req, err := http.NewRequestWithContext(m.ctx, "GET", pingURL, nil)
	if err != nil {
		return -1, err
	}

	resp, err := m.client.Do(req)
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	latency := int(time.Since(start).Milliseconds())
	return latency, nil
}

// ===== 工具函数 =====

// extractIPFromURL 从URL中提取IP地址
func extractIPFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	// 如果是IP地址直接返回
	if net.ParseIP(host) != nil {
		return host
	}
	// 尝试DNS解析
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return host
	}
	return ips[0].String()
}

// extractIPFromAddr 从 addr:port 格式中提取IP
func extractIPFromAddr(addr string) string {
	// 处理可能的协议前缀
	if idx := strings.Index(addr, "://"); idx >= 0 {
		addr = addr[idx+3:]
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if net.ParseIP(host) != nil {
		return host
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return host
	}
	return ips[0].String()
}

// resolveHost 解析主机名为IP
func resolveHost(host string) string {
	if net.ParseIP(host) != nil {
		return host
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return host
	}
	return ips[0].String()
}

// uniqueUints 去重
func uniqueUints(ids []uint) []uint {
	seen := make(map[uint]bool)
	var result []uint
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			result = append(result, id)
		}
	}
	return result
}

func min(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint) uint {
	if a > b {
		return a
	}
	return b
}
