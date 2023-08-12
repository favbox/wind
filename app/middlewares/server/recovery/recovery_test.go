package recovery

import (
	"context"
	"fmt"
	"testing"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/protocol/consts"
	"github.com/stretchr/testify/assert"
)

func TestRecovery(t *testing.T) {
	ctx := app.NewContext(0)
	var hc app.HandlersChain
	hc = append(hc, func(c context.Context, ctx *app.RequestContext) {
		fmt.Println("这是测试")
		panic("测试")
	})
	ctx.SetHandlers(hc)

	Recovery()(context.Background(), ctx)

	assert.Equal(t, 500, ctx.Response.StatusCode())
}

func TestWithRecoveryHandler(t *testing.T) {
	ctx := app.NewContext(0)
	var hc app.HandlersChain
	hc = append(hc, func(c context.Context, ctx *app.RequestContext) {
		fmt.Println("这是测试")
		panic("测试")
	})
	ctx.SetHandlers(hc)

	Recovery(WithRecoveryHandler(myRecoveryHandler))(context.Background(), ctx)

	assert.Equal(t, consts.StatusNotImplemented, ctx.Response.StatusCode())
	assert.Equal(t, `{"msg":"测试"}`, string(ctx.Response.Body()))
}
