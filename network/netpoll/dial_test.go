package netpoll

import (
	"context"
	"crypto/tls"
	"testing"
	"time"

	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/mock"
	"github.com/stretchr/testify/assert"
)

func TestDial(t *testing.T) {
	t.Run("NetpollDial", func(t *testing.T) {
		const nw = "tcp"
		const addr = "localhost:10100"
		transporter := NewTransporter(&config.Options{
			Addr:    addr,
			Network: nw,
		})
		go transporter.ListenAndServe(func(ctx context.Context, conn interface{}) error {
			return nil
		})
		defer transporter.Close()
		time.Sleep(100 * time.Millisecond)

		// 新建拨号器
		dial := NewDialer()

		// 错误地址，拨不通
		_, err := dial.DialConnection("tcp", "localhost:10101", time.Second, nil) // wrong addr
		assert.NotNil(t, err)

		// 正确地址，拨的通
		nwConn, err := dial.DialConnection(nw, addr, time.Second, nil)
		defer nwConn.Close()
		assert.Nil(t, err)
		_, err = nwConn.Write([]byte("abcdef"))
		assert.Nil(t, err)

		// 兼容 DialTimeout
		nConn, err := dial.DialTimeout(nw, addr, time.Second, nil)
		assert.Nil(t, err)
		defer nConn.Close()
		_, err = nConn.Write([]byte("abcdef"))
		assert.Nil(t, err)
	})

	t.Run("NetpollNotSupportTLS", func(t *testing.T) {
		dial := NewDialer()
		_, err := dial.AddTLS(mock.NewConn(""), nil)
		assert.Equal(t, errTLSNotSupported, err)
		_, err = dial.DialConnection("tcp", "localhost:10102", time.Microsecond, &tls.Config{})
		assert.Equal(t, errTLSNotSupported, err)
		_, err = dial.DialTimeout("tcp", "localhost:10102", time.Microsecond, &tls.Config{})
		assert.Equal(t, errTLSNotSupported, err)
	})
}
