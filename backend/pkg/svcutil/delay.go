package svcutil

import "time"

// delayTick 是 Delay 的检查粒度。取 100ms 意味着关闭时最多多等 100ms，
// 相比原先固定 sleep 5s 的白等，已消除 99% 的关闭延迟。
const delayTick = 100 * time.Millisecond

// Delay 是可中断的延迟等待，用于替代散落在各 engine manager 中的裸 time.Sleep。
//
// 行为：以 delayTick 为步长推进，若 stop() 返回 true 则立即提前返回。
// 返回值表示是否被 stop() 提前打断（true = 提前返回，调用方应跳过后续重启等动作）。
//
// 典型用法（崩溃自动重启）：
//
//	if svcutil.Delay(5*time.Second, m.isStopping) {
//	    return // 关闭中，不再重启
//	}
//
// 相比原写法 "time.Sleep(5s) 之后再查 stopping"，Delay 避免了关闭时白等整个延迟。
func Delay(d time.Duration, stop func() bool) bool {
	if d <= 0 {
		return stop != nil && stop()
	}
	if stop == nil {
		time.Sleep(d)
		return false
	}

	deadline := time.NewTimer(d)
	defer deadline.Stop()
	tick := time.NewTicker(delayTick)
	defer tick.Stop()

	for {
		select {
		case <-deadline.C:
			return false
		case <-tick.C:
			if stop() {
				return true
			}
		}
	}
}
