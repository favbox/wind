package protocol

import (
	"io"
	"net"
	"sync"

	"github.com/favbox/wind/common/bytebufferpool"
	"github.com/favbox/wind/common/compress"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/nocopy"
	"github.com/favbox/wind/network"
)

var (
	// 响应的主体缓冲池，减少 GC
	responseBodyPool bytebufferpool.Pool

	// 响应的实例池，减少 GC
	responsePool sync.Pool
)

// Response 表示 HTTP 请求。
//
// 禁止拷贝 Response 实例。替代方法为创建新实例或使用 CopyTo。
//
// # Response 实例不能用于并发协程。
type Response struct {
	noCopy nocopy.NoCopy

	// Response 标头
	//
	// 禁止值拷贝 Header。可使用 Header 指针。
	Header ResponseHeader

	// 尽快刷新标头，无需等待第一个主体字节。
	// 仅与 bodyStream 相关。
	ImmediateHeaderFlush bool

	bodyStream      io.Reader
	w               responseBodyWriter
	body            *bytebufferpool.ByteBuffer
	bodyRaw         []byte
	maxKeepBodySize int

	// 若为真，Response.Read() 则跳过正文读取。
	// 用于读取 HEAD 响应。
	//
	// 若为真，Response.Write() 则跳过正文设置。
	// 用于写入 HEAD 响应。
	SkipBody bool

	// 并发网络连接的远程 TCP 地址
	raddr net.Addr
	// 并发网络连接的本地 TCP 地址
	laddr net.Addr

	// If set a hijackWriter, wind will skip the default header/body writer process.
	hijackWriter network.ExtWriter
}

type responseBodyWriter struct {
	r *Response
}

func (w *responseBodyWriter) Write(p []byte) (int, error) {
	w.r.AppendBody(p)
	return len(p), nil
}

// AppendBody 追加 p 至响应主体的字节缓冲区。
//
// 函数返回后，复用 p 是安全的。
func (resp *Response) AppendBody(p []byte) {
	_ = resp.CloseBodyStream()
	if resp.hijackWriter != nil {
		_, _ = resp.hijackWriter.Write(p)
		return
	}
	_, _ = resp.BodyBuffer().Write(p)
}

// AppendBodyString 追加 s 至响应的主体字节缓冲区。
func (resp *Response) AppendBodyString(s string) {
	_ = resp.CloseBodyStream()
	if resp.hijackWriter != nil {
		_, _ = resp.hijackWriter.Write(bytesconv.S2b(s))
		return
	}
	_, _ = resp.BodyBuffer().WriteString(s)
}

// Body 返回响应的主体。
func (resp *Response) Body() []byte {
	body, _ := resp.BodyE()
	return body
}

// BodyE 返回响应的主体和错误。
// 如果获取失败则返回 nil。
func (resp *Response) BodyE() ([]byte, error) {
	// 优先使用流中的主体
	if resp.bodyStream != nil {
		bodyBuf := resp.BodyBuffer()
		bodyBuf.Reset()
		zw := network.NewWriter(bodyBuf)
		_, err := utils.CopyZeroAlloc(zw, resp.bodyStream)
		_ = resp.CloseBodyStream()
		if err != nil {
			return nil, err
		}
	}

	// 再尝试 bodyRaw 和 body
	return resp.BodyBytes(), nil
}

// BodyBytes 返回响应的主体缓冲区的字节切片形式。
func (resp *Response) BodyBytes() []byte {
	if resp.bodyRaw != nil {
		return resp.bodyRaw
	}
	if resp.body == nil {
		return nil
	}
	return resp.body.B
}

// BodyBuffer 返回响应的主体缓冲区。
//
// 如果为空，则从响应主体池中获取一个新字节缓冲区。
func (resp *Response) BodyBuffer() *bytebufferpool.ByteBuffer {
	if resp.body == nil {
		resp.body = responseBodyPool.Get()
	}
	resp.bodyRaw = nil
	return resp.body
}

// BodyGunzip 返回解压缩后的主体数据。
//
// 如果响应标头包含 'Content-Encoding: gzip' 还要读取未压缩的主体，则使用该方法。
// 使用 Body 读取压缩后的响应体内容。
func (resp *Response) BodyGunzip() ([]byte, error) {
	return gunzipData(resp.Body())
}

// BodyStream 返回响应的正文流。
func (resp *Response) BodyStream() io.Reader {
	return resp.bodyStream
}

// BodyWriter 返回用于填充响应主体的写入器。
// 如果在 RequestHandler 内部使用，则从 RequestHandler 返回后不得使用返回的写入器。
// 在这种情况下，请使用 RequestContext.Write 或 SetBodyStreamWriter。
func (resp *Response) BodyWriter() io.Writer {
	resp.w.r = resp
	return &resp.w
}

// BodyWriteTo 将相应主体写入 w。
func (resp *Response) BodyWriteTo(w io.Writer) error {
	zw := network.NewWriter(w)
	if resp.bodyStream != nil {
		_, err := utils.CopyZeroAlloc(zw, resp.bodyStream)
		_ = resp.CloseBodyStream()
		return err
	}

	body := resp.BodyBytes()
	_, _ = zw.WriteBinary(body)
	return zw.Flush()
}

func gunzipData(p []byte) ([]byte, error) {
	var bb bytebufferpool.ByteBuffer
	_, err := compress.WriteGunzip(&bb, p)
	if err != nil {
		return nil, err
	}
	return bb.B, nil
}

// CloseBodyStream 关闭响应的主体数据流。
func (resp *Response) CloseBodyStream() error {
	if resp.bodyStream == nil {
		return nil
	}

	var err error
	if bsc, ok := resp.bodyStream.(io.Closer); ok {
		err = bsc.Close()
	}
	resp.bodyStream = nil
	return err
}

// ConnectionClose 返回响应头是否已设置 'Connection: close'。
func (resp *Response) ConnectionClose() bool {
	return resp.Header.ConnectionClose()
}

// SetConnectionClose 设置响应的连接关闭标头。
func (resp *Response) SetConnectionClose() {
	resp.Header.SetConnectionClose(true)
}

// ConstructBodyStream 同时设置响应的主体字节缓冲区和流。
func (resp *Response) ConstructBodyStream(body *bytebufferpool.ByteBuffer, bodyStream io.Reader) {
	resp.body = body
	resp.bodyStream = bodyStream
}

// CopyTo 拷贝正文流之外的响应信息到 dst。
func (resp *Response) CopyTo(dst *Response) {
	resp.CopyToSkipBody(dst)
	if resp.bodyRaw != nil {
		dst.bodyRaw = append(dst.bodyRaw[:0], resp.bodyRaw...)
		if dst.body != nil {
			dst.body.Reset()
		}
	} else if resp.body != nil {
		dst.BodyBuffer().Set(resp.body.B)
	} else if dst.body != nil {
		dst.body.Reset()
	}
}

func (resp *Response) CopyToSkipBody(dst *Response) {
	dst.Reset()
	resp.Header.CopyTo(&dst.Header)
	dst.SkipBody = resp.SkipBody
	dst.raddr = resp.raddr
	dst.laddr = resp.laddr
}

func (resp *Response) GetHijackWriter() network.ExtWriter {
	return resp.hijackWriter
}

// HasBodyBytes 是否有主体字节？
func (resp *Response) HasBodyBytes() bool {
	return len(resp.BodyBytes()) != 0
}

// HijackWriter 设置 hijack 写入器。
func (resp *Response) HijackWriter(writer network.ExtWriter) {
	resp.hijackWriter = writer
}

// IsBodyStream 主体是由 SetBodyStream 设置的吗？
func (resp *Response) IsBodyStream() bool {
	return resp.bodyStream != nil
}

// LocalAddr 返回本地网络地址。该地址并发共享，勿改。
func (resp *Response) LocalAddr() net.Addr {
	return resp.laddr
}

// RemoteAddr 返回远程网址。该网址并发共享，勿改。
func (resp *Response) RemoteAddr() net.Addr {
	return resp.raddr
}

// MustSkipBody 根据 SkipBody 和 StatusCode 判断：是否要跳过对主体的处理?
func (resp *Response) MustSkipBody() bool {
	return resp.SkipBody || resp.Header.MustSkipContentLength()
}

// ParseNetAddr 解析远程和本地的网络地址。
func (resp *Response) ParseNetAddr(conn network.Conn) {
	resp.raddr = conn.RemoteAddr()
	resp.laddr = conn.LocalAddr()
}

// Reset 重置响应。
func (resp *Response) Reset() {
	resp.Header.Reset()
	resp.resetSkipHeader()
	resp.SkipBody = false
	resp.raddr = nil
	resp.laddr = nil
	resp.ImmediateHeaderFlush = false
	resp.hijackWriter = nil
}

// ResetBody 只重置响应的主体。
//
//   - 若主体字节数 ≤ 保留值，仅重置不清空
//   - 若主体字节数 ＞ 保留值，清空并返回池
func (resp *Response) ResetBody() {
	resp.bodyRaw = nil
	_ = resp.CloseBodyStream()
	if resp.body != nil {
		if resp.body.Len() <= resp.maxKeepBodySize {
			resp.body.Reset()
			return
		}
		responseBodyPool.Put(resp.body)
		resp.body = nil
	}
}

func (resp *Response) resetSkipHeader() {
	resp.ResetBody()
}

// SetBody 设置响应体。
//
// 函数返回后，可安全复用 body。
func (resp *Response) SetBody(body []byte) {
	_ = resp.CloseBodyStream()
	if resp.GetHijackWriter() == nil {
		resp.BodyBuffer().Set(body)
		return
	}

	// 若 hijack 写入器支持 SetBody() 则使用。
	if setter, ok := resp.GetHijackWriter().(interface {
		SetBody(b []byte)
	}); ok {
		setter.SetBody(body)
		return
	}

	// 否则调用 Write() 来替代。
	_, _ = resp.GetHijackWriter().Write(body)
}

// SetBodyRaw 设置响应主体，但不复制它。
//
// 基于此，内容体不可修改。
func (resp *Response) SetBodyRaw(body []byte) {
	resp.ResetBody()
	resp.bodyRaw = body
}

// SetBodyStream 设置响应的正文流和大小（可选）。
//
// 若 bodySize >= 0，那么在返回 io.EOF 之前，bodyStream 必须提供确切的 bodySize 字节。
//
// 若 bodySize < 0，那么, 则读取 bodyStream 直至 io.EOF。
//
// 若 bodyStream 实现了 io.Closer，则读取完请求的所有主体数据后调用 bodyStream.Close()。
func (resp *Response) SetBodyStream(bodyStream io.Reader, bodySize int) {
	resp.ResetBody()
	resp.bodyStream = bodyStream
	resp.Header.SetContentLength(bodySize)
}

// SetBodyStreamNoReset 类似于 SetBodyStream，但不重置先前的主体。
func (resp *Response) SetBodyStreamNoReset(bodyStream io.Reader, bodySize int) {
	resp.bodyStream = bodyStream
	resp.Header.SetContentLength(bodySize)
}

// SetBodyString 设置响应的主体。
func (resp *Response) SetBodyString(body string) {
	_ = resp.CloseBodyStream()
	resp.BodyBuffer().SetString(body)
}

// SetMaxKeepBodySize 设置响应正文的最大保留字节数。
func (resp *Response) SetMaxKeepBodySize(n int) {
	resp.maxKeepBodySize = n
}

// SetStatusCode 设置响应的状态码。
func (resp *Response) SetStatusCode(statusCode int) {
	resp.Header.SetStatusCode(statusCode)
}

// StatusCode 返回响应的状态码。
func (resp *Response) StatusCode() int {
	return resp.Header.StatusCode()
}

// AcquireResponse 从响应池获取空响应实例。
//
// 当实例不再用时，调用 ReleaseResponse 进行释放，以减少 GC，提高性能。
func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		return &Response{}
	}
	return v.(*Response)
}

// ReleaseResponse 将通过 AcquireResponse 获取的响应实例放回池中。
//
// 放回池后禁止再调该实例或成员。
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}

// SwapResponseBody 交换两个响应的主体。
func SwapResponseBody(a *Response, b *Response) {
	a.body, b.body = b.body, a.body
	a.bodyRaw, b.bodyRaw = b.bodyRaw, a.bodyRaw
	a.bodyStream, b.bodyStream = b.bodyStream, a.bodyStream
}
