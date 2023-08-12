package traceinfo

import (
	"sync"
	"time"

	"github.com/favbox/gosky/wind/pkg/common/tracer/stats"
)

var eventPool sync.Pool

func init() {
	eventPool.New = newEvent
}

// Event 是发生于特定时间的事件。
type Event interface {
	Event() stats.Event
	Status() stats.Status
	Info() string
	Time() time.Time
	IsNil() bool
}

type event struct {
	event  stats.Event
	status stats.Status
	info   string
	time   time.Time
}

func (e *event) Event() stats.Event {
	return e.event
}

func (e *event) Status() stats.Status {
	return e.status
}

func (e *event) Info() string {
	return e.info
}

func (e *event) Time() time.Time {
	return e.time
}

func (e *event) IsNil() bool {
	return e == nil
}

func (e *event) Recycle() {
	e.zero()
	eventPool.Put(e)
}

func (e *event) zero() {
	e.event = nil
	e.status = 0
	e.info = ""
	e.time = time.Time{}
}

func newEvent() any {
	return &event{}
}
