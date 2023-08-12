package ut

import (
	"context"
	"io"

	"github.com/favbox/wind/route"
)

// Header 表明一个 http 标头的键值对。
type Header struct {
	Key   string
	Value string
}

// Body 用于设置 Request.Body
type Body struct {
	Body io.Reader
	Len  int
}

// PerformRequest 发送一个构造好的请求至给定引擎（无需网络传输）。
//
// url 可以是标准的相对路径，也可以是绝对路径
//
// 若引擎 engine.IsStreamRequestBody() 可流式处理请求正文，则设置正文为 bodyStream
// 否则，设置正文为 bodyBytes
//
// 返回的 ResponseRecorder 已被刷新写入，亦即它的状态码始终被置为 200。
//
// 查看 ./request_test.go 了解更多示例。
func PerformRequest(engine *route.Engine, method, url string, body *Body, headers ...Header) *ResponseRecorder {
	ctx := createUtRequestContext(engine, method, url, body, headers...)
	engine.ServeHTTP(context.Background(), ctx)

	w := NewRecorder()
	h := w.Header()
	ctx.Response.Header.CopyTo(h)

	w.WriteHeader(ctx.Response.StatusCode())
	w.Write(ctx.Response.Body())
	w.Flush()

	return w
}
