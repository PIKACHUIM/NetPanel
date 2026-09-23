package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterWindow(t *testing.T) {
	l := NewLimiter(50*time.Millisecond, 3)
	for i := 0; i < 3; i++ {
		if !l.Allow("ip1") {
			t.Fatalf("第 %d 次请求应放行", i+1)
		}
	}
	if l.Allow("ip1") {
		t.Error("超限后应拒绝")
	}
	// 不同 key 互不影响
	if !l.Allow("ip2") {
		t.Error("不同 key 应独立计数")
	}
	// 窗口过后恢复
	time.Sleep(60 * time.Millisecond)
	if !l.Allow("ip1") {
		t.Error("窗口过后应重新放行")
	}
}

func TestBackoffProgression(t *testing.T) {
	b := NewBackoff(1*time.Second, 1*time.Minute, 3)
	key := "ip|user"

	// 前 3 次失败不锁定
	for i := 0; i < 3; i++ {
		b.RecordFailure(key)
		if _, blocked := b.Blocked(key); blocked {
			t.Fatalf("第 %d 次失败不应锁定", i+1)
		}
	}
	// 第 4 次失败开始锁定
	b.RecordFailure(key)
	if wait, blocked := b.Blocked(key); !blocked || wait <= 0 {
		t.Error("连续失败超过阈值后应锁定")
	}
	// 成功后重置
	b.Reset(key)
	if _, blocked := b.Blocked(key); blocked {
		t.Error("重置后不应锁定")
	}
}

func TestBackoffResetAfterIdle(t *testing.T) {
	b := NewBackoff(10*time.Millisecond, 20*time.Millisecond, 0)
	key := "ip|user"
	for i := 0; i < 5; i++ {
		b.RecordFailure(key)
	}
	time.Sleep(25 * time.Millisecond)
	if _, blocked := b.Blocked(key); blocked {
		t.Error("超过 max 空闲期后应视为放弃攻击并重置")
	}
}
