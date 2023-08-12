package client

import (
	"context"

	"github.com/favbox/wind/protocol"
)

// Endpoint 远程调用方法。
type Endpoint func(ctx context.Context, req *protocol.Request, resp *protocol.Response) (err error)

// Middleware 处理输入的 Endpoint 然后输出处理后的 Endpoint。
type Middleware func(Endpoint) Endpoint

// 连接一组中间件为一个中间件，以便链式调用。
func chain(mws ...Middleware) Middleware {
	return func(next Endpoint) Endpoint {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}
		return next
	}
}
