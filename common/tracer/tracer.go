package tracer

import (
	"context"

	"github.com/favbox/wind/app"
)

// Tracer 在 HTTP 开始和结束时执行。
type Tracer interface {
	Start(ctx context.Context, c *app.RequestContext) context.Context
	Finish(ctx context.Context, c *app.RequestContext)
}

// Controller 跟踪控制器
type Controller interface {
	// Append 追加一个追踪器。
	Append(col Tracer)
	// DoStart 启动跟踪器。
	DoStart(ctx context.Context, c *app.RequestContext) context.Context
	// DoFinish 以相反的顺序调用跟踪器。
	DoFinish(ctx context.Context, c *app.RequestContext, err error)
	// HasTracer 是否有跟踪器？
	HasTracer() bool
}
