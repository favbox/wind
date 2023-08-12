package timer

import (
	"sync"
	"time"
)

var timerPool sync.Pool

// AcquireTimer 从池中取空计时器，。
func AcquireTimer(timeout time.Duration) *time.Timer {
	v := timerPool.Get()
	if v == nil {
		return time.NewTimer(timeout)
	}
	t := v.(*time.Timer)
	initTimer(t, timeout)
	return t
}

// ReleaseTimer 将 AcquireTimer 取出的 *time.Timer 放回池中。 放后勿碰，以防竞争。
func ReleaseTimer(t *time.Timer) {
	stopTimer(t)
	timerPool.Put(t)
}

// 初始化计时器，并在超时后更新它以便在其通道上发送当前时间。
func initTimer(t *time.Timer, timeout time.Duration) *time.Timer {
	if t == nil {
		return time.NewTimer(timeout)
	}
	if t.Reset(timeout) {
		panic("BUG: 活动计时器被困在 initTimer() 中了")
	}
	return t
}

// 为确保 Stop 后通道为空，检查返回值并排空通道。
func stopTimer(t *time.Timer) {
	if !t.Stop() {
		// 如果计时器已停止但无人收集其值，则从其通道收集可能添加的时间。
		select {
		case <-t.C:
		default:
		}
	}
}
