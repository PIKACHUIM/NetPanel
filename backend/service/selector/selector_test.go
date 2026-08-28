package selector

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeProber 注入式探测器：按线路 id 返回固定延迟/错误。
// latencies/httpLatencies/errors 可变，测试可通过公开 API 路径（ProbeAll）
// 模拟线路质量变化，而不是直接写 Selector 内部状态。
type fakeProber struct {
	latencies     map[string]time.Duration
	httpLatencies map[string]time.Duration
	errors        map[string]error
}

func (f *fakeProber) Probe(_ context.Context, line Line) ProbeResult {
	if err := f.errors[line.ID]; err != nil {
		return ProbeResult{LineID: line.ID, Err: err}
	}
	res := ProbeResult{LineID: line.ID, TCPLatency: f.latencies[line.ID]}
	if hl, ok := f.httpLatencies[line.ID]; ok {
		res.HTTPLatency = hl
	}
	return res
}

// countingProber 包装器：统计最大并发探测数（验证信号量限流）。
type countingProber struct {
	inner  Prober
	mu     sync.Mutex
	active int
	peak   int
}

func (c *countingProber) Probe(ctx context.Context, line Line) ProbeResult {
	c.mu.Lock()
	c.active++
	if c.active > c.peak {
		c.peak = c.active
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.active--
		c.mu.Unlock()
	}()
	// 让探测留出重叠窗口，才能测出真实并发上限。
	time.Sleep(20 * time.Millisecond)
	return c.inner.Probe(ctx, line)
}

func mkLines(ids ...string) []Line {
	lines := make([]Line, 0, len(ids))
	for _, id := range ids {
		lines = append(lines, Line{ID: id, Name: id, Tool: "fake", Address: "127.0.0.1:1"})
	}
	return lines
}

func newFake(lines []Line, lat map[string]time.Duration, errs map[string]error) (*Selector, *fakeProber) {
	f := &fakeProber{latencies: lat, errors: errs}
	s := NewSelector(f, 50*time.Millisecond)
	s.SetLines(lines)
	return s, f
}

func TestSelectPicksFastest(t *testing.T) {
	s, _ := newFake(
		mkLines("a", "b", "c"),
		map[string]time.Duration{"a": 300 * time.Millisecond, "b": 100 * time.Millisecond, "c": 200 * time.Millisecond},
		nil,
	)
	s.ProbeAll(context.Background())
	sel := s.Select()
	if sel.LineID != "b" {
		t.Fatalf("expected fastest line b, got %q", sel.LineID)
	}
	if sel.Locked {
		t.Fatal("expected auto mode, got locked")
	}
}

func TestToolFilterRestrictsAutoSelection(t *testing.T) {
	// 三条线路：a/b 属 wireguard（参与自动选线），c 属 frp（被过滤）。
	// c 延迟最低，但过滤后不应被自动选中；手动锁定 c 仍应生效。
	lines := []Line{
		{ID: "a", Name: "a", Tool: "wireguard", Address: "127.0.0.1:1"},
		{ID: "b", Name: "b", Tool: "wireguard", Address: "127.0.0.1:1"},
		{ID: "c", Name: "c", Tool: "frp", Address: "127.0.0.1:1"},
	}
	f := &fakeProber{
		latencies: map[string]time.Duration{
			"a": 50 * time.Millisecond,
			"b": 100 * time.Millisecond,
			"c": 10 * time.Millisecond,
		},
	}
	s := NewSelector(f, 50*time.Millisecond)
	s.SetToolFilter([]string{"wireguard"})
	s.SetLines(lines)
	s.ProbeAll(context.Background())

	// 自动选线：过滤后只在 wireguard 中选，a(50ms) < b(100ms) → a。
	sel := s.Select()
	if sel.LineID != "a" {
		t.Fatalf("expected filtered auto-select a, got %q", sel.LineID)
	}

	// 手动锁定被过滤工具线路 c：仍应生效（不受工具过滤影响）。
	s.Lock("c")
	sel = s.Select()
	if sel.LineID != "c" || !sel.Locked {
		t.Fatalf("expected manual lock c to win, got %q locked=%v", sel.LineID, sel.Locked)
	}
}

func TestToolFilterEmptyMeansAllTools(t *testing.T) {
	lines := []Line{
		{ID: "a", Name: "a", Tool: "frp", Address: "127.0.0.1:1"},
		{ID: "b", Name: "b", Tool: "wireguard", Address: "127.0.0.1:1"},
	}
	f := &fakeProber{
		latencies: map[string]time.Duration{"a": 10 * time.Millisecond, "b": 200 * time.Millisecond},
	}
	s := NewSelector(f, 50*time.Millisecond)
	s.SetToolFilter(nil) // 空 = 全部参与
	s.SetLines(lines)
	s.ProbeAll(context.Background())
	sel := s.Select()
	if sel.LineID != "a" {
		t.Fatalf("expected unfiltered fastest a, got %q", sel.LineID)
	}
}

func TestSelectPrefersHTTPLatencyWhenProbed(t *testing.T) {
	// b 的 TCP 握手更慢，但 HTTP 出网更快；配置了 ProbeURL 时应选 b。
	// 覆盖 #B 修复：测速结果里的 HTTP 延迟参与选线排序。
	lines := []Line{
		{ID: "a", Name: "a", Tool: "fake", Address: "127.0.0.1:1", ProbeURL: "http://a/probe"},
		{ID: "b", Name: "b", Tool: "fake", Address: "127.0.0.1:1", ProbeURL: "http://b/probe"},
	}
	f := &fakeProber{
		latencies:     map[string]time.Duration{"a": 10 * time.Millisecond, "b": 30 * time.Millisecond},
		httpLatencies: map[string]time.Duration{"a": 200 * time.Millisecond, "b": 20 * time.Millisecond},
	}
	s := NewSelector(f, 50*time.Millisecond)
	s.SetLines(lines)
	s.ProbeAll(context.Background())
	sel := s.Select()
	if sel.LineID != "b" {
		t.Fatalf("expected HTTP-fastest b, got %q", sel.LineID)
	}
	if sel.Latency != 20*time.Millisecond {
		t.Fatalf("expected effective latency 20ms, got %v", sel.Latency)
	}
}

func TestSelectSkipsUnavailableLines(t *testing.T) {
	s, _ := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"b": 100 * time.Millisecond},
		map[string]error{"a": &ProbeError{Reason: "refused"}},
	)
	s.ProbeAll(context.Background())
	sel := s.Select()
	if sel.LineID != "b" {
		t.Fatalf("expected fallback to b, got %q", sel.LineID)
	}
}

func TestSelectEmptyWhenAllDown(t *testing.T) {
	s, _ := newFake(
		mkLines("a", "b"),
		nil,
		map[string]error{"a": errors.New("down"), "b": errors.New("down")},
	)
	s.ProbeAll(context.Background())
	sel := s.Select()
	if sel.LineID != "" {
		t.Fatalf("expected empty selection, got %q", sel.LineID)
	}
}

func TestToleranceHysteresis(t *testing.T) {
	// a=90ms、b=100ms：首次 Select（无当前线路）直接选最快 a。
	s, f := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 90 * time.Millisecond, "b": 100 * time.Millisecond},
		nil,
	)
	s.ProbeAll(context.Background())
	first := s.Select()
	if first.LineID != "a" {
		t.Fatalf("first select should pick fastest a, got %q", first.LineID)
	}
	// b 提升到 40ms：a(90)-b(40)=50 <= tolerance(50) → 保持 a，不抖动。
	f.latencies["b"] = 40 * time.Millisecond
	s.ProbeAll(context.Background())
	second := s.Select()
	if second.LineID != "a" {
		t.Fatalf("expected hysteresis keep a, got %q", second.LineID)
	}
	// b 提升到 30ms：a(90)-b(30)=60 > tolerance(50) → 切到 b。
	f.latencies["b"] = 30 * time.Millisecond
	s.ProbeAll(context.Background())
	third := s.Select()
	if third.LineID != "b" {
		t.Fatalf("expected switch to b after big gap, got %q", third.LineID)
	}
}

func TestLockOverridesAutoAndUnlockRestores(t *testing.T) {
	s, _ := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 300 * time.Millisecond, "b": 100 * time.Millisecond},
		nil,
	)
	s.ProbeAll(context.Background())
	s.Lock("a")
	sel := s.Select()
	if !sel.Locked || sel.LineID != "a" {
		t.Fatalf("expected locked a, got %+v", sel)
	}
	s.Unlock()
	sel = s.Select()
	if sel.Locked || sel.LineID != "b" {
		t.Fatalf("expected auto back to b, got %+v", sel)
	}
}

func TestLockUnknownLineIgnored(t *testing.T) {
	s, _ := newFake(mkLines("a"), map[string]time.Duration{"a": 10 * time.Millisecond}, nil)
	s.ProbeAll(context.Background())
	s.Lock("does-not-exist")
	if s.lockedLine != "" {
		t.Fatalf("unknown lock should be ignored, got %q", s.lockedLine)
	}
}

func TestLockReleasedWhenLineDies(t *testing.T) {
	s, f := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 10 * time.Millisecond, "b": 20 * time.Millisecond},
		nil,
	)
	s.ProbeAll(context.Background())
	s.Lock("a")
	// a 失效 → Select 应解除锁并退回可用线路 b。
	f.errors = map[string]error{"a": &ProbeError{Reason: "timeout"}}
	s.ProbeAll(context.Background())
	sel := s.Select()
	if sel.Locked {
		t.Fatal("expected lock to be released")
	}
	if sel.LineID != "b" {
		t.Fatalf("expected fallback to b, got %q", sel.LineID)
	}
}

func TestSetLinesPrunesStaleResultsAndDeadLock(t *testing.T) {
	s, _ := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 10 * time.Millisecond, "b": 20 * time.Millisecond},
		nil,
	)
	s.ProbeAll(context.Background())
	s.Lock("a")
	s.SetLines(mkLines("b", "c"))
	if _, ok := s.results["a"]; ok {
		t.Fatal("stale result for removed line a should be pruned")
	}
	if s.lockedLine != "" {
		t.Fatal("lock on removed line should be released")
	}
}

func TestSnapshotThreadSafe(t *testing.T) {
	s, _ := newFake(mkLines("a"), map[string]time.Duration{"a": 10 * time.Millisecond}, nil)
	s.ProbeAll(context.Background())
	s.Select()
	st := s.Snapshot()
	if len(st.Lines) != 1 || st.Current != "a" {
		t.Fatalf("unexpected snapshot: %+v", st)
	}
}

func TestProbeAllConcurrencyLimited(t *testing.T) {
	// 8 条线路、并发上限 3：实测最大并发必须 <= 3（覆盖 #D 信号量限流）。
	f := &fakeProber{
		latencies: map[string]time.Duration{
			"a": 1, "b": 1, "c": 1, "d": 1, "e": 1, "f": 1, "g": 1, "h": 1,
		},
	}
	counter := &countingProber{inner: f}
	s := NewSelector(counter, 50*time.Millisecond)
	s.SetMaxConcurrent(3)
	s.SetLines(mkLines("a", "b", "c", "d", "e", "f", "g", "h"))
	s.ProbeAll(context.Background())
	if counter.peak > 3 {
		t.Fatalf("expected peak concurrency <= 3, got %d", counter.peak)
	}
	if counter.peak < 1 {
		t.Fatal("expected at least some concurrency")
	}
}

func TestFailureThresholdTransientFailureKeepsLine(t *testing.T) {
	// 阈值 2：a 瞬时失败一次（failStreak=1 < 2），仍应保持 a，不切线。
	s, f := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 10 * time.Millisecond, "b": 20 * time.Millisecond},
		nil,
	)
	s.SetFailureThreshold(2)
	s.ProbeAll(context.Background())
	first := s.Select()
	if first.LineID != "a" {
		t.Fatalf("expected initial pick a, got %q", first.LineID)
	}
	f.errors = map[string]error{"a": &ProbeError{Reason: "timeout"}}
	s.ProbeAll(context.Background())
	second := s.Select()
	if second.LineID != "a" {
		t.Fatalf("expected keep a after one transient failure, got %q", second.LineID)
	}
	if second.Latency != 10*time.Millisecond {
		t.Fatalf("expected fallback latency from lastGood 10ms, got %v", second.Latency)
	}
}

func TestFailureThresholdPersistentFailureSwitches(t *testing.T) {
	// 阈值 2：a 连续失败两次（failStreak=2 >= 2）后判不可用，切到 b。
	s, f := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 10 * time.Millisecond, "b": 20 * time.Millisecond},
		nil,
	)
	s.SetFailureThreshold(2)
	s.ProbeAll(context.Background())
	if sel := s.Select(); sel.LineID != "a" {
		t.Fatalf("expected initial pick a, got %q", sel.LineID)
	}
	f.errors = map[string]error{"a": &ProbeError{Reason: "timeout"}}
	s.ProbeAll(context.Background()) // failStreak[a]=1
	if sel := s.Select(); sel.LineID != "a" {
		t.Fatalf("expected still a after 1 failure, got %q", sel.LineID)
	}
	s.ProbeAll(context.Background()) // failStreak[a]=2，达到阈值
	third := s.Select()
	if third.LineID != "b" {
		t.Fatalf("expected switch to b after 2 failures, got %q", third.LineID)
	}
}

func TestFailureThresholdRecoveryReturnsToFastest(t *testing.T) {
	// 阈值 2：a 失败两次期间选到 b；a 恢复且延迟优势超过容差后，应自动切回 a。
	// b 用 100ms 使 a(10) 与 b(100) 差 90ms > tolerance(50)，恢复后必然回切。
	s, f := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 10 * time.Millisecond, "b": 100 * time.Millisecond},
		nil,
	)
	s.SetFailureThreshold(2)
	s.ProbeAll(context.Background())
	if sel := s.Select(); sel.LineID != "a" {
		t.Fatalf("expected initial pick a, got %q", sel.LineID)
	}
	f.errors = map[string]error{"a": &ProbeError{Reason: "timeout"}}
	s.ProbeAll(context.Background()) // failStreak[a]=1，仍保持 a
	if sel := s.Select(); sel.LineID != "a" {
		t.Fatalf("expected keep a after 1 failure, got %q", sel.LineID)
	}
	s.ProbeAll(context.Background()) // failStreak[a]=2，切到 b
	if sel := s.Select(); sel.LineID != "b" {
		t.Fatalf("expected b after 2 failures, got %q", sel.LineID)
	}
	delete(f.errors, "a") // a 恢复
	s.ProbeAll(context.Background())
	final := s.Select()
	if final.LineID != "a" {
		t.Fatalf("expected switch back to a after recovery, got %q", final.LineID)
	}
}

func TestFailureThresholdNoLastGoodStaysUnusable(t *testing.T) {
	// 从未成功过的线路即使失败次数低于阈值也不可用（无 lastGood 兜底）。
	s, _ := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"b": 20 * time.Millisecond},
		map[string]error{"a": &ProbeError{Reason: "refused"}},
	)
	s.SetFailureThreshold(3)
	s.ProbeAll(context.Background()) // failStreak[a]=1
	if sel := s.Select(); sel.LineID != "b" {
		t.Fatalf("expected only b usable, got %q", sel.LineID)
	}
}

func TestDomainHost(t *testing.T) {
	cases := map[string]string{
		"my-tunnel.cfargotunnel.com:443": "my-tunnel.cfargotunnel.com",
		"1.2.3.4:7000":                   "1.2.3.4",
		"plain-host":                     "plain-host",
		"":                               "",
	}
	for in, want := range cases {
		if got := domainHost(in); got != want {
			t.Errorf("domainHost(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSetToleranceChangesHysteresisWindow 验证 SetTolerance 后防抖窗口随之变化：
// 设置大容差时，当前线路即使比最优慢很多也不切换（在容差内）。
func TestSetToleranceChangesHysteresisWindow(t *testing.T) {
	s, f := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 10 * time.Millisecond, "b": 20 * time.Millisecond},
		nil,
	)
	s.ProbeAll(context.Background())
	first := s.Select()
	if first.LineID != "a" {
		t.Fatalf("first select should pick fastest a, got %q", first.LineID)
	}

	// 设置大容差 500ms：b 提升到 11ms（a=10，差 1ms），应保持 a
	s.SetTolerance(500 * time.Millisecond)
	f.latencies["b"] = 11 * time.Millisecond
	s.ProbeAll(context.Background())
	second := s.Select()
	if second.LineID != "a" {
		t.Fatalf("with large tolerance, expected keep a, got %q", second.LineID)
	}

	// 切回小容差 1µs：b 提升到 9ms（比 a=10ms 快 1ms），差距 > 容差 → 切到 b
	s.SetTolerance(1 * time.Microsecond)
	f.latencies["b"] = 9 * time.Millisecond
	s.ProbeAll(context.Background())
	third := s.Select()
	if third.LineID != "b" {
		t.Fatalf("with tiny tolerance, expected switch to b, got %q", third.LineID)
	}
}
