package netpoll

import (
	"errors"
	"io"
	"strings"
	"syscall"

	"github.com/cloudwego/netpoll"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/hlog"
	"github.com/favbox/wind/network"
)

// Conn 实现基于 netpoll 的网络连接。
type Conn struct {
	network.Conn
}

// --- 实现 network.ErrorNormalization ---

func (c *Conn) ToWindError(err error) error {
	if errors.Is(err, netpoll.ErrConnClosed) || errors.Is(err, syscall.EPIPE) {
		return errs.ErrConnectionClosed
	}

	// 目前只统一读取超时
	if errors.Is(err, netpoll.ErrReadTimeout) {
		return errs.ErrTimeout
	}
	return err
}

// --- 实现 network.Reader ---

func (c *Conn) Len() int {
	return c.Conn.Len()
}

func (c *Conn) Peek(n int) (b []byte, err error) {
	b, err = c.Conn.Peek(n)
	err = normalizeErr(err)
	return
}

func (c *Conn) Skip(n int) error {
	return c.Conn.Skip(n)
}

func (c *Conn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	err = normalizeErr(err)
	return n, err
}

func (c *Conn) ReadByte() (b byte, err error) {
	b, err = c.Conn.ReadByte()
	err = normalizeErr(err)
	return
}

func (c *Conn) ReadBinary(n int) (b []byte, err error) {
	b, err = c.Conn.ReadBinary(n)
	err = normalizeErr(err)
	return
}

func (c *Conn) Release() error {
	return c.Conn.Release()
}

// --- 实现 network.Writer ---

func (c *Conn) Malloc(n int) (buf []byte, err error) {
	return c.Conn.Malloc(n)
}

func (c *Conn) WriteBinary(b []byte) (n int, err error) {
	return c.Conn.WriteBinary(b)
}

func (c *Conn) Flush() error {
	return c.Conn.Flush()
}

// --- 实现 network.HandleSpecificError ---

// HandleSpecificError 判断特定错误是否需要忽略。
func (c *Conn) HandleSpecificError(err error, remoteIP string) (needIgnore bool) {
	// 需要忽略错误
	if errors.Is(err, netpoll.ErrConnClosed) || errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		// 忽略因连接被关闭或重置产生的 flush 错误
		if strings.Contains(err.Error(), "when flush") {
			return true
		}
		hlog.SystemLogger().Debugf("Netpoll error=%s, remoteAddr=%s", err.Error(), remoteIP)
		return true
	}

	// 其他为不可忽略的错误
	return false
}

func normalizeErr(err error) error {
	if errors.Is(err, netpoll.ErrEOF) {
		return io.EOF
	}

	return err
}

// 将 netpoll 连接转为 wind HTTP 连接
func newConn(c netpoll.Connection) network.Conn {
	return &Conn{Conn: c.(network.Conn)}
}
