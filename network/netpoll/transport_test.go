package netpoll

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/network"
	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestTransport(t *testing.T) {
	t.Parallel()

	const nw = "tcp"
	const addr = "localhost:10103"

	t.Run("TestDefault", func(t *testing.T) {
		var onConnFlag, onAcceptFlag, onDataFlag int32
		transporter := NewTransporter(&config.Options{
			Addr:    addr,
			Network: nw,
			OnAccept: func(conn net.Conn) context.Context {
				fmt.Println("连接已接受，适合做IP黑名单验证")
				atomic.StoreInt32(&onAcceptFlag, 1)
				return context.Background()
			},
			OnConnect: func(ctx context.Context, conn network.Conn) context.Context {
				fmt.Println("连接已建立")
				atomic.StoreInt32(&onConnFlag, 1)
				return ctx
			},
			WriteTimeout: time.Second,
		})
		go transporter.ListenAndServe(func(ctx context.Context, conn any) error {
			// fmt.Println("数据已准备")
			atomic.StoreInt32(&onDataFlag, 1)
			return nil
		})
		defer transporter.Close()
		time.Sleep(100 * time.Millisecond)

		dialer := NewDialer()
		conn, err := dialer.DialConnection(nw, addr, time.Second, nil)
		assert.Nil(t, err)
		_, err = conn.Write([]byte("123"))
		assert.Nil(t, err)
		time.Sleep(100 * time.Millisecond)

		assert.True(t, atomic.LoadInt32(&onConnFlag) == 1)
		assert.True(t, atomic.LoadInt32(&onAcceptFlag) == 1)
		assert.True(t, atomic.LoadInt32(&onDataFlag) == 1)
	})

	t.Run("TestListenConfig", func(t *testing.T) {
		listenCfg := &net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEADDR, 1)
				syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			})
		}}
		transporter := NewTransporter(&config.Options{
			Addr:         addr,
			Network:      nw,
			ListenConfig: listenCfg,
		})
		go transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
			return nil
		})
		defer transporter.Close()
	})

	t.Run("TestExceptionCase", func(t *testing.T) {
		assert.Panics(t, func() { // listen err
			transporter := NewTransporter(&config.Options{
				Network: "未指定网络类型",
			})
			transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
				return nil
			})
		})
	})
}
