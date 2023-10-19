package sse

import (
	"github.com/favbox/wind/app"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol/http1/resp"
)

const (
	ContentType  = "text/event-stream"
	noCache      = "no-cache"
	cacheControl = "Cache-Control"
	LastEventID  = "Last-Event-ID"
)

type Event struct {
	Event string
	ID    string
	Retry uint64
	Data  []byte
}

// GetLastEventID 获取请求头中可能存在的 Last-Event-ID 值。
func GetLastEventID(c *app.RequestContext) string {
	return c.Request.Header.Get(LastEventID)
}

type Stream struct {
	w network.ExtWriter
}

// NewStream 为发布事件创建一个新的流。
func NewStream(c *app.RequestContext) *Stream {
	c.Response.Header.SetContentType(ContentType)
	if c.Response.Header.Get(cacheControl) == "" {
		c.Response.Header.Set(cacheControl, noCache)
	}

	writer := resp.NewChunkedBodyWriter(&c.Response, c.GetWriter())
	c.Response.HijackWriter(writer)
	return &Stream{writer}
}

// Publish 发布事件至客户端。
func (s *Stream) Publish(event *Event) error {
	err := Encode(s.w, event)
	if err != nil {
		return err
	}
	return s.w.Flush()
}
