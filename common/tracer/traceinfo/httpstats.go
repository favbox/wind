package traceinfo

import (
	"sync"
	"time"

	"github.com/favbox/gosky/wind/pkg/common/tracer/stats"
)

var (
	once        sync.Once
	maxEventNum int
)

// HTTPStats 用于收集有关 HTTP 的统计信息。
type HTTPStats interface {
	Record(event stats.Event, status stats.Status, info string)
	GetEvent(event stats.Event) Event
	SendSize() int
	SetSendSize(size int)
	RecvSize() int
	SetRecvSize(size int)
	Error() error
	SetError(err error)
	Panicked() (bool, any)
	SetPanicked(x any)
	Level() stats.Level
	SetLevel(level stats.Level)
	Reset()
}

type httpStats struct {
	sync.RWMutex
	level stats.Level

	events []Event

	sendSize int
	recvSize int

	err      error
	panicErr any
}

func (h *httpStats) Record(e stats.Event, status stats.Status, info string) {
	if e.Level() > h.level {
		return
	}
	evt := eventPool.Get().(*event)
	evt.event = e
	evt.status = status
	evt.info = info
	evt.time = time.Now()

	idx := e.Index()
	h.Lock()
	h.events[idx] = evt
	h.Unlock()
}

func (h *httpStats) GetEvent(e stats.Event) Event {
	idx := e.Index()
	h.RLock()
	evt := h.events[idx]
	h.RUnlock()
	if evt == nil || evt.IsNil() {
		return nil
	}
	return evt
}

func (h *httpStats) SendSize() int {
	return h.sendSize
}

func (h *httpStats) SetSendSize(size int) {
	h.sendSize = size
}

func (h *httpStats) RecvSize() int {
	return h.recvSize
}

func (h *httpStats) SetRecvSize(size int) {
	h.recvSize = size
}

func (h *httpStats) Error() error {
	return h.err
}

func (h *httpStats) SetError(err error) {
	h.err = err
}

func (h *httpStats) Panicked() (bool, any) {
	return h.panicErr != nil, h.panicErr
}

func (h *httpStats) SetPanicked(x any) {
	h.panicErr = x
}

func (h *httpStats) Level() stats.Level {
	return h.level
}

func (h *httpStats) SetLevel(level stats.Level) {
	h.level = level
}

func (h *httpStats) Reset() {
	h.err = nil
	h.panicErr = nil
	h.recvSize = 0
	h.sendSize = 0
	for i := range h.events {
		if h.events[i] != nil {
			h.events[i].(*event).Recycle()
			h.events[i] = nil
		}
	}
}

// ImmutableView 限制为只读模式。
func (h *httpStats) ImmutableView() HTTPStats {
	return h
}

// NewHTTPStats 创建新的 HTTP 统计采集器。
func NewHTTPStats() HTTPStats {
	once.Do(func() {
		stats.FinishInitialization()
		maxEventNum = stats.MaxEventNum()
	})
	return &httpStats{
		// 基于定义的事件个数，精准创建事件切片
		events: make([]Event, maxEventNum),
	}
}
