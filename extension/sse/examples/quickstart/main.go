package main

import (
	"context"
	"time"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/app/server"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/extension/sse"
	"github.com/favbox/wind/protocol/consts"
)

func main() {
	w := server.Default()

	w.GET("/sse", func(ctx context.Context, c *app.RequestContext) {
		// 客户端可用 Last-Event-ID 标头告知服务端它最后收到的事件
		lastEventID := sse.GetLastEventID(c)
		wlog.CtxInfof(ctx, "上次事件编号：%s", lastEventID)

		// 你必须在首次调用渲染前设置状态码和响应头
		c.SetStatusCode(consts.StatusOK)
		s := sse.NewStream(c)
		for t := range time.NewTicker(1 * time.Second).C {
			event := &sse.Event{
				Event: "timestamp",
				Data:  []byte(t.Format(time.DateTime)),
			}
			err := s.Publish(event)
			if err != nil {
				return
			}
		}
	})

	w.Spin()
}
