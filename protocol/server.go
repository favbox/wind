package protocol

import (
	"context"

	"github.com/favbox/wind/network"
)

// Server 定义普通服务器接口，需实现连接的 Serve 方法。
type Server interface {
	// Serve 提供 network.Conn 服务。
	Serve(ctx context.Context, conn network.Conn) error
}

// StreamServer 定义流式服务器接口，需实现连接的 Serve 方法。
type StreamServer interface {
	// Serve 提供 network.StreamConn 服务。
	Serve(ctx context.Context, conn network.StreamConn) error
}
