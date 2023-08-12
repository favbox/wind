package network

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"time"
)

// Reader 用于缓冲读取。
type Reader interface {
	// Len 返回可读数据总长度。
	Len() int

	// Peek 返回 n 个字节，但不移动指针。
	Peek(n int) ([]byte, error)

	// Skip 跳过 n 个字节。
	Skip(n int) error

	// ReadByte 读取 1 个字节，并移动指针。
	ReadByte() (byte, error)

	// ReadBinary 读取 n 个字节，并移动指针。
	ReadBinary(n int) (p []byte, err error)

	// Release 释放所有读取切片占用的内存。
	//
	// 在确认先前读取的数据不再使用后，需要主动执行该方法来回收内存。
	//
	// 调用 Release 后，通过 Peek 等方法获取的切片将成为无效地址，无法再使用。
	Release() error
}

// Writer 用于缓冲写入。
type Writer interface {
	// Malloc 分配一块 n 字节的内存缓冲区来暂存数据。
	Malloc(n int) (buf []byte, err error)

	// WriteBinary 向用户缓冲区写入字节切片。注意：在成功刷新之前，b 应有效。
	WriteBinary(b []byte) (n int, err error)

	// Flush 向对端发送数据。
	Flush() error
}

// ReadWriter 适用于缓冲读取器和写入器。
type ReadWriter interface {
	Reader
	Writer
}

// Conn 表示普通读写的连接。
type Conn interface {
	net.Conn
	Reader
	Writer

	// SetReadTimeout 设置每个连接读取进程的超时时长
	SetReadTimeout(t time.Duration) error
	// SetWriteTimeout 设置每个连接写入进程的超时时长
	SetWriteTimeout(t time.Duration) error
}

// ConnTLSer 表示安全读写的连接。
type ConnTLSer interface {
	Handshake() error
	ConnectionState() tls.ConnectionState
}

// HandleSpecificError 表示特定错误的处理程序。
type HandleSpecificError interface {
	HandleSpecificError(err error, remoteIP string) (needIgnore bool)
}

// ErrorNormalization 表示错误的规范化程序。
type ErrorNormalization interface {
	// ToWindError 将 err 转为 wind 错误。
	ToWindError(err error) error
}

// DialFunc 定义拨打给定网址返回对应连接的拨号函数。
type DialFunc func(addr string) (Conn, error)

/****************** 基于流的网络连接 ******************/

// StreamConn 表示流式读写的连接。
type StreamConn interface {
	GetRawConnection() any
	// HandshakeComplete 阻塞至握手完成（或失败）。
	HandshakeComplete() context.Context
	// GetVersion 返回用于连接的协议版本。
	GetVersion() uint32
	// CloseWithError 关闭出错的连接。错误字符串将发给对端。
	CloseWithError(err ApplicationError, errMsg string) error
	// LocalAddr 返回本地地址。
	LocalAddr() net.Addr
	// RemoteAddr 返回对端地址。
	RemoteAddr() net.Addr
	// Context 当连接关闭时，上下文将被取消。
	Context() context.Context
	// Streamer 是流式操作的接口。
	Streamer
}

// Streamer 表示流操作的接口。
type Streamer interface {
	// AcceptStream 返回对端打开的下一个流。
	// 它会阻塞，直至有可用流为止。
	// 若连接因超时而关闭，则返回 net.Error 并将 Timeout() 置为 true。
	AcceptStream(ctx context.Context) (Stream, error)

	// AcceptUniStream 返回对端打开的下一个单向流。
	// 它会阻塞，直至有可用流为止。
	// 若连接因超时而关闭，则返回 net.Error 并将 Timeout() 置为 true。
	AcceptUniStream(ctx context.Context) (ReceiveStream, error)

	// OpenStream 打开一个新的双向 QUIC 流。
	// 仅当数据已在流上发送后，对端才能接收该流。
	// 若出错，则满足 net.Error 接口。
	// 当达到对端的流限制时，err.Temporary() 将为 true。
	// 若连接因超时而关闭，则 Timeout() 将为 true。
	OpenStream() (Stream, error)

	// OpenStreamSync 打开一个新的双向 QUIC 流（阻塞式）。
	// 它会阻塞，直至有新流能被打开为止。
	// 若出错，则满足 net.Error 接口。
	// 若因连接超时而关闭，则 Timeout() 将为 true。
	OpenStreamSync() (Stream, error)

	// OpenUniStream 打开一个新的单向 QUIC 流。
	// 若出错，则满足 net.Error 接口。
	// 当达到对端的流限制时，err.Temporary() 将为 true。
	// 若连接因超时而关闭，则 Timeout() 将为 true。
	OpenUniStream() (SendStream, error)

	// OpenUniStreamSync 打开一个新的单向 QUIC 流（阻塞式）。
	// 它会阻塞，直至有新流能被打开为止。
	// 若出错，则满足 net.Error 接口。
	// 若连接因超时而关闭，则 Timeout() 将为 true。
	OpenUniStreamSync(ctx context.Context) (SendStream, error)
}

// Stream 表示接收流和发送流操作的接口。
type Stream interface {
	ReceiveStream
	SendStream
}

// ReceiveStream 接收流，是在流上接收数据的接口。
type ReceiveStream interface {
	StreamID() int64
	io.Reader

	// CancelRead 中断在流上的接收。
	// 它会要求对端停止传输流数据。
	// Read 将立即解锁，以后的 Read 调用将失败。
	// 当多次调用或读到 io.EOF 后会执行 no-op 空操作。
	CancelRead(err ApplicationError)

	// SetReadDeadline 设置未来读取调用和当前已阻塞读取调用的截止时间。
	// t 的零值意为读取不会超时。
	SetReadDeadline(t time.Time) error
}

// SendStream 发送流，是在流上发送数据的接口。
type SendStream interface {
	StreamID() int64
	// Writer 将数据写入流。
	// 支持超时退出并返回 net.Error，详见 SetDeadline 和 SetWriteDeadline。
	// 如果流被对端取消，则返回错误将实现 StreamError 接口且 Canceled() == true。
	io.Writer

	// CancelWrite 中断在流上的发送。
	// 已发送但未到达对端的数据不保证能可靠交付。
	// Write 将立即解锁，以后的 Write 调用将失败。
	// 当多次调用或流已关闭会执行 no-op 空操作。
	CancelWrite(err ApplicationError)

	// Closer 关闭写方向的流。
	// 调用 Close 后，不可再调用 Write，两者也不可同时调用。
	// 调用 CancelWrite 后，也不可再调用 Close。
	io.Closer

	// Context 一旦流的写入端关闭，上下文就会被取消。
	// 当调用写入端的 Close() 或 CancelWrite()，以及对端取消读取流的时候，会发生上述情况。
	Context() context.Context

	// SetWriteDeadline 设置未来写入调用和当前已阻塞写入调用的截止时间。
	// 即使写入超时也可能返回 n > 0，表明部分数据已写入成功。
	// t 的零值意为写入不会超时。
	SetWriteDeadline(t time.Time) error
}

type ApplicationError interface {
	ErrCode() uint64
	fmt.Stringer
}
