package main

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bytedance/gopkg/lang/fastrand"
	"github.com/favbox/wind/app"
	"github.com/favbox/wind/app/server"
	"github.com/favbox/wind/extension/sse"
)

// ClientChan 待广播至所有已注册客户端连接通道中的事件消息。
type ClientChan chan sse.Event

func main() {
	w := server.Default()

	// 初始化新的流式服务器
	srv := NewServer()

	// 增加：每秒一次
	go func() {
		for {
			time.Sleep(1 * time.Second)
			now := time.Now()
			for _, stock := range []string{"AAPL", "AMZN"} {
				// 发送当前时间至客户端消息通道
				srv.Price <- sse.Event{
					Event: stock,
					ID:    strconv.FormatInt(now.UnixMilli(), 10),
					Data:  []byte(fmt.Sprintf("%f", fastrand.Float64()*100)),
				}
			}
		}
	}()

	// 已授权的客户端可流式发送事件
	w.GET("/price", srv.serveHTTP(), func(ctx context.Context, c *app.RequestContext) {
		v, ok := c.Get("clientChan")
		if !ok {
			return
		}
		clientChan, ok := v.(ClientChan)
		if !ok {
			return
		}
		c.SetStatusCode(http.StatusOK)

		// 劫持响应编写器，将编码后的消息写入客户端
		stream := sse.NewStream(c)
		for event := range clientChan {
			_ = stream.Publish(&event)
		}
	})

	w.Spin()
}
