package svcutil

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestDelayReturnsFalseAfterFullDuration(t *testing.T) {
	start := time.Now()
	if Delay(300*time.Millisecond, func() bool { return false }) {
		t.Fatal("stop 恒为 false 时 Delay 不应返回 true")
	}
	if el := time.Since(start); el < 250*time.Millisecond {
		t.Fatalf("实际等待 %v，短于预期 250ms", el)
	}
}

func TestDelayAbortsEarlyWhenStopFlips(t *testing.T) {
	var stop atomic.Bool
	go func() {
		time.Sleep(120 * time.Millisecond)
		stop.Store(true)
	}()

	start := time.Now()
	aborted := Delay(5*time.Second, stop.Load)
	el := time.Since(start)

	if !aborted {
		t.Fatal("stop 变为 true 后 Delay 应返回 true")
	}
	if el > time.Second {
		t.Fatalf("提前打断耗时 %v，不应接近 5s", el)
	}
}

func TestDelayZeroDurationChecksStopImmediately(t *testing.T) {
	if !Delay(0, func() bool { return true }) {
		t.Fatal("d<=0 且 stop 为 true 时应返回 true")
	}
	if Delay(0, func() bool { return false }) {
		t.Fatal("d<=0 且 stop 为 false 时应返回 false")
	}
	if Delay(-time.Second, nil) {
		t.Fatal("负数延迟应视为立即完成")
	}
}

func TestDelayNilStopSleeps(t *testing.T) {
	start := time.Now()
	if Delay(200*time.Millisecond, nil) {
		t.Fatal("stop 为 nil 时不应返回 true")
	}
	if el := time.Since(start); el < 150*time.Millisecond {
		t.Fatalf("nil stop 时应完整等待，实际 %v", el)
	}
}
