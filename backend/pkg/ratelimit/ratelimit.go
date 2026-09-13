// Package ratelimit 进程内限流器，用于登录、初始化等无认证端点的防滥用。
// 单实例内存态（重启即清零）；对多实例部署仅提供实例级限制，足够覆盖
// 面板典型的单实例家庭场景。
package ratelimit

import (
	"sync"
	"time"
)

// Limiter 固定窗口计数限流器（按 key，通常传客户端 IP）。
type Limiter struct {
	mu     sync.Mutex
	window time.Duration
	max    int
	counts map[string]*windowCount
}

type windowCount struct {
	start time.Time
	n     int
}

// NewLimiter 创建限流器：每个窗口期内每 key 最多放行 max 次请求。
func NewLimiter(window time.Duration, max int) *Limiter {
	return &Limiter{window: window, max: max, counts: make(map[string]*windowCount)}
}

// Allow 记录一次请求并判断是否放行。
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	wc, ok := l.counts[key]
	if !ok || now.Sub(wc.start) >= l.window {
		// 惰性清理：key 数量过大时移除已过期窗口，防止内存无界增长
		if len(l.counts) >= 4096 {
			for k, v := range l.counts {
				if now.Sub(v.start) >= l.window {
					delete(l.counts, k)
				}
			}
		}
		l.counts[key] = &windowCount{start: now, n: 1}
		return true
	}
	wc.n++
	return wc.n <= l.max
}

// Backoff 登录失败指数退避：连续失败达到阈值后，按倍数增长锁定时长。
type Backoff struct {
	mu      sync.Mutex
	base    time.Duration
	max     time.Duration
	free    int // 免锁定失败次数
	entries map[string]*backoffEntry
}

type backoffEntry struct {
	failures int
	lastFail time.Time
}

// NewBackoff 创建退避器：连续失败 free 次后开始锁定，
// 锁定时长 = base × 2^(超出次数)，封顶 max。
func NewBackoff(base, max time.Duration, free int) *Backoff {
	return &Backoff{base: base, max: max, free: free, entries: make(map[string]*backoffEntry)}
}

// Blocked 返回该 key 是否处于锁定状态及剩余等待时长。
func (b *Backoff) Blocked(key string) (time.Duration, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.entries[key]
	if !ok {
		return 0, false
	}
	// 距上次失败超过 max 视为放弃攻击，重置
	if time.Since(e.lastFail) > b.max {
		delete(b.entries, key)
		return 0, false
	}
	if e.failures <= b.free {
		return 0, false
	}
	lock := b.base << (e.failures - b.free - 1)
	if lock > b.max {
		lock = b.max
	}
	remaining := lock - time.Since(e.lastFail)
	if remaining <= 0 {
		return 0, false
	}
	return remaining, true
}

// RecordFailure 记录一次失败。
func (b *Backoff) RecordFailure(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	e, ok := b.entries[key]
	if !ok || time.Since(e.lastFail) > b.max {
		b.entries[key] = &backoffEntry{failures: 1, lastFail: time.Now()}
		return
	}
	e.failures++
	e.lastFail = time.Now()
}

// Reset 登录成功后清除该 key 的失败记录。
func (b *Backoff) Reset(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, key)
}
