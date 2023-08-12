package resp

import (
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"

	"github.com/favbox/wind/common/bytebufferpool"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/hlog"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/protocol/http1/ext"
)

// ErrBodyStreamWritePanic 返回于写入正文流发生恐慌时。
type ErrBodyStreamWritePanic struct {
	error
}

type h1Response struct {
	*protocol.Response
}

// 返回 http1 响应的字符串表达。
//
// 出错时返回错误信息而非响应表达。
//
// 对于性能很关键的代码，请使用 Write 而非 String。
func (h1Resp *h1Response) String() string {
	w := bytebufferpool.Get()
	zw := network.NewWriter(w)
	if err := Write(h1Resp.Response, zw); err != nil {
		return err.Error()
	}
	if err := zw.Flush(); err != nil {
		return err.Error()
	}
	s := string(w.B)
	bytebufferpool.Put(w)
	return s
}

// GetHTTP1Response 获取响应的 http1 字符串形式。
func GetHTTP1Response(resp *protocol.Response) fmt.Stringer {
	return &h1Response{resp}
}

// ReadBodyStream 流式读取 r 到响应 resp。
func ReadBodyStream(resp *protocol.Response, r network.Reader, maxBodySize int, closeCallback func(shouldClose bool) error) error {
	resp.ResetBody()
	err := ReadHeader(&resp.Header, r)
	if err != nil {
		return err
	}

	if resp.Header.StatusCode() == consts.StatusContinue {
		// 读取下一个响应，根据 http://www.w3.org/Protocols/rfc2616/rfc2616-sec8.html
		if err = ReadHeader(&resp.Header, r); err != nil {
			return err
		}
	}

	if resp.MustSkipBody() {
		return nil
	}

	bodyBuf := resp.BodyBuffer()
	bodyBuf.Reset()
	bodyBuf.B, err = ext.ReadBodyWithStreaming(r, resp.Header.ContentLength(), maxBodySize, bodyBuf.B)
	if err != nil {
		if errors.Is(err, errs.ErrBodyTooLarge) {
			bodyStream := ext.AcquireBodyStream(bodyBuf, r, resp.Header.Trailer(), resp.Header.ContentLength())
			resp.ConstructBodyStream(bodyBuf, convertClientRespStream(bodyStream, closeCallback))
			return nil
		}

		if errors.Is(err, errs.ErrChunkedStream) {
			bodyStream := ext.AcquireBodyStream(bodyBuf, r, resp.Header.Trailer(), -1)
			resp.ConstructBodyStream(bodyBuf, convertClientRespStream(bodyStream, closeCallback))
			return nil
		}

		resp.Reset()
		return err
	}

	bodyStream := ext.AcquireBodyStream(bodyBuf, r, resp.Header.Trailer(), resp.Header.ContentLength())
	resp.ConstructBodyStream(bodyBuf, convertClientRespStream(bodyStream, closeCallback))
	return nil
}

// Read 读取 r 到请求 req（包括正文）。
//
// 若 r 已关闭则返回 io.EOF。
func Read(resp *protocol.Response, r network.Reader) error {
	return ReadHeaderAndLimitBody(resp, r, 0)
}

// ReadHeaderAndLimitBody 读取 r 到请求 req，限定正文大小。
//
// 若 maxBodySize > 0 且正文大小超此限制，则 ErrBodyTooLarge 将被返回。
//
// 若 r 已关闭则返回 io.EOF。
func ReadHeaderAndLimitBody(resp *protocol.Response, zr network.Reader, maxBodySize int) error {
	resp.ResetBody()
	err := ReadHeader(&resp.Header, zr)
	if err != nil {
		return err
	}
	if resp.Header.StatusCode() == consts.StatusContinue {
		// 读取下一个响应，根据 http://www.w3.org/Protocols/rfc2616/rfc2616-sec8.html
		if err = ReadHeader(&resp.Header, zr); err != nil {
			return err
		}
	}

	if !resp.MustSkipBody() {
		bodyBuf := resp.BodyBuffer()
		bodyBuf.Reset()
		bodyBuf.B, err = ext.ReadBody(zr, resp.Header.ContentLength(), maxBodySize, bodyBuf.B)
		if err != nil {
			return err
		}
		if resp.Header.ContentLength() == -1 {
			err = ext.ReadTrailer(resp.Header.Trailer(), zr)
			if err != nil && err != io.EOF {
				return err
			}
		}
		resp.Header.SetContentLength(len(bodyBuf.B))
	}

	return nil
}

// Write 写响应到网路写入器。
//
// Write 出于性能原因不会刷新响应到网络写入器。
func Write(resp *protocol.Response, w network.Writer) error {
	sendBody := !resp.MustSkipBody()

	if resp.IsBodyStream() {
		return writeBodyStream(resp, w, sendBody)
	}

	body := resp.BodyBytes()
	bodyLen := len(body)
	if sendBody || bodyLen > 0 {
		resp.Header.SetContentLength(bodyLen)
	}

	header := resp.Header.Header()
	_, err := w.WriteBinary(header)
	if err != nil {
		return err
	}
	resp.Header.SetHeaderLength(len(header))
	// 写入正文
	if sendBody && bodyLen > 0 {
		_, err = w.WriteBinary(body)
	}
	return err
}

func writeBodyStream(resp *protocol.Response, w network.Writer, sendBody bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &ErrBodyStreamWritePanic{
				error: fmt.Errorf("恐慌发生于写入正文流的时候：%+v", r),
			}
		}
	}()

	contentLength := resp.Header.ContentLength()
	if contentLength < 0 {
		lrSize := ext.LimitedReaderSize(resp.BodyStream())
		if lrSize >= 0 {
			contentLength = int(lrSize)
			if int64(contentLength) != lrSize {
				contentLength = -1
			}
			if contentLength >= 0 {
				resp.Header.SetContentLength(contentLength)
			}
		}
	}
	if contentLength >= 0 {
		if err = WriteHeader(&resp.Header, w); err == nil && sendBody {
			if resp.ImmediateHeaderFlush {
				err = w.Flush()
			}
			if err == nil {
				err = ext.WriteBodyFixedSize(w, resp.BodyStream(), int64(contentLength))
			}
		}
	} else {
		resp.Header.SetContentLength(-1)
		if err = WriteHeader(&resp.Header, w); err == nil && sendBody {
			if resp.ImmediateHeaderFlush {
				err = w.Flush()
			}
			if err == nil {
				err = ext.WriteBodyChunked(w, resp.BodyStream())
			}
			if err == nil {
				err = ext.WriteTrailer(resp.Header.Trailer(), w)
			}
		}
	}
	err1 := resp.CloseBodyStream()
	if err == nil {
		err = err1
	}

	return err
}

var clientRespStreamPool = sync.Pool{
	New: func() any {
		return &clientRespStream{}
	},
}

// 池化管理的客户端响应流。
type clientRespStream struct {
	r             io.Reader
	closeCallback func(shouldClose bool) error
}

func (c *clientRespStream) Read(p []byte) (n int, err error) {
	return c.r.Read(p)
}

func (c *clientRespStream) Close() error {
	runtime.SetFinalizer(c, nil)
	// 如果释放时发生错误，则连接可能处于异常状态。
	// 在回调中关闭它，以避免其他意外问题。
	err := ext.ReleaseBodyStream(c.r)
	shouldClose := false
	if err != nil {
		shouldClose = true
		hlog.Warnf("连接即将关闭而非回收，因为在正文流释放过程中发生了错误：%s", err.Error())
	}
	if c.closeCallback != nil {
		err = c.closeCallback(shouldClose)
	}
	c.reset()
	return err
}

func (c *clientRespStream) reset() {
	c.closeCallback = nil
	c.r = nil
	clientRespStreamPool.Put(c)
}

func convertClientRespStream(bs io.Reader, fn func(shouldClose bool) error) *clientRespStream {
	clientStream := clientRespStreamPool.Get().(*clientRespStream)
	clientStream.r = bs
	clientStream.closeCallback = fn
	runtime.SetFinalizer(clientStream, (*clientRespStream).Close)
	return clientStream
}
