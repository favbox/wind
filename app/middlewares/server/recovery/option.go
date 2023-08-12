package recovery

import (
	"context"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/protocol/consts"
)

// 表示一个恐慌恢复的自定义选项结构体。
type options struct {
	// 恐慌恢复处理器。
	recoveryHandler func(c context.Context, ctx *app.RequestContext, err any, stack []byte)
}

// Option 自定义选项的应用函数。
type Option func(o *options)

// 默认的恐慌恢复处理器。
func defaultRecoveryHandler(c context.Context, ctx *app.RequestContext, err any, stack []byte) {
	wlog.SystemLogger().CtxErrorf(c, "[恐慌恢复] 恐慌=%v\n堆栈=%s", err, stack)
	ctx.AbortWithStatus(consts.StatusInternalServerError)
}

// 创建一个自定义恐慌恢复的结构，并应用自定义选项。
func newOptions(opts ...Option) *options {
	cfg := &options{recoveryHandler: defaultRecoveryHandler}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// WithRecoveryHandler 自定义恐慌恢复处理器。
func WithRecoveryHandler(f func(c context.Context, ctx *app.RequestContext, err any, stack []byte)) Option {
	return func(o *options) {
		o.recoveryHandler = f
	}
}
