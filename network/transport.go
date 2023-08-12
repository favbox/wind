package network

import "context"

// Transporter 表示网络传输层接口。
type Transporter interface {
	// ListenAndServe 监听并准备接收连接。
	ListenAndServe(OnData) error

	// Close 立即关闭传输器。
	Close() error

	// Shutdown 平滑关闭传输器。
	Shutdown(ctx context.Context) error
}

// OnData 连接数据(如客户端请求数据)准备完毕时的回调函数。
type OnData func(ctx context.Context, conn any) error
