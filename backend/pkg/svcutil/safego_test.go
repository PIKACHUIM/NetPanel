package svcutil

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestSafeGoRecoversPanicAndRestarts(t *testing.T) {
	var runs int32
	done := make(chan struct{})

	SafeGo(logrus.New(), "test.restart", true, func() {
		n := atomic.AddInt32(&runs, 1)
		if n >= 3 {
			close(done) // 第三次运行视为已自动纠偏，正常返回
			return
		}
		panic("boom")
	})

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("panic 后未自动重启，运行次数=%d", atomic.LoadInt32(&runs))
	}
	if atomic.LoadInt32(&runs) < 3 {
		t.Fatalf("运行次数不足 3，实际 %d", atomic.LoadInt32(&runs))
	}
}

func TestSafeGoNoRestart(t *testing.T) {
	var runs int32
	SafeGo(logrus.New(), "test.norestart", false, func() {
		atomic.AddInt32(&runs, 1)
		panic("boom")
	})

	time.Sleep(300 * time.Millisecond)
	if n := atomic.LoadInt32(&runs); n != 1 {
		t.Fatalf("restart=false 应只运行 1 次，实际 %d", n)
	}
}

func TestSafeGoNormalExitNoRestart(t *testing.T) {
	var runs int32
	done := make(chan struct{})
	SafeGo(logrus.New(), "test.normal", true, func() {
		atomic.AddInt32(&runs, 1)
		close(done) // 正常返回，不应重启
	})

	<-done
	time.Sleep(300 * time.Millisecond)
	if n := atomic.LoadInt32(&runs); n != 1 {
		t.Fatalf("正常退出不应重启，运行 %d 次", n)
	}
}

func TestOnceGuardTriggersOnce(t *testing.T) {
	var n int32
	g := NewOnceGuard(func() { atomic.AddInt32(&n, 1) })
	g.Trigger()
	g.Trigger()
	g.Trigger()
	if atomic.LoadInt32(&n) != 1 {
		t.Fatalf("应只触发 1 次，实际 %d", n)
	}
}
