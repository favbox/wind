package ut

import (
	"bytes"

	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
)

// ResponseRecorder 记录处理器的响应以供稍后测试。
type ResponseRecorder struct {
	Code        int
	header      *protocol.ResponseHeader
	Body        *bytes.Buffer
	Flushed     bool
	result      *protocol.Response
	wroteHeader bool
}

// NewRecorder 返回一个实例化的响应记录器。
func NewRecorder() *ResponseRecorder {
	return &ResponseRecorder{
		Code:   consts.StatusOK,
		header: new(protocol.ResponseHeader),
		Body:   new(bytes.Buffer),
	}
}

// Header 返回响应标头以便在处理器中修改（mutate）。
// 要想测试在处理器完成后写入的标头，使用 Result 方法并查看返回的响应值的 Header。
func (r *ResponseRecorder) Header() *protocol.ResponseHeader {
	m := r.header
	if m == nil {
		m = new(protocol.ResponseHeader)
		r.header = m
	}
	return m
}

// Write 实现 io.Writer。缓冲数据 p 会被写入 Body。
func (r *ResponseRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(consts.StatusOK)
	}
	if r.Body != nil {
		r.Body.Write(p)
	}
	return len(p), nil
}

// WriteString 实现 io.StringWriter。将 s 写入 Body。
func (r *ResponseRecorder) WriteString(s string) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(consts.StatusOK)
	}
	if r.Body != nil {
		r.Body.WriteString(s)
	}
	return len(s), nil
}

// WriteHeader 发送给定状态码的 HTTP 响应标头。
func (r *ResponseRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	if r.header == nil {
		r.header = new(protocol.ResponseHeader)
	}
	r.header.SetStatusCode(code)
	r.Code = code
	r.wroteHeader = true
}

// Flush 实现 http.Flusher。要测试 Flush 是否已被调用，请看 Flushed。
func (r *ResponseRecorder) Flush() {
	if !r.wroteHeader {
		r.WriteHeader(consts.StatusOK)
	}
	r.Flushed = true
}

// Result 返回处理器生成的结果。
//
// 返回的响应至少填充了状态码、标头、正文和可选地挂车。
// 未来可能会填充更多的字段，因此调用方不应在测试中 DeepEqual 结果。
//
// Response.Header 是第一次写入调用时的标头快照，若处理器从未写入，则是此快照。
//
// Response.Body 保证为非零，Body.Read 调用保证不会返回 io.EOF 以外的任何错误。
//
// Result 只能在处理器完成后方可调用。
func (r *ResponseRecorder) Result() *protocol.Response {
	if r.result != nil {
		return r.result
	}

	resp := new(protocol.Response)
	h := r.Header()
	h.CopyTo(&resp.Header)
	if r.Body != nil {
		b := r.Body.Bytes()
		resp.SetBody(b)
		resp.Header.SetContentLength(len(b))
	}

	r.result = resp
	return resp
}
