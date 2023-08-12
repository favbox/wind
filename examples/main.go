package main

import (
	"context"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/app/server"
	"github.com/favbox/wind/protocol/consts"
)

func main() {
	wind := server.Default(server.WithHostPorts("0.0.0.0:8888"))
	wind.GET("/", func(c context.Context, ctx *app.RequestContext) {
		ctx.String(consts.StatusOK, "hello, world!")
	})
	wind.Spin()
}
