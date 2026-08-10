// Package selector 实现「多工具穿透线路统一抽象 + 自动测速选线」。
//
// NetPanel 内置 frp / nps / easytier / wireguard 等多个穿透工具，各自维护
// 自己的 Manager。本包提供线路层的公共抽象：任何工具只要把它的可用线路
// 描述为 []Line（地址 + 可选 HTTP 204 探测地址），就能接入统一的
// 并发测速、自动选线（容差防抖）、手动锁线与后台守护。
//
// 选线语义参考 Clash 的 url-test：按延迟排序取最快，tolerance 内不切换
// 避免抖动；锁线时完全跳过自动切换。
package selector

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Line 一条可选的穿透线路（多工具适配的统一描述）。
type Line struct {
	// ID 线路唯一标识（如 "frp:1" / "nps:2"）。
	ID string
	// Name 展示名（如 "阿里云 frps"）。
	Name string
	// Tool 来源工具（"frp" / "nps" / "easytier" / "wireguard" / "cloudflare" ...）。
	Tool string
	// Layer 线路层次："" / "port"（端口层，TCP 握手探测，默认）或
	// "domain"（域名层，HTTPS 204 探测，如 Cloudflare Tunnel）。
	// 域名层线路未配置 ProbeURL 时，探测会自动使用 https://<host>。
	Layer string
	// Address 探测地址（host:port）。做 TCP 握手延迟测量。
	Address string
	// ProbeURL 可选。非空时额外做一次 HTTP 204 探测（用于区分「能连上」与
	// 「能正常出网」）。
	ProbeURL string
}

// ProbeResult 单条线路的一次测速结果。
type ProbeResult struct {
	LineID string
	// TCPLatency TCP 握手延迟。
	TCPLatency time.Duration
	// HTTPLatency HTTP 204 探测延迟；未配置 ProbeURL 时为 0。
	HTTPLatency time.Duration
	// Err 非空表示该线路本次不可用（超时/拒绝/握手失败）。
	Err error
}

// ProbeError 标记线路不可用，带人类可读原因。
type ProbeError struct {
	Reason string
}

func (e *ProbeError) Error() string { return e.Reason }

// Prober 单条线路的探测器。测试可注入假实现，避免真实网络依赖。
type Prober interface {
	Probe(ctx context.Context, line Line) ProbeResult
}

// TCPProber 默认实现：TCP 握手 + 可选 HTTP 204 探测。
type TCPProber struct {
	// Timeout 单条线路探测总超时（默认 3s）。
	Timeout time.Duration
	// VerifyTLS 为 true 时校验证书；默认 false 跳过证书校验——穿透入口
	// 常是 IP:PORT 直连，证书与 SNI 均不可信，探测只关心可达性。
	VerifyTLS bool

	// once/transport 复用同一个 http.Transport，避免每次探测都新建
	// 连接池（高频探测时开销显著）。
	once      sync.Once
	transport *http.Transport
}

// transportFor 惰性初始化并复用 Transport（并发安全，http.Transport
// 内部自带连接池与并发保护）。
func (p *TCPProber) transportFor() *http.Transport {
	p.once.Do(func() {
		p.transport = &http.Transport{
			// 穿透入口常是 IP:PORT 直连，证书与 SNI 均不可信；仅探测可达性，
			// 默认跳过证书校验（VerifyTLS 为 true 时校验证书）。
			TLSClientConfig: &tls.Config{InsecureSkipVerify: !p.VerifyTLS},
		}
	})
	return p.transport
}

// Probe 并发安全，单次调用阻塞至探测完成。
func (p *TCPProber) Probe(ctx context.Context, line Line) ProbeResult {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", line.Address)
	if err != nil {
		return ProbeResult{LineID: line.ID, Err: &ProbeError{Reason: err.Error()}}
	}
	tcpLatency := time.Since(start)
	_ = conn.Close()

	res := ProbeResult{LineID: line.ID, TCPLatency: tcpLatency}
	probeURL := line.ProbeURL
	// 域名层线路未配置探测 URL 时，自动使用 https://<host> 做 HTTPS 204 探测
	// （域名层入口通常是 HTTPS，如 Cloudflare Tunnel 的 cfargotunnel.com）。
	if probeURL == "" && line.Layer == "domain" {
		if host := domainHost(line.Address); host != "" {
			probeURL = "https://" + host
		}
	}
	if probeURL == "" {
		return res
	}

	httpStart := time.Now()
	client := &http.Client{Timeout: timeout, Transport: p.transportFor()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
	if err != nil {
		res.Err = &ProbeError{Reason: err.Error()}
		return res
	}
	resp, err := client.Do(req)
	if err != nil {
		res.Err = &ProbeError{Reason: err.Error()}
		return res
	}
	defer resp.Body.Close()
	// 204 是约定探测响应；部分实现返回 200 也视为可达（放宽容错）。
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		res.Err = &ProbeError{Reason: "probe url returned " + resp.Status}
		return res
	}
	res.HTTPLatency = time.Since(httpStart)
	return res
}

// Selection 一次选线结果。
type Selection struct {
	// LineID 当前应使用的线路；无可用线路时为空串。
	LineID string
	// Locked 是否处于手动锁线状态。
	Locked bool
	// Latency 当前线路最近一次测得的 TCP 延迟。
	Latency time.Duration
}

// Selector 持有线路集合与选线状态，负责并发测速与自动/手动选线。
type Selector struct {
	mu sync.Mutex

	prober Prober
	// tolerance 延迟差在此范围内不切换（防抖）。
	tolerance time.Duration
	// maxConcurrent 单轮探测的最大并发数（信号量限流，避免线路过多时
	// 同时打爆对端/本机连接）。
	maxConcurrent int
	// lines 当前线路（按添加顺序）。
	lines []Line
	// results 最近一次测速结果（lineID -> result）。
	results map[string]ProbeResult

	// failureThreshold 连续失败多少次后线路才判为不可用（默认 1：任何一次
	// 失败都立即视为不可用；调大可容忍瞬时抖动，避免频繁切线）。
	failureThreshold int
	// failStreak 每条线路的连续失败次数（成功探测时清零）。
	failStreak map[string]int
	// lastGood 每条线路最近一次成功探测的结果（失败未达阈值时兜底选线）。
	lastGood map[string]ProbeResult

	// lockedLine 手动锁定的线路；空串表示自动模式。
	lockedLine string
	// current 当前生效线路 id。
	current string
}

// NewSelector 创建选择器。tolerance<=0 时取默认 50ms；maxConcurrent<=0 时
// 取默认 8。
func NewSelector(prober Prober, tolerance time.Duration) *Selector {
	if prober == nil {
		prober = &TCPProber{}
	}
	if tolerance <= 0 {
		tolerance = 50 * time.Millisecond
	}
	return &Selector{
		prober:           prober,
		tolerance:        tolerance,
		maxConcurrent:    8,
		failureThreshold: 1,
		results:          make(map[string]ProbeResult),
		failStreak:       make(map[string]int),
		lastGood:         make(map[string]ProbeResult),
	}
}

// SetMaxConcurrent 设置单轮探测的最大并发数（须在首次 ProbeAll 前调用，
// 否则不保证生效）。<=0 时重置为默认 8。
func (s *Selector) SetMaxConcurrent(n int) {
	if n <= 0 {
		n = 8
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxConcurrent = n
}

// SetFailureThreshold 设置线路判为不可用所需的连续失败次数（须在首次
// ProbeAll 前调用，否则不保证生效）。默认 1（任何一次失败立即判不可用）；
// 调大可容忍瞬时抖动，例如 2 表示连续两次探测失败才切换线路。
func (s *Selector) SetFailureThreshold(n int) {
	if n <= 0 {
		n = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failureThreshold = n
}

// SetLines 全量替换线路集合（保留锁线与当前选择；失效的锁线自动解除）。
// 拷贝传入 slice，避免调用方后续修改污染内部状态。
func (s *Selector) SetLines(lines []Line) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lines = append([]Line(nil), lines...)
	known := make(map[string]bool, len(lines))
	for _, l := range lines {
		known[l.ID] = true
	}
	for id := range s.results {
		if !known[id] {
			delete(s.results, id)
		}
	}
	for id := range s.failStreak {
		if !known[id] {
			delete(s.failStreak, id)
			delete(s.lastGood, id)
		}
	}
	if s.lockedLine != "" && !known[s.lockedLine] {
		s.lockedLine = ""
	}
	if s.current != "" && !known[s.current] {
		s.current = ""
	}
}

// Lines 返回当前线路列表（拷贝）。
func (s *Selector) Lines() []Line {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Line(nil), s.lines...)
}

// ProbeAll 并发探测全部线路并刷新结果，返回按线路 id 索引的结果。
// 并发数受 maxConcurrent 信号量限制，线路过多时不会同时打爆对端/本机连接。
func (s *Selector) ProbeAll(ctx context.Context) map[string]ProbeResult {
	s.mu.Lock()
	lines := append([]Line(nil), s.lines...)
	maxConcurrent := s.maxConcurrent
	s.mu.Unlock()

	if len(lines) == 0 {
		return map[string]ProbeResult{}
	}

	// 信号量限流：最多 maxConcurrent 个探测并发执行。
	sem := make(chan struct{}, maxConcurrent)
	results := make(map[string]ProbeResult, len(lines))
	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, line := range lines {
		wg.Add(1)
		go func(l Line) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				results[l.ID] = ProbeResult{LineID: l.ID, Err: &ProbeError{Reason: "probe canceled"}}
				mu.Unlock()
				return
			}
			r := s.prober.Probe(ctx, l)
			mu.Lock()
			results[l.ID] = r
			mu.Unlock()
		}(line)
	}
	wg.Wait()

	s.mu.Lock()
	s.results = results
	// 更新连续失败计数与最近成功结果：成功清零，失败累加（供阈值判可用）。
	for id, r := range results {
		if r.Err != nil {
			s.failStreak[id]++
		} else {
			s.failStreak[id] = 0
			s.lastGood[id] = r
		}
	}
	s.mu.Unlock()
	return results
}

// effectiveLatency 返回用于选线的延迟：配置了 ProbeURL 且 HTTP 探测成功时
// 以 HTTP 延迟为准（它反映真实的出网可用性），否则回退到 TCP 握手延迟。
func effectiveLatency(r ProbeResult) time.Duration {
	if r.HTTPLatency > 0 {
		return r.HTTPLatency
	}
	return r.TCPLatency
}

// domainHost 从 host:port 形式的地址中提取 host（域名层探测用）。
func domainHost(address string) string {
	if address == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		// 无端口时视为纯 host
		return address
	}
	return host
}

// usable 判断线路当前是否可用：最近一次探测成功；或失败但连续失败次数未达
// 阈值（此时用最近成功结果兜底选线）。无任何成功记录的线路始终视为不可用。
func (s *Selector) usable(r ProbeResult) bool {
	if r.Err == nil {
		return true
	}
	if s.failStreak[r.LineID] >= s.failureThreshold {
		return false
	}
	_, ok := s.lastGood[r.LineID]
	return ok
}

// latencyFor 返回线路参与排序的延迟：失败但未达阈值时用最近成功结果兜底，
// 避免瞬时抖动把延迟抬到异常高。
func (s *Selector) latencyFor(r ProbeResult) time.Duration {
	if r.Err != nil {
		if lg, ok := s.lastGood[r.LineID]; ok {
			return effectiveLatency(lg)
		}
	}
	return effectiveLatency(r)
}

// Select 根据最新结果选线：锁线优先；否则取最快可用线路，且与当前线路的
// 延迟差超过 tolerance 才切换（防抖）。
func (s *Selector) Select() Selection {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lockedLine != "" {
		if r, ok := s.results[s.lockedLine]; ok && s.usable(r) {
			s.current = s.lockedLine
			return Selection{LineID: s.lockedLine, Locked: true, Latency: s.latencyFor(r)}
		}
		// 锁定的线路失效：解除锁，退回自动模式。
		s.lockedLine = ""
	}

	best := s.bestUsable()
	if best == "" {
		s.current = ""
		return Selection{Locked: false}
	}

	// 防抖：当前线路仍可用，且不比最优慢超过 tolerance 时保持现状；
	// 只有当当前线路比最优慢得更多（或更慢）才切换，避免频繁抖动。
	if s.current != "" {
		cur, curOK := s.results[s.current]
		bestR, bestOK := s.results[best]
		if curOK && bestOK && s.usable(cur) && s.latencyFor(cur)-s.latencyFor(bestR) <= s.tolerance {
			return Selection{LineID: s.current, Latency: s.latencyFor(cur)}
		}
	}
	s.current = best
	return Selection{LineID: best, Latency: s.latencyFor(s.results[best])}
}

// bestUsable 返回可用线路中延迟最小的一条；无可用时返回空串。
// 排序键为 effectiveLatency：配置了 ProbeURL 时优先按 HTTP 出网延迟，
// 否则按 TCP 握手延迟。
func (s *Selector) bestUsable() string {
	type cand struct {
		id  string
		lat time.Duration
	}
	var cands []cand
	for id, r := range s.results {
		if !s.usable(r) {
			continue
		}
		cands = append(cands, cand{id: id, lat: s.latencyFor(r)})
	}
	if len(cands) == 0 {
		return ""
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].lat < cands[j].lat })
	return cands[0].id
}

// Lock 手动锁定某条线路（必须存在于当前线路集合中，否则忽略）。
func (s *Selector) Lock(lineID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, l := range s.lines {
		if l.ID == lineID {
			s.lockedLine = lineID
			s.current = lineID
			return
		}
	}
}

// Unlock 解除手动锁线，回到自动模式。清空 current，使下一次 Select
// 立即按最新测速结果选择最优线路，而不是留在被锁的那条上。
func (s *Selector) Unlock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockedLine = ""
	s.current = ""
}

// State 返回当前完整状态（供 API/UI 展示）。
type State struct {
	Lines   []Line
	Results map[string]ProbeResult
	Current string
	Locked  string
}

// Snapshot 返回当前状态（拷贝，线程安全）。
func (s *Selector) Snapshot() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	results := make(map[string]ProbeResult, len(s.results))
	for k, v := range s.results {
		results[k] = v
	}
	return State{
		Lines:   append([]Line(nil), s.lines...),
		Results: results,
		Current: s.current,
		Locked:  s.lockedLine,
	}
}

// Run 后台守护：按 interval 周期探测并自动选线，直到 ctx 取消。
// 探测并发进行，单轮最坏耗时约等于探测超时；不会阻塞调用方。
func (s *Selector) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.ProbeAll(ctx)
			sel := s.Select()
			// 用返回值而非直接读 s.current（后者在锁外，会与并发调用产生数据竞争）。
			log.Printf("[selector] current line: %q", sel.LineID)
		}
	}
}
