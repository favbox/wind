package stats

import (
	"context"
	"runtime/debug"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/common/tracer"
	"github.com/favbox/wind/common/tracer/stats"
	"github.com/favbox/wind/common/wlog"
)

// Controller 用于控制跟踪器。
type Controller struct {
	tracers []tracer.Tracer
}

// Append 追加一个新的跟踪器到控制器。
func (ctl *Controller) Append(col tracer.Tracer) {
	ctl.tracers = append(ctl.tracers, col)
}

// DoStart 启动跟踪器。
func (ctl *Controller) DoStart(ctx context.Context, c *app.RequestContext) context.Context {
	defer ctl.tryRecover()
	Record(c.GetTraceInfo(), stats.HTTPStart, nil)

	for _, col := range ctl.tracers {
		ctx = col.Start(ctx, c)
	}
	return ctx
}

// DoFinish 以相反的顺序调用跟踪器。
func (ctl *Controller) DoFinish(ctx context.Context, c *app.RequestContext, err error) {
	defer ctl.tryRecover()
	Record(c.GetTraceInfo(), stats.HTTPFinish, err)
	if err != nil {
		c.GetTraceInfo().Stats().SetError(err)
	}

	// 倒序执行
	for i := len(ctl.tracers) - 1; i >= 0; i-- {
		ctl.tracers[i].Finish(ctx, c)
	}
}

// HasTracer 是否有跟踪器？
func (ctl *Controller) HasTracer() bool {
	return ctl != nil && len(ctl.tracers) > 0
}

func (ctl *Controller) tryRecover() {
	if err := recover(); err != nil {
		wlog.SystemLogger().Warnf("在调用跟踪器时出现恐慌。这不影响 http 调用，但可能丢失度量指标和日志等监控数据：%s, %s", err, string(debug.Stack()))
	}
}
