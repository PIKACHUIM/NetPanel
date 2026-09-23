// Package ratelimit 提供简单的内存级登录失败限流。
//
// 登录接口此前无任何失败次数限制，攻击者可对已知用户名做在线暴力破解。
// 这里按「客户端 IP + 用户名」维度做滑动窗口计数，达到阈值后临时拒绝，
// 并在窗口超时后自动恢复。仅适用于单进程部署，多实例需接入共享存储。
package ratelimit

import (
	"sync"
	"time"
)

// entry 单个维度的计数状态。
type entry struct {
	failures int
	window   time.Time
	blocked  bool
	until    time.Time
}

// Limiter 滑动窗口限流器。
type Limiter struct {
	mu          sync.Mutex
	entries     map[string]*entry
	maxFailures int
	window      time.Duration
	blockFor    time.Duration
}

// New 创建限流器。
func New(maxFailures int, window, blockFor time.Duration) *Limiter {
	return &Limiter{
		entries:     make(map[string]*entry),
		maxFailures: maxFailures,
		window:      window,
		blockFor:    blockFor,
	}
}

// Allowed 判断该 key 当前是否允许尝试；key 通常为 IP+用户名。
func (l *Limiter) Allowed(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.entries[key]
	if !ok {
		return true
	}
	if e.blocked {
		if now.Before(e.until) {
			return false
		}
		// 封禁到期，重置
		delete(l.entries, key)
		return true
	}
	// 窗口已滚动，计数清零
	if now.Sub(e.window) > l.window {
		delete(l.entries, key)
	}
	return true
}

// RecordFailure 记录一次失败。
func (l *Limiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.window) > l.window {
		e = &entry{window: now}
		l.entries[key] = e
	}
	e.failures++
	if e.failures >= l.maxFailures {
		e.blocked = true
		e.until = now.Add(l.blockFor)
	}
}

// Reset 登录成功后清零该 key 的失败计数。
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}
