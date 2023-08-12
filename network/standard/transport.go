package standard

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/network"
)

type transport struct {
	// 请求读取的每个连接缓冲区大小
	// 也用于限制标头的最大尺寸。
	//
	// 如需客户端发送大 KB 的 RequestURI 或 标头（如很大的 cookie）
	// 请增加次缓冲区大小。
	//
	// 若未设置则使用默认缓冲大小。
	readBufferSize   int
	network          string
	addr             string
	keepAliveTimeout time.Duration
	readTimeout      time.Duration
	handler          network.OnData
	ln               net.Listener
	tls              *tls.Config
	listenConfig     *net.ListenConfig
	lock             sync.Mutex
	OnAccept         func(conn net.Conn) context.Context
	OnConnect        func(ctx context.Context, conn network.Conn) context.Context
}

func (t *transport) ListenAndServe(onData network.OnData) error {
	t.handler = onData
	return t.serve()
}

func (t *transport) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	return t.Shutdown(ctx)
}

func (t *transport) Shutdown(ctx context.Context) error {
	defer func() {
		network.UnlinkUdsFile(t.network, t.addr)
	}()

	t.lock.Lock()
	if t.ln != nil {
		_ = t.ln.Close()
	}
	t.lock.Unlock()
	<-ctx.Done()
	return nil
}

func (t *transport) serve() (err error) {
	_ = network.UnlinkUdsFile(t.network, t.addr)
	t.lock.Lock()
	if t.listenConfig != nil {
		t.ln, err = t.listenConfig.Listen(context.Background(), t.network, t.addr)
	} else {
		t.ln, err = net.Listen(t.network, t.addr)
	}
	t.lock.Unlock()
	if err != nil {
		return err
	}
	wlog.SystemLogger().Infof("HTTP服务器监听地址=%s", t.ln.Addr().String())
	for {
		ctx := context.Background()
		conn, err := t.ln.Accept()
		if err != nil {
			wlog.SystemLogger().Errorf("错误=%s", err.Error())
			return err
		}

		if t.OnAccept != nil {
			ctx = t.OnAccept(conn)
		}

		var c network.Conn
		if t.tls != nil {
			c = newTLSConn(tls.Server(conn, t.tls), t.readBufferSize)
		} else {
			c = newConn(conn, t.readBufferSize)
		}

		if t.OnConnect != nil {
			ctx = t.OnConnect(ctx, c)
		}
		go t.handler(ctx, c)
	}
}

// NewTransporter 创建标准库网络传输器。
func NewTransporter(options *config.Options) network.Transporter {
	return &transport{
		readBufferSize:   options.ReadBufferSize,
		network:          options.Network,
		addr:             options.Addr,
		keepAliveTimeout: options.KeepAliveTimeout,
		readTimeout:      options.ReadTimeout,
		tls:              options.TLS,
		listenConfig:     options.ListenConfig,
		OnAccept:         options.OnAccept,
		OnConnect:        options.OnConnect,
	}
}
