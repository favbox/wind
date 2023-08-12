package mock

import (
	"bytes"
	"io"
	"net"
	"strings"
	"time"

	"github.com/cloudwego/netpoll"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/network"
)

var (
	ErrReadTimeout  = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "read timeout")
	ErrWriteTimeout = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "write timeout")
)

type Recorder interface {
	network.Reader
	WroteLen() int
}

type recorder struct {
	c *Conn
	network.Reader
}

func (r *recorder) WroteLen() int {
	return r.c.wroteLen
}

type Conn struct {
	readTimeout time.Duration
	zr          network.Reader
	zw          network.ReadWriter
	wroteLen    int
}

// --- 实现 network.Conn ---

func (m *Conn) SetReadTimeout(t time.Duration) error {
	m.readTimeout = t
	return nil
}

func (m *Conn) SetWriteTimeout(t time.Duration) error {
	return nil
}

// --- 实现 network.Reader ---

func (m *Conn) Peek(n int) ([]byte, error) {
	b, err := m.zr.Peek(n)
	if err != nil || len(b) != n {
		if m.readTimeout <= 0 {
			// 模拟永远超时
			select {}
		}
		time.Sleep(m.readTimeout)
		return nil, errs.ErrTimeout
	}
	return b, err
}

func (m *Conn) Skip(n int) error {
	return m.zr.Skip(n)
}

func (m *Conn) Release() error {
	return nil
}

func (m *Conn) Len() int {
	return m.zr.Len()
}

func (m *Conn) ReadByte() (byte, error) {
	return m.zr.ReadByte()
}

func (m *Conn) ReadBinary(n int) (p []byte, err error) {
	return m.zr.(netpoll.Reader).ReadBinary(n)
}

// --- 实现 network.Writer ---

func (m *Conn) Malloc(n int) (buf []byte, err error) {
	m.wroteLen += n
	return m.zw.Malloc(n)
}

func (m *Conn) WriteBinary(b []byte) (n int, err error) {
	n, err = m.zw.WriteBinary(b)
	m.wroteLen += n
	return n, err
}

func (m *Conn) Flush() error {
	return m.zw.Flush()
}

// --- 实现 net.Conn ---

func (m *Conn) Read(b []byte) (n int, err error) {
	return netpoll.NewIOReader(m.zr.(netpoll.Reader)).Read(b)
}

func (m *Conn) Write(b []byte) (n int, err error) {
	return netpoll.NewIOWriter(m.zw.(netpoll.ReadWriter)).Write(b)
}

func (m *Conn) Close() error {
	return nil
}

func (m *Conn) LocalAddr() net.Addr {
	return nil
}

func (m *Conn) RemoteAddr() net.Addr {
	return nil
}

func (m *Conn) SetDeadline(t time.Time) error {
	panic("待实现")
}

func (m *Conn) SetReadDeadline(t time.Time) error {
	m.readTimeout = -time.Since(t)
	return nil
}

func (m *Conn) SetWriteDeadline(t time.Time) error {
	panic("待实现")
}

// --- 其他扩展 ---

func (m *Conn) WriterRecorder() Recorder {
	return &recorder{
		c:      m,
		Reader: m.zw,
	}
}

func (m *Conn) Reader() network.Reader {
	return m.zr
}

func (m *Conn) Writer() network.Writer {
	return m.zw
}

func (m *Conn) IsActive() bool {
	panic("待实现")
}

func (m *Conn) SetIdleTimeout(t time.Duration) error {
	return nil
}

func (m *Conn) SetOnRequest(on netpoll.OnRequest) error {
	panic("待实现")
}

func (m *Conn) AddCloseCallback(callback netpoll.CloseCallback) error {
	panic("待实现")
}

func (m *Conn) GetReadTimeout() time.Duration {
	return m.readTimeout
}

// NewConn 创建指定原始请求字符串的连接。
func NewConn(source string) *Conn {
	zr := netpoll.NewReader(strings.NewReader(source))
	zw := netpoll.NewReadWriter(&bytes.Buffer{})

	return &Conn{
		zr: zr,
		zw: zw,
	}
}

type BrokenConn struct {
	*Conn
}

func (c *BrokenConn) Peek(n int) ([]byte, error) {
	return nil, io.ErrUnexpectedEOF
}

func (o *BrokenConn) Read(b []byte) (n int, err error) {
	return 0, io.ErrUnexpectedEOF
}

func (c *BrokenConn) Flush() error {
	return errs.ErrConnectionClosed
}

func NewBrokenConn(source string) *BrokenConn {
	return &BrokenConn{NewConn(source)}
}

type OneTimeConn struct {
	isRead        bool
	isFlushed     bool
	contentLength int
	*Conn
}

func (o *OneTimeConn) Peek(n int) ([]byte, error) {
	if o.isRead {
		return nil, io.EOF
	}
	return o.Conn.Peek(n)
}

func (o *OneTimeConn) Skip(n int) error {
	if o.isRead {
		return io.EOF
	}
	o.contentLength -= n
	if o.contentLength == 0 {
		o.isRead = true
	}
	return o.Conn.Skip(n)
}

func (o *OneTimeConn) Flush() error {
	if o.isFlushed {
		return errs.ErrConnectionClosed
	}
	o.isFlushed = true
	return o.Conn.Flush()
}

func NewOneTimeConn(source string) *OneTimeConn {
	return &OneTimeConn{isRead: false, isFlushed: false, Conn: NewConn(source), contentLength: len(source)}
}

// SlowReadConn 模拟慢读取连接。
type SlowReadConn struct {
	*Conn
}

func (m *SlowReadConn) SetWriteTimeout(t time.Duration) error {
	return nil
}
func (m *SlowReadConn) SetReadTimeout(t time.Duration) error {
	m.Conn.readTimeout = t
	return nil
}
func (m *SlowReadConn) Peek(i int) ([]byte, error) {
	b, err := m.zr.Peek(i)
	if m.readTimeout > 0 {
		time.Sleep(m.readTimeout)
	} else {
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil || len(b) != i {
		return nil, ErrReadTimeout
	}
	return b, err
}

func NewSlowReadConn(source string) *SlowReadConn {
	return &SlowReadConn{Conn: NewConn(source)}
}

func SlowReadDialer(addr string) (network.Conn, error) {
	return NewSlowReadConn(""), nil
}

// SlowWriteConn 模拟慢写入(休眠 100 毫秒)连接。
type SlowWriteConn struct {
	*Conn
	writeTimeout time.Duration
}

func (m *SlowWriteConn) SetWriteTimeout(t time.Duration) error {
	m.writeTimeout = t
	return nil
}

func (m *SlowWriteConn) Flush() error {
	err := m.zw.Flush()
	time.Sleep(100 * time.Millisecond)
	if err == nil {
		time.Sleep(m.writeTimeout)
		return ErrWriteTimeout
	}
	return err
}

func NewSlowWriteConn(source string) *SlowWriteConn {
	return &SlowWriteConn{
		Conn:         NewConn(source),
		writeTimeout: 0,
	}
}

func SlowWriteDialer(addr string) (network.Conn, error) {
	return NewSlowWriteConn(""), nil
}

// ErrorReadConn 模拟错误读取连接。
type ErrorReadConn struct {
	*Conn
	errorToReturn error
}

func (m *ErrorReadConn) Peek(n int) ([]byte, error) {
	return nil, m.errorToReturn
}

func NewErrorReadConn(err error) *ErrorReadConn {
	return &ErrorReadConn{
		Conn:          NewConn(""),
		errorToReturn: err,
	}
}

// StreamConn 模拟流式连接。
type StreamConn struct {
	Data []byte
}

func (m *StreamConn) Peek(n int) ([]byte, error) {
	if len(m.Data) >= n {
		return m.Data[:n], nil
	}
	if n == 1 {

	}
	return nil, errs.NewPublic("数据不足")
}

func (m *StreamConn) Skip(n int) error {
	if len(m.Data) >= n {
		m.Data = m.Data[n:]
		return nil
	}
	return errs.NewPublic("数据不足")
}

func (m *StreamConn) Release() error {
	panic("implement me")
}

func (m *StreamConn) Len() int {
	return len(m.Data)
}

func (m *StreamConn) ReadByte() (byte, error) {
	panic("implement me")
}

func (m *StreamConn) ReadBinary(n int) (p []byte, err error) {
	panic("implement me")
}

func NewStreamConn() *StreamConn {
	return &StreamConn{
		Data: make([]byte, 1<<15, 1<<16),
	}
}

func DialerFun(addr string) (network.Conn, error) {
	return NewConn(""), nil
}
