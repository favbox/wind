package http2

import (
	"bufio"
	"net/http"
	"sync"
)

// responseWriter 是 http.ResponseWriter 的实现。
// 它被有意设计得很小（仅1个指针宽度）以尽量减少垃圾回收。
// 在请求结束时（在 handlerDone 中），内部的 responseWriterState 指针被清零，
// 此后对 responseWriter 的调用会导致崩溃（调用者的错误），但更大的
// responseWriterState 和缓冲区在多个请求之间被重用。
type responseWriter struct {
	rws *responseWriterState
}

// 可选。确保 http.ResponseWriter 接口被实现。
var (
	_ http.Flusher = (*responseWriter)(nil)
	_ stringWriter = (*responseWriter)(nil)
)

type responseWriterState struct {
	// immutable within a request:
	stream *stream
	body   *requestBody // to close at end of request, if DATA frames didn't
	conn   *serverConn

	// TODO: adjust buffer writing sizes based on server config, frame size updates from peer, etc
	bw *bufio.Writer // writing to a chunkWriter{this *responseWriterState}

	status      int  // status code passed to WriteHeaders
	wroteHeader bool // WriteHeaders called (explicitly or implicitly). Not necessarily sent to user yet.
	sentHeader  bool // have we sent the header frame?
	handlerDone bool // handler has finished
	dirty       bool // a Write failed; don't reuse this responseWriterState

	sentContentLen int64 // non-zero if handler set a Content-Length header
	wroteBytes     int64

	closeNotifierMu sync.Mutex // guards closeNotifierCh
	closeNotifierCh chan bool  // nil until first used
}
