package main

import (
	"context"
	"net/http"
	"time"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/app/server"
	"github.com/favbox/wind/common/json"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/extension/sse"
)

// ChatServer 实现一个聊天服务器以演示 wind 中服务器发送事件的用法。
// 支持消息单发和群发。
type ChatServer struct {
	// 群发消息通道
	BroadcastMessageC chan ChatMessage

	// 单发消息通道
	DirectMessageC chan ChatMessage

	// 每个用户一个的聊天消息通道
	Receive map[string]chan ChatMessage
}

type ChatMessage struct {
	Type      string
	From      string
	To        string
	Message   string
	Timestamp time.Time
}

func main() {
	w := server.Default()

	srv := NewServer()

	// 确保用户有接收通道
	w.Use(srv.CreateReceiveChannel())

	// 服务端阻塞式等待消息，并向用户客户端发送流式更新
	w.GET("/chat/sse", srv.ServerSentEvent)

	// 单发消息
	w.GET("/chat/direct", srv.Direct)

	// 群发消息
	w.GET("/chat/broadcast", srv.Broadcast)

	w.Spin()
}

func NewServer() (srv *ChatServer) {
	srv = &ChatServer{
		BroadcastMessageC: make(chan ChatMessage),
		DirectMessageC:    make(chan ChatMessage),
		Receive:           make(map[string]chan ChatMessage),
	}

	go srv.relay()

	return
}

func (srv *ChatServer) CreateReceiveChannel() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		username := c.Query("username")
		// 若用户尚无通道则，先创建一个
		if _, found := srv.Receive[username]; !found {
			receive := make(chan ChatMessage, 100)
			srv.Receive[username] = receive
		}
		c.Next(ctx)
	}
}

func (srv *ChatServer) ServerSentEvent(ctx context.Context, c *app.RequestContext) {
	// 生产环境，应使用其他方式如 Authorization 获取用户的标识
	username := c.Query("username")

	stream := sse.NewStream(c)
	for msg := range srv.Receive[username] {
		payload, err := json.Marshal(msg)
		if err != nil {
			c.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		wlog.CtxInfof(ctx, "收到消息：%+v", msg)
		event := &sse.Event{
			Event: msg.Type,
			Data:  payload,
		}
		c.SetStatusCode(http.StatusOK)
		err = stream.Publish(event)
		if err != nil {
			return
		}
	}
}

func (srv *ChatServer) relay() {
	for {
		select {
		// 接收群发消息，然后发给所有用户
		case msg := <-srv.BroadcastMessageC:
			for _, r := range srv.Receive {
				r <- msg
			}
		// 接收单发消息，然后发给指定用户
		case msg := <-srv.DirectMessageC:
			srv.Receive[msg.To] <- msg
		}
	}
}

func (srv *ChatServer) Direct(ctx context.Context, c *app.RequestContext) {
	// 生产环境，应使用其他方式如 Authorization 获取用户的标识
	from := c.Query("username")
	to := c.Query("to")
	message := c.Query("message")

	msg := ChatMessage{
		Type:      "direct",
		From:      from,
		To:        to,
		Message:   message,
		Timestamp: time.Now(),
	}
	// 发送消息至群发通道
	srv.DirectMessageC <- msg

	wlog.CtxInfof(ctx, "发送消息：%+v", msg)
	c.JSON(http.StatusOK, utils.H{
		"message": "success",
	})
}

func (srv *ChatServer) Broadcast(ctx context.Context, c *app.RequestContext) {
	// 生产环境，应使用其他方式如 Authorization 获取用户的标识
	from := c.Query("username")
	message := c.Query("message")

	msg := ChatMessage{
		Type:      "broadcast",
		From:      from,
		Message:   message,
		Timestamp: time.Now(),
	}
	// 发送消息至单发通道
	srv.BroadcastMessageC <- msg

	wlog.CtxInfof(ctx, "发送消息：%+v", msg)
	c.JSON(http.StatusOK, utils.H{
		"message": "success",
	})
}
