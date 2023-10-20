package main

import (
	"context"
	"log"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/extension/sse"
)

// Server 用于保存当前已连接的客户端列表并广播事件给这些客户端。
type Server struct {
	// 该事件由主事件收集协程推送至此
	Price chan sse.Event

	// 新的客户端连接
	NewClients chan chan sse.Event

	// 已关闭的客户端连接
	ClosedClients chan chan sse.Event

	// 全部客户端
	TotalClients map[chan sse.Event]bool
}

// 监听客户端的所有请求。
// 处理客户端的新增及移除并广播消息至客户端。
func (srv *Server) listen() {
	for {
		select {
		// 加入新的可用客户端
		case client := <-srv.NewClients:
			srv.TotalClients[client] = true
			log.Printf("客户端已加入。共 %d 个已注册客户端。", len(srv.TotalClients))

			// 移除已关闭的客户端
		case client := <-srv.ClosedClients:
			delete(srv.TotalClients, client)
			close(client)
			log.Printf("客户端已移除。共 %d 个已注册客户端。", len(srv.TotalClients))

			// 广播消息至全部客户端
		case event := <-srv.Price:
			for clientMessageChan := range srv.TotalClients {
				clientMessageChan <- event
			}
		}
	}
}

// 负责登记客户端的进进出出。
func (srv *Server) serveHTTP() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		// 初始化客户端通道
		clientChan := make(ClientChan)

		// 发送新的客户端连接至事件服务器
		srv.NewClients <- clientChan

		defer func() {
			// 发送已关闭的连接至事件服务器
			srv.ClosedClients <- clientChan
		}()

		c.Set("clientChan", clientChan)

		c.Next(ctx)
	}
}

func NewServer() (srv *Server) {
	srv = &Server{
		Price:         make(chan sse.Event),
		NewClients:    make(chan chan sse.Event),
		ClosedClients: make(chan chan sse.Event),
		TotalClients:  make(map[chan sse.Event]bool),
	}

	go srv.listen()

	return
}
