package http2

import (
	"context"
	"time"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/internal/bytesconv"
)

// stream 表示一个流。这是服务器协程所需的最小元数据。
// 实际上大部分 stream state 由 responseWriter 中 http.Handler 的协程所拥有。
// 因为 responseWriter 的 responseWriterState 在处理程序结束后被回收，故这个结构体故意没有
// 指向 *responseWriter{,State} 本身的指针，因为处理程序会将 responseWriter 的状态字段置空。
type stream struct {
	// immutable:
	sc      *serverConn
	id      uint32
	body    *pipe       // non-nil if expecting DATA frames
	cw      closeWaiter // closed wait stream transitions to closed state
	reqCtx  *app.RequestContext
	baseCtx context.Context

	// owned by serverConn's serve loop:
	bodyBytes        int64 // body bytes seen so far
	declBodyBytes    int64 // or -1 if undeclared
	flow             flow  // limits writing from Handler to client
	inflow           flow  // what the client is allowed to POST/etc to us
	state            streamState
	resetQueued      bool        // RST_STREAM queued for write; set by sc.resetStream
	gotTrailerHeader bool        // HEADER frame for trailers was seen
	wroteHeaders     bool        // whether we wrote headers (not status 100)
	writeDeadline    *time.Timer // nil if unused
	rw               *responseWriter
	handler          app.HandlerFunc

	trailer []trailerKV
}

type trailerKV struct {
	key   string
	value string
}

func (st *stream) processTrailerHeaders(f *MetaHeadersFrame) error {
	sc := st.sc
	sc.serveG.check()
	if st.gotTrailerHeader {
		return streamError(st.id, ErrCodeProtocol)
	}
	st.gotTrailerHeader = true
	if !f.StreamEnded() {
		return streamError(st.id, ErrCodeProtocol)
	}

	if len(f.PseudoFields()) > 0 {
		return streamError(st.id, ErrCodeProtocol)
	}

	if st.trailer == nil {
		st.trailer = make([]trailerKV, 0, len(f.RegularFields()))
	}
	for _, hf := range f.RegularFields() {
		key := st.sc.canonicalHeader(hf.Name)
		st.trailer = append(st.trailer, trailerKV{key, hf.Value})
	}

	st.endStream()
	return nil
}

func (st *stream) copyTrailer() {
	for _, kv := range st.trailer {
		st.reqCtx.Request.Header.Trailer().UpdateArgBytes(bytesconv.S2b(kv.key), bytesconv.S2b(kv.value))
	}
}
