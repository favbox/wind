package mock

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/cloudwego/netpoll"
	errs "github.com/favbox/wind/common/errors"
	"github.com/stretchr/testify/assert"
)

func TestConn(t *testing.T) {
	t.Run("TestReader", func(t *testing.T) {
		s1 := "abcdef4343"
		conn1 := NewConn(s1)
		assert.Nil(t, conn1.SetWriteTimeout(1))
		err := conn1.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		assert.Equal(t, nil, err)
		err = conn1.SetReadTimeout(time.Millisecond * 100)
		assert.Equal(t, nil, err)

		// Peek Skip Read
		b, _ := conn1.Peek(1)
		assert.Equal(t, []byte{'a'}, b)
		conn1.Skip(1)                   // 游标跳过了 a
		readByte, _ := conn1.ReadByte() // 游标跳过了 b
		assert.Equal(t, byte('b'), readByte)

		p := make([]byte, 100)
		n, err := conn1.Read(p) // 从 c 开始读取 100 个字节
		assert.Equal(t, nil, err)
		assert.Equal(t, s1[2:], string(p[:n]))

		_, err = conn1.Peek(1) // 上一步已经读到底了，此步骤取不出来
		assert.Equal(t, errs.ErrTimeout, err)

		conn2 := NewConn(s1)             // 重新来
		p, _ = conn2.ReadBinary(len(s1)) // 一次性读完
		assert.Equal(t, s1, string(p))
		assert.Equal(t, 0, conn2.Len()) // 没有可读的了
		// Reader
		assert.Equal(t, conn2.zr, conn2.Reader())
	})

	t.Run("TestReadWriter", func(t *testing.T) {
		s1 := "abcdef4343"
		conn := NewConn(s1)
		p, err := conn.ReadBinary(len(s1)) // 一次性全读出来
		assert.Equal(t, nil, err)
		assert.Equal(t, s1, string(p))

		wr := conn.WriterRecorder()
		s2 := "efghljk"
		// WriteBinary
		n, err := conn.WriteBinary([]byte(s2)) // 写入缓冲区
		assert.Equal(t, nil, err)
		assert.Equal(t, len(s2), n)
		assert.Equal(t, len(s2), wr.WroteLen())

		// Flush
		p, _ = wr.ReadBinary(len(s2)) // 此时上一步写入的数据还在缓冲区，所以读不出来
		assert.Equal(t, len(p), 0)

		conn.Flush()                  // 将缓冲区数据发至对端
		p, _ = wr.ReadBinary(len(s2)) // 可以读出来了
		assert.Equal(t, s2, string(p))

		// Write
		s3 := "foobarbaz"
		n, err = conn.Write([]byte(s3)) // 直接写入对端
		assert.Equal(t, nil, err)
		assert.Equal(t, len(s3), n)
		p, _ = wr.ReadBinary(len(s3))
		assert.Equal(t, s3, string(p))

		// Malloc
		buf, _ := conn.Malloc(10)
		assert.Equal(t, 10, len(buf))
		// Writer
		assert.Equal(t, conn.zw, conn.Writer())

		_, err = DialerFun("")
		assert.Equal(t, nil, err)
	})

	t.Run("TestNotImplement", func(t *testing.T) {
		conn := NewConn("")
		t1 := time.Now().Add(time.Millisecond)
		du1 := time.Second
		assert.Equal(t, nil, conn.Release())
		assert.Equal(t, nil, conn.Close())
		assert.Equal(t, nil, conn.LocalAddr())
		assert.Equal(t, nil, conn.RemoteAddr())
		assert.Equal(t, nil, conn.SetIdleTimeout(du1))
		assert.Panics(t, func() {
			conn.SetDeadline(t1)
		})
		assert.Panics(t, func() {
			conn.SetWriteDeadline(t1)
		})
		assert.Panics(t, func() {
			conn.IsActive()
		})
		assert.Panics(t, func() {
			conn.SetOnRequest(func(ctx context.Context, connection netpoll.Connection) error {
				return nil
			})
		})
		assert.Panics(t, func() {
			conn.AddCloseCallback(func(connection netpoll.Connection) error {
				return nil
			})
		})
	})
}

func TestSlowConn(t *testing.T) {
	t.Run("TestSlowReadConn", func(t *testing.T) {
		s1 := "abcdefg"
		conn := NewSlowReadConn(s1)
		assert.Nil(t, conn.SetWriteTimeout(1))
		assert.Nil(t, conn.SetReadTimeout(1))
		assert.Equal(t, time.Duration(1), conn.readTimeout)

		b, err := conn.Peek(4)
		assert.Equal(t, nil, err)
		assert.Equal(t, s1[:4], string(b))
		conn.Skip(len(s1))
		_, err = conn.Peek(1)
		assert.Equal(t, ErrReadTimeout, err)
		_, err = SlowReadDialer("")
		assert.Equal(t, nil, err)
	})

	t.Run("TestSlowWriteConn", func(t *testing.T) {
		conn, err := SlowWriteDialer("")
		assert.Equal(t, nil, err)
		conn.SetWriteTimeout(time.Millisecond * 100)
		err = conn.Flush()
		assert.Equal(t, ErrWriteTimeout, err)
	})
}

func TestBrokenConn_Flush(t *testing.T) {
	conn := NewBrokenConn("")
	n, err := conn.Writer().WriteBinary([]byte("Foo"))
	assert.Equal(t, 3, n)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrConnectionClosed, conn.Flush())
}

func TestBrokenConn_Peek(t *testing.T) {
	conn := NewBrokenConn("Foo")
	buf, err := conn.Peek(3)
	assert.Nil(t, buf)
	assert.Equal(t, io.ErrUnexpectedEOF, err)
}

func TestOneTimeConn_Flush(t *testing.T) {
	conn := NewOneTimeConn("")
	n, err := conn.Writer().WriteBinary([]byte("Foo"))
	assert.Equal(t, 3, n)
	assert.Nil(t, err)
	assert.Nil(t, conn.Flush())
	n, err = conn.Writer().WriteBinary([]byte("Bar"))
	assert.Equal(t, 3, n)
	assert.Nil(t, err)
	assert.Equal(t, errs.ErrConnectionClosed, conn.Flush())
}

func TestOneTimeConn_Skip(t *testing.T) {
	conn := NewOneTimeConn("FooBar")
	buf, err := conn.Peek(3)
	assert.Equal(t, "Foo", string(buf))
	assert.Nil(t, err)
	assert.Nil(t, conn.Skip(3))
	assert.Equal(t, 3, conn.contentLength)

	buf, err = conn.Peek(3)
	assert.Equal(t, "Bar", string(buf))
	assert.Nil(t, err)
	assert.Nil(t, conn.Skip(3))
	assert.Equal(t, 0, conn.contentLength)

	buf, err = conn.Peek(3)
	assert.Equal(t, 0, len(buf))
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, io.EOF, conn.Skip(3))
	assert.Equal(t, 0, conn.contentLength)
}
