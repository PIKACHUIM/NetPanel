// Package tunservice 穿透服务层（用户视角）：
// 把 frp / nps / easytier / wireguard / cftunnel 等工具的穿透实例聚合为
// 「服务」——一个服务（目标 IP:端口）由多条线路（Line）提供访问，
// 支持统一状态查看与一键启停。线路关联关系由服务的 LineRefs 指定。
package tunservice

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"github.com/netpanel/netpanel/model"
	"github.com/netpanel/netpanel/service/cftunnel"
	"github.com/netpanel/netpanel/service/easytier"
	"github.com/netpanel/netpanel/service/frp"
	"github.com/netpanel/netpanel/service/linereg"
	"github.com/netpanel/netpanel/service/nps"
	"github.com/netpanel/netpanel/service/selector"
	"github.com/netpanel/netpanel/service/wireguard"
)

// LineInfo 服务下的一条线路（含实时状态）
type LineInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tool    string `json:"tool"`
	Address string `json:"address"`
	Layer   string `json:"layer"` // port / domain
	Status  string `json:"status"`
	// Latency 最近一次测得的延迟（ns），未探测为 0
	Latency int64 `json:"latency"`
}

// ServiceView 服务及其线路视图
type ServiceView struct {
	model.TunService
	Lines []LineInfo `json:"lines"`
}

// Manager 穿透服务管理器
type Manager struct {
	db           *gorm.DB
	log          *logrus.Logger
	linereg      *linereg.Manager
	frpMgr       *frp.Manager
	npsMgr       *nps.Manager
	easytierMgr  *easytier.Manager
	wireguardMgr *wireguard.Manager
	cftunnelMgr  *cftunnel.Manager
}

// NewManager 创建穿透服务管理器。
func NewManager(db *gorm.DB, log *logrus.Logger, lineregMgr *linereg.Manager,
	frpMgr *frp.Manager, npsMgr *nps.Manager, easytierMgr *easytier.Manager,
	wireguardMgr *wireguard.Manager, cftunnelMgr *cftunnel.Manager) *Manager {
	return &Manager{
		db:           db,
		log:          log,
		linereg:      lineregMgr,
		frpMgr:       frpMgr,
		npsMgr:       npsMgr,
		easytierMgr:  easytierMgr,
		wireguardMgr: wireguardMgr,
		cftunnelMgr:  cftunnelMgr,
	}
}

// List 返回全部穿透服务及其线路。
func (m *Manager) List() ([]ServiceView, error) {
	var services []model.TunService
	if err := m.db.Order("id desc").Find(&services).Error; err != nil {
		return nil, err
	}
	views := make([]ServiceView, 0, len(services))
	for i := range services {
		views = append(views, m.buildView(&services[i]))
	}
	return views, nil
}

// Get 返回单个服务的线路视图。
func (m *Manager) Get(id uint) (*ServiceView, error) {
	var svc model.TunService
	if err := m.db.First(&svc, id).Error; err != nil {
		return nil, err
	}
	v := m.buildView(&svc)
	return &v, nil
}

// Start 启动服务关联的全部线路。
func (m *Manager) Start(id uint) error {
	svc, err := m.Get(id)
	if err != nil {
		return err
	}
	var errs []string
	for _, line := range svc.Lines {
		if err := m.startLine(line.ID); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", line.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("部分线路启动失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

// Stop 停止服务关联的全部线路。
func (m *Manager) Stop(id uint) {
	if svc, err := m.Get(id); err == nil {
		for _, line := range svc.Lines {
			m.stopLine(line.ID)
		}
	}
}

// RebindPort 端口层切换落地：把服务关联的穿透规则重建到当前选中线路。
//
// 语义（先停后起，短暂抖动可接受）：
//   - 停掉服务下「非选中线路」的运行中客户端，使端口映射只跟随当前线路；
//   - 确保选中线路的客户端已运行（未运行时启动，运行中保持，避免无谓重启）。
//
// 仅服务处于启用状态时生效；由 linereg 在选线变化时调用。
func (m *Manager) RebindPort(svcID uint, lineID string) error {
	var svc model.TunService
	if err := m.db.First(&svc, svcID).Error; err != nil {
		return err
	}
	if !svc.Enable {
		return nil
	}
	var refs []string
	if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err != nil {
		return nil
	}
	selected := false
	for _, ref := range refs {
		if ref == lineID {
			selected = true
			break
		}
	}
	if !selected {
		return nil
	}
	// 1) 停掉非选中线路的运行中客户端
	for _, ref := range refs {
		if ref != lineID && m.toolStatus(ref) == "running" {
			m.stopLine(ref)
			m.log.Infof("[穿透服务][%d] 停止非选中线路客户端 %s", svcID, ref)
		}
	}
	// 2) 确保选中线路客户端运行（未运行才启动，避免每轮刷新无谓重启）
	if m.toolStatus(lineID) == "running" {
		return nil
	}
	if err := m.startLine(lineID); err != nil {
		m.log.Errorf("[穿透服务][%d] 启动选中线路 %s 失败: %v", svcID, lineID, err)
		return err
	}
	m.log.Infof("[穿透服务][%d] 端口层已重绑到线路 %s", svcID, lineID)
	return nil
}

// History 返回服务关联线路的探测历史（按线路分组，每条线路取最近 limit 条）。
func (m *Manager) History(id uint, limit int) (map[string][]model.ProbeHistory, error) {
	svc, err := m.Get(id)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	result := make(map[string][]model.ProbeHistory, len(svc.Lines))
	for _, line := range svc.Lines {
		var history []model.ProbeHistory
		if err := m.db.Where("line_id = ?", line.ID).
			Order("id desc").Limit(limit).Find(&history).Error; err != nil {
			continue
		}
		// 时间正序返回（趋势图从左到右）
		for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
			history[i], history[j] = history[j], history[i]
		}
		result[line.ID] = history
	}
	return result, nil
}

// buildView 组装服务视图：解析 LineRefs，逐条取线路信息并聚合状态。
func (m *Manager) buildView(s *model.TunService) ServiceView {
	v := ServiceView{TunService: *s, Lines: []LineInfo{}}
	var refs []string
	if err := json.Unmarshal([]byte(s.LineRefs), &refs); err != nil {
		refs = nil
	}
	hasRunning := false
	hasError := false
	for _, ref := range refs {
		info := m.lineInfo(ref)
		v.Lines = append(v.Lines, info)
		switch info.Status {
		case "running":
			hasRunning = true
		case "error":
			hasError = true
		}
	}
	// 聚合状态：任一线路运行中 -> running；否则有错误 -> error；其余 stopped
	switch {
	case hasRunning:
		v.Status = "running"
	case hasError:
		v.Status = "error"
	case len(refs) > 0:
		v.Status = "stopped"
	default:
		v.Status = "stopped"
	}
	return v
}

// lineInfo 根据线路 ID 从 selector/linereg 与各工具管理器取实时信息。
func (m *Manager) lineInfo(lineID string) LineInfo {
	info := LineInfo{ID: lineID, Status: "stopped"}
	if m.linereg == nil {
		return info
	}
	// 从 selector 取静态信息（地址/工具/名称）与最近延迟
	st := m.linereg.Selector().Snapshot()
	if st.Current != "" {
		// 无特殊处理：Snapshot 的 Lines 已包含全部线路
	}
	for _, l := range st.Lines {
		if l.ID == lineID {
			info.Name = l.Name
			info.Tool = l.Tool
			info.Address = l.Address
			info.Layer = layerFor(l)
			if r, ok := st.Results[lineID]; ok && r.Err == nil {
				info.Latency = int64(latencyOf(r))
			}
			break
		}
	}
	info.Status = m.toolStatus(lineID)
	return info
}

// layerFor 按工具判定线路层次：cftunnel 为域名层，其余为端口层。
func layerFor(l selector.Line) string {
	if l.Tool == "cloudflare" {
		return "domain"
	}
	return "port"
}

// latencyOf 提取线路的有效延迟（HTTP 优先，否则 TCP）。
func latencyOf(r selector.ProbeResult) int64 {
	if r.HTTPLatency > 0 {
		return int64(r.HTTPLatency)
	}
	return int64(r.TCPLatency)
}

// SpeedtestLine 一次即时测速的单条线路结果。
type SpeedtestLine struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tool    string `json:"tool"`
	Address string `json:"address"`
	Layer   string `json:"layer"`
	// Latency 有效延迟（ns）；探测失败为 0。
	Latency int64  `json:"latency"`
	// Error 非空表示本次探测失败（超时/拒绝等）。
	Error string `json:"error,omitempty"`
}

// Speedtest 对服务的关联线路做一次即时并发测速（不刷新内部选线状态）。
// 返回按有效延迟升序排列的结果，失败线路排最后。供「测速弹窗」等
// 用户主动触发的一次性测速使用。
func (m *Manager) Speedtest(svcID uint) ([]SpeedtestLine, error) {
	var svc model.TunService
	if err := m.db.First(&svc, svcID).Error; err != nil {
		return nil, err
	}
	var refs []string
	if err := json.Unmarshal([]byte(svc.LineRefs), &refs); err != nil {
		refs = nil
	}
	// 从 selector 收集线路静态信息（地址/工具/名称/层次）
	st := m.linereg.Selector().Snapshot()
	byID := make(map[string]selector.Line, len(st.Lines))
	for _, l := range st.Lines {
		byID[l.ID] = l
	}
	var lines []selector.Line
	for _, ref := range refs {
		if l, ok := byID[ref]; ok {
			lines = append(lines, l)
		}
	}
	// 即时并发测速：不刷新内部状态（ProbeLines 语义）
	results := m.linereg.Selector().ProbeLines(context.Background(), lines)

	out := make([]SpeedtestLine, 0, len(refs))
	for _, ref := range refs {
		item := SpeedtestLine{ID: ref}
		if l, ok := byID[ref]; ok {
			item.Name = l.Name
			item.Tool = l.Tool
			item.Address = l.Address
			item.Layer = layerFor(l)
		}
		if r, ok := results[ref]; ok && r.Err == nil {
			item.Latency = latencyOf(r)
		} else if ok && r.Err != nil {
			item.Error = r.Err.Error()
		} else {
			item.Error = "未探测"
		}
		out = append(out, item)
	}
	// 延迟升序，失败（Error 非空）排最后
	sort.SliceStable(out, func(i, j int) bool {
		ei, ej := out[i].Error != "", out[j].Error != ""
		if ei != ej {
			return !ei
		}
		return out[i].Latency < out[j].Latency
	})
	return out, nil
}

// parseLineID 解析线路 ID："frp:3" -> ("frp", 3)；"easytier:1:0" -> ("easytier", 1)。
func parseLineID(lineID string) (tool string, id uint, ok bool) {
	parts := strings.Split(lineID, ":")
	if len(parts) < 2 {
		return "", 0, false
	}
	tool = parts[0]
	n, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return "", 0, false
	}
	return tool, uint(n), true
}

// toolStatus 查询线路所属工具的运行状态。
func (m *Manager) toolStatus(lineID string) string {
	tool, id, ok := parseLineID(lineID)
	if !ok {
		return "stopped"
	}
	switch tool {
	case "frp":
		if m.frpMgr != nil {
			return m.frpMgr.GetClientStatus(id)
		}
	case "nps":
		if m.npsMgr != nil {
			return m.npsMgr.GetClientStatus(id)
		}
	case "easytier":
		if m.easytierMgr != nil {
			return m.easytierMgr.GetClientStatus(id)
		}
	case "wg":
		if m.wireguardMgr != nil {
			return m.wireguardMgr.GetStatus(id)
		}
	case "cftunnel":
		if m.cftunnelMgr != nil {
			return m.cftunnelMgr.GetStatus(id)
		}
	}
	return "stopped"
}

// startLine 按工具调用对应管理器的启动方法。
func (m *Manager) startLine(lineID string) error {
	tool, id, ok := parseLineID(lineID)
	if !ok {
		return fmt.Errorf("非法线路 ID: %q", lineID)
	}
	switch tool {
	case "frp":
		return m.frpMgr.StartClient(id)
	case "nps":
		return m.npsMgr.StartClient(id)
	case "easytier":
		return m.easytierMgr.StartClient(id)
	case "wg":
		return m.wireguardMgr.Start(id)
	case "cftunnel":
		return m.cftunnelMgr.Start(id)
	}
	return fmt.Errorf("未知工具: %q", tool)
}

// stopLine 按工具调用对应管理器的停止方法。
func (m *Manager) stopLine(lineID string) {
	tool, id, ok := parseLineID(lineID)
	if !ok {
		return
	}
	switch tool {
	case "frp":
		m.frpMgr.StopClient(id)
	case "nps":
		m.npsMgr.StopClient(id)
	case "easytier":
		m.easytierMgr.StopClient(id)
	case "wg":
		m.wireguardMgr.Stop(id)
	case "cftunnel":
		m.cftunnelMgr.Stop(id)
	}
}
