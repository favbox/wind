package ut

import (
	"io"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route"
)

// CreateUtRequestContext 创建一个用于测试的请求上下文。
func CreateUtRequestContext(method, url string, body *Body, headers ...Header) *app.RequestContext {
	engine := route.NewEngine(config.NewOptions(nil))
	return createUtRequestContext(engine, method, url, body, headers...)
}

func createUtRequestContext(engine *route.Engine, method, url string, body *Body, headers ...Header) *app.RequestContext {
	ctx := engine.NewContext()

	var r *protocol.Request
	if body != nil && body.Body != nil {
		r = protocol.NewRequest(method, url, body.Body)
		r.CopyTo(&ctx.Request)
		if engine.IsStreamRequestBody() || body.Len == -1 {
			ctx.Request.SetBodyStream(body.Body, body.Len)
		} else {
			buf, err := io.ReadAll(&io.LimitedReader{R: body.Body, N: int64(body.Len)})
			ctx.Request.SetBody(buf)
			if err != nil && err != io.EOF {
				panic(err)
			}
		}
	} else {
		r = protocol.NewRequest(method, url, nil)
		r.CopyTo(&ctx.Request)
	}

	for _, v := range headers {
		// 不为空就追加
		if ctx.Request.Header.Get(v.Key) != "" {
			ctx.Request.Header.Add(v.Key, v.Value)
		} else {
			// 为空就是第一次设置
			ctx.Request.Header.Set(v.Key, v.Value)
		}
	}

	return ctx
}
