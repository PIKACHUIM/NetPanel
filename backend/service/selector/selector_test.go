package selector

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeProber 注入式探测器：按线路 id 返回固定延迟/错误。
type fakeProber struct {
	latencies map[string]time.Duration
	errors    map[string]error
}

func (f *fakeProber) Probe(_ context.Context, line Line) ProbeResult {
	if err := f.errors[line.ID]; err != nil {
		return ProbeResult{LineID: line.ID, Err: err}
	}
	return ProbeResult{LineID: line.ID, TCPLatency: f.latencies[line.ID]}
}

func mkLines(ids ...string) []Line {
	lines := make([]Line, 0, len(ids))
	for i, id := range ids {
		lines = append(lines, Line{ID: id, Name: id, Tool: "fake", Address: "127.0.0.1:1"})
		_ = i
	}
	return lines
}

func newFake(lines []Line, lat map[string]time.Duration, errs map[string]error) *Selector {
	s := NewSelector(&fakeProber{latencies: lat, errors: errs}, 50*time.Millisecond)
	s.SetLines(lines)
	return s
}

func TestSelectPicksFastest(t *testing.T) {
	s := newFake(
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

func TestSelectSkipsUnavailableLines(t *testing.T) {
	s := newFake(
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
	s := newFake(
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
	s := newFake(
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
	s.results["b"] = ProbeResult{LineID: "b", TCPLatency: 40 * time.Millisecond}
	second := s.Select()
	if second.LineID != "a" {
		t.Fatalf("expected hysteresis keep a, got %q", second.LineID)
	}
	// b 提升到 30ms：a(90)-b(30)=60 > tolerance(50) → 切到 b。
	s.results["b"] = ProbeResult{LineID: "b", TCPLatency: 30 * time.Millisecond}
	third := s.Select()
	if third.LineID != "b" {
		t.Fatalf("expected switch to b after big gap, got %q", third.LineID)
	}
}

func TestLockOverridesAutoAndUnlockRestores(t *testing.T) {
	s := newFake(
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
	s := newFake(mkLines("a"), map[string]time.Duration{"a": 10 * time.Millisecond}, nil)
	s.ProbeAll(context.Background())
	s.Lock("does-not-exist")
	if s.lockedLine != "" {
		t.Fatalf("unknown lock should be ignored, got %q", s.lockedLine)
	}
}

func TestLockReleasedWhenLineDies(t *testing.T) {
	s := newFake(
		mkLines("a", "b"),
		map[string]time.Duration{"a": 10 * time.Millisecond, "b": 20 * time.Millisecond},
		nil,
	)
	s.ProbeAll(context.Background())
	s.Lock("a")
	// a 失效 → Select 应解除锁并退回可用线路 b。
	s.results["a"] = ProbeResult{LineID: "a", Err: &ProbeError{Reason: "timeout"}}
	sel := s.Select()
	if sel.Locked {
		t.Fatal("expected lock to be released")
	}
	if sel.LineID != "b" {
		t.Fatalf("expected fallback to b, got %q", sel.LineID)
	}
}

func TestSetLinesPrunesStaleResultsAndDeadLock(t *testing.T) {
	s := newFake(
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
	s := newFake(mkLines("a"), map[string]time.Duration{"a": 10 * time.Millisecond}, nil)
	s.ProbeAll(context.Background())
	s.Select()
	st := s.Snapshot()
	if len(st.Lines) != 1 || st.Current != "a" {
		t.Fatalf("unexpected snapshot: %+v", st)
	}
}
