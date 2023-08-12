package netpoll

import (
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/netpoll"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/network"
)

var _ network.Transporter = (*transport)(nil)

func init() {
	// 禁用 netpoll 的日志
	netpoll.SetLoggerOutput(io.Discard)
}

type transport struct {
	sync.RWMutex
	network          string
	addr             string
	keepAliveTimeout time.Duration
	readTimeout      time.Duration
	writeTimeout     time.Duration
	listener         net.Listener
	eventLoop        netpoll.EventLoop
	listenConfig     *net.ListenConfig
	OnAccept         func(conn net.Conn) context.Context
	OnConnect        func(ctx context.Context, conn network.Conn) context.Context
}

// ListenAndServe 绑定监听地址并持续服务，除非出现错误或传输器关闭。
func (t *transport) ListenAndServe(onReq network.OnData) (err error) {
	_ = network.UnlinkUdsFile(t.network, t.addr)
	if t.listenConfig != nil {
		t.listener, err = t.listenConfig.Listen(context.Background(), t.network, t.addr)
	} else {
		t.listener, err = net.Listen(t.network, t.addr)
	}

	if err != nil {
		panic("创建 netpoll 监听器失败：" + err.Error())
	}

	// 为 EventLoop 初始化自定义选项
	opts := []netpoll.Option{
		netpoll.WithIdleTimeout(t.keepAliveTimeout),
		netpoll.WithOnPrepare(func(conn netpoll.Connection) context.Context {
			// 设置准备期间的读写超时
			_ = conn.SetReadTimeout(t.readTimeout)
			if t.writeTimeout > 0 {
				_ = conn.SetWriteTimeout(t.writeTimeout)
			}
			// 设置准备期间，连接请求被接受时的回调
			if t.OnAccept != nil {
				return t.OnAccept(newConn(conn))
			}
			return context.Background()
		}),
	}

	if t.OnConnect != nil {
		// 设置建立连接时的回调
		opts = append(opts, netpoll.WithOnConnect(func(ctx context.Context, conn netpoll.Connection) context.Context {
			return t.OnConnect(ctx, newConn(conn))
		}))
	}

	// 创建 EventLoop
	t.Lock()
	t.eventLoop, err = netpoll.NewEventLoop(func(ctx context.Context, connection netpoll.Connection) error {
		return onReq(ctx, newConn(connection))
	}, opts...)
	t.Unlock()
	if err != nil {
		panic("创建 netpoll event-loop 失败")
	}

	// 启动服务器
	wlog.SystemLogger().Infof("HTTP服务器监听地址=%s", t.listener.Addr().String())
	t.RLock()
	err = t.eventLoop.Serve(t.listener)
	t.RUnlock()
	if err != nil {
		panic("netpoll event-loop 无法启动监听服务：" + err.Error())
	}

	return nil
}

// Close 强制传输器立即关闭（无超时等待）。
func (t *transport) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	return t.Shutdown(ctx)
}

// Shutdown 停止监听器并优雅关闭。 将等待所有连接关闭，直到触达截止时间。
func (t *transport) Shutdown(ctx context.Context) error {
	defer func() {
		_ = network.UnlinkUdsFile(t.network, t.addr)
		t.RUnlock()
	}()
	t.RLock()
	if t.eventLoop == nil {
		return nil
	}
	return t.eventLoop.Shutdown(ctx)
}

// NewTransporter 创建 netpoll 网络传输器。
func NewTransporter(options *config.Options) network.Transporter {
	return &transport{
		RWMutex:          sync.RWMutex{},
		network:          options.Network,
		addr:             options.Addr,
		keepAliveTimeout: options.KeepAliveTimeout,
		readTimeout:      options.ReadTimeout,
		writeTimeout:     options.WriteTimeout,
		listener:         nil,
		eventLoop:        nil,
		listenConfig:     options.ListenConfig,
		OnAccept:         options.OnAccept,
		OnConnect:        options.OnConnect,
	}
}
