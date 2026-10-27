// Package svcutil 服务运行时工具
package svcutil

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SafeGo 以 panic 隔离方式运行 fn：
//   - fn 内任何 panic 被捕获并记录堆栈，不会拖垮整个进程；
//   - restart 非nil 时，panic 后按指数退避自动重启 fn（1s 起，每次翻倍，
//     上限 60s），实现「部分崩溃不影响核心 + 自动纠偏」；
//   - 正常路径零开销。
//
// 典型用法（manager 的循环任务）：
//
//	svcutil.SafeGo(log, "monitor.probe", stopCh, func() {
//	    for { select { case <-ticker.C: doProbe() ; case <-stopCh: return } }
//	})
//
// 注意：fn 需自己响应 stopCh 退出；重启时从函数头重新执行（ticker 需在 fn
// 内创建，确保每次重启都是全新状态）。
func SafeGo(log *logrus.Logger, name string, restart bool, fn func()) {
	go func() {
		backoff := time.Second
		for {
			err := runRecovered(fn)
			if err == nil {
				return // 正常退出（fn 返回），不再重启
			}
			log.Errorf("[%s] panic 已隔离: %v\n%s", name, err, debug.Stack())
			if !restart {
				return
			}
			time.Sleep(backoff)
			if backoff < 60*time.Second {
				backoff *= 2
			}
		}
	}()
}

// runRecovered 运行 fn，把 panic 转为 error；正常返回 nil
func runRecovered(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	fn()
	return nil
}

// OnceGuard 保证多个 goroutine 只触发一次的关闭（辅助工具，防重复 close panic）
type OnceGuard struct {
	mu   sync.Mutex
	done bool
	fn   func()
}

// NewOnceGuard 创建一次性执行守卫
func NewOnceGuard(fn func()) *OnceGuard { return &OnceGuard{fn: fn} }

// Trigger 首次调用时执行 fn，后续调用无操作
func (g *OnceGuard) Trigger() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.done {
		g.done = true
		g.fn()
	}
}

// 引擎心跳注册表：长驻循环每轮上报时间戳，供 /system/health 自检判断存活
var (
	engineHeartbeats = map[string]time.Time{}
	heartbeatMu      sync.Mutex
)

// BeatEngineHeartbeat 引擎循环每轮调用，上报自己还活着
func BeatEngineHeartbeat(name string) {
	heartbeatMu.Lock()
	engineHeartbeats[name] = time.Now()
	heartbeatMu.Unlock()
}

// EngineHeartbeats 返回心跳快照（只读副本）
func EngineHeartbeats() map[string]time.Time {
	heartbeatMu.Lock()
	defer heartbeatMu.Unlock()
	snap := make(map[string]time.Time, len(engineHeartbeats))
	for k, v := range engineHeartbeats {
		snap[k] = v
	}
	return snap
}
