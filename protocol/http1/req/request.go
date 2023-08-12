package req

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"

	"github.com/favbox/wind/common/bytebufferpool"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/protocol/http1/ext"
)

var (
	errRequestHostRequired = errs.NewPublic("请求缺少必要的主机标头")
	errGETOnly             = errs.NewPublic("收到非 GET 请求")
	errBodyTooLarge        = errs.New(errs.ErrBodyTooLarge, errs.ErrorTypePublic, "http1/req")
)

type h1Request struct {
	*protocol.Request
}

func (h1Req *h1Request) String() string {
	w := bytebufferpool.Get()
	zw := network.NewWriter(w)
	if err := Write(h1Req.Request, zw); err != nil {
		return err.Error()
	}
	if err := zw.Flush(); err != nil {
		return err.Error()
	}
	s := string(w.B)
	bytebufferpool.Put(w)
	return s
}

// GetHTTP1Request 获取请求的 http1 字符串形式。
func GetHTTP1Request(req *protocol.Request) fmt.Stringer {
	return &h1Request{req}
}

// ReadBodyStream 流式读取 zr 到请求 req。
func ReadBodyStream(req *protocol.Request, zr network.Reader, maxBodySize int, getOnly, preParseMultipartForm bool) error {
	if getOnly && !req.Header.IsGet() {
		return errGETOnly
	}

	if req.MayContinue() {
		return nil
	}

	return ContinueReadBodyStream(req, zr, maxBodySize, preParseMultipartForm)
}

// ContinueReadBodyStream 如果请求标头包含“Expect:100 continue”，则读取流中的请求正文。
func ContinueReadBodyStream(req *protocol.Request, zr network.Reader, maxBodySize int, preParseMultipartForm ...bool) error {
	var err error
	contentLength := req.Header.ContentLength()
	if contentLength > 0 {
		if len(preParseMultipartForm) == 0 || preParseMultipartForm[0] {
			// 已知长度的预读多部分表单数据。
			// 通过此方式，我们限制了大文件上传的内存使用，因为如果文件大小超过了 DefaultMaxInMemoryFileSize
			// 将会流式输入到临时文件。
			req.SetMultipartFormBoundary(string(req.Header.MultipartFormBoundary()))
			if len(req.MultipartFormBoundary()) > 0 && len(req.Header.PeekContentEncoding()) == 0 {
				err = protocol.ParseMultipartForm(zr.(io.Reader), req, contentLength, consts.DefaultMaxInMemoryFileSize)
				if err != nil {
					req.Reset()
				}
				return err
			}
		}
	}

	if contentLength == -2 {
		// 不忽略正文的请求（非 GET、HEAD），则设置内容长度。
		// 标识主体对 http 请求没有意义，因为主体的末尾是由连接关闭决定的。
		// 所以，对于没有 'Content-Length' 和 'Transfer-Encoding' 标头的请求来说，只需忽略请求正文即可。

		// 参见 https://tools.ietf.org/html/rfc7230#section-3.3.2
		if !req.Header.IgnoreBody() {
			req.Header.SetContentLength(0)
		}
		return nil
	}

	bodyBuf := req.BodyBuffer()
	bodyBuf.Reset()
	bodyBuf.B, err = ext.ReadBodyWithStreaming(zr, contentLength, maxBodySize, bodyBuf.B)
	if err != nil {
		if errors.Is(err, errs.ErrBodyTooLarge) {
			req.Header.SetContentLength(contentLength)
			req.ConstructBodyStream(bodyBuf, ext.AcquireBodyStream(bodyBuf, zr, req.Header.Trailer(), contentLength))

			return nil
		}
		if errors.Is(err, errs.ErrChunkedStream) {
			req.ConstructBodyStream(bodyBuf, ext.AcquireBodyStream(bodyBuf, zr, req.Header.Trailer(), contentLength))
			return nil
		}
		req.Reset()
		return err
	}

	req.ConstructBodyStream(bodyBuf, ext.AcquireBodyStream(bodyBuf, zr, req.Header.Trailer(), contentLength))
	return nil
}

// Read 读取 r 到请求 req（包括正文）。
//
// 为删除临时上传文件，必在读取多部分表单请求后调用 RemoveMultipartFormFiles 或 Reset。
//
// 如果 MayContinue 返回 true，则调用者必须：
//
//   - 若请求标头不满足调用方的要求，则发送 StatusExpectationFailed 响应。
//   - 或在用 ContinueReadBody 读取请求正文之前发送 StatusContinue 响应。
//   - 或关闭连接。
//
// 若读取标头之前请求已关闭，则返回 io.EOF。
func Read(req *protocol.Request, r network.Reader, preParse ...bool) error {
	return ReadHeaderAndLimitBody(req, r, 0, preParse...)
}

// ReadHeaderAndLimitBody 读取 r 到请求 req，限定正文大小。
//
// 为删除临时上传文件，必在读取多部分表单请求后调用 RemoveMultipartFormFiles 或 Reset。
//
// 如果 MayContinue 返回 true，则调用者必须：
//
//   - 若请求标头不满足调用方要求，则发送 StatusExpectationFailed 响应。
//   - 或在用 ContinueReadBody 读取请求正文之前发送 StatusContinue 响应。
//   - 或关闭连接。
//
// 若读取标头之前请求已关闭，则返回 io.EOF。
func ReadHeaderAndLimitBody(req *protocol.Request, r network.Reader, maxBodySize int, preParse ...bool) error {
	var parse bool
	if len(preParse) == 0 {
		parse = true
	} else {
		parse = preParse[0]
	}
	req.ResetSkipHeader()

	if err := ReadHeader(&req.Header, r); err != nil {
		return err
	}

	return ReadLimitBody(req, r, maxBodySize, false, parse)
}

func ReadLimitBody(req *protocol.Request, r network.Reader, maxBodySize int, getOnly bool, preParseMultipartForm bool) error {
	// 不要在此重置请求 - 调用方须在此前就重置它。
	if getOnly && !req.Header.IsGet() {
		return errGETOnly
	}

	if req.MayContinue() {
		return nil
	}

	return ContinueReadBody(req, r, maxBodySize, preParseMultipartForm)
}

// ContinueReadBody 如果请求标头包含“Expect:100 continue”，则读取请求正文。
func ContinueReadBody(req *protocol.Request, r network.Reader, maxBodySize int, preParseMultipartForm ...bool) error {
	var err error
	contentLength := req.Header.ContentLength()
	if contentLength > 0 {
		if maxBodySize > 0 && contentLength > maxBodySize {
			return errBodyTooLarge
		}

		if len(preParseMultipartForm) == 0 || preParseMultipartForm[0] {
			// 已知长度的预读多部分表单数据。
			// 通过此方式，我们限制了大文件上传的内存使用，因为如果文件大小超过了 DefaultMaxInMemoryFileSize
			// 将会流式输入到临时文件。
			req.SetMultipartFormBoundary(string(req.Header.MultipartFormBoundary()))
			if len(req.MultipartFormBoundary()) > 0 && len(req.Header.PeekContentEncoding()) == 0 {
				err = protocol.ParseMultipartForm(r.(io.Reader), req, contentLength, consts.DefaultMaxInMemoryFileSize)
				if err != nil {
					req.Reset()
				}
				return err
			}
		}

		// 该优化仅适用于 ping-pong 场景，而 ext.ReadBody 是一个常用函数，故我们在 ext.ReadBody 之前处理该场景。
		buf, err := r.Peek(contentLength)
		if err != nil {
			return err
		}
		r.Skip(contentLength)
		req.SetBodyRaw(buf)
		return nil
	}

	if contentLength == -2 {
		// 不忽略正文的请求（非 GET、HEAD），则设置内容长度。
		// 标识主体对 http 请求没有意义，因为主体的末尾是由连接关闭决定的。
		// 所以，对于没有 'Content-Length' 和 'Transfer-Encoding' 标头的请求来说，只需忽略请求正文即可。

		// 参见 https://tools.ietf.org/html/rfc7230#section-3.3.2
		if !req.Header.IgnoreBody() {
			req.Header.SetContentLength(0)
		}
		return nil
	}

	bodyBuf := req.BodyBuffer()
	bodyBuf.Reset()
	bodyBuf.B, err = ext.ReadBody(r, contentLength, maxBodySize, bodyBuf.B)
	if err != nil {
		req.Reset()
		return err
	}

	if req.Header.ContentLength() == -1 {
		err = ext.ReadTrailer(req.Header.Trailer(), r)
		if err != nil && err != io.EOF {
			return err
		}
	}

	req.Header.SetContentLength(len(bodyBuf.B))
	return nil
}

// Write 写请求到网络写入器。
//
// Write 出于性能原因不会刷新请求到网络写入器。
func Write(req *protocol.Request, w network.Writer) error {
	return write(req, w, false)
}

// ProxyWrite 类似 Write，但
func ProxyWrite(req *protocol.Request, w network.Writer) error {
	return write(req, w, true)
}

func handleMultipart(req *protocol.Request) error {
	if len(req.MultipartFiles()) == 0 && len(req.MultipartFields()) == 0 {
		return nil
	}

	var err error
	bodyBuffer := &bytes.Buffer{}
	w := multipart.NewWriter(bodyBuffer)
	if len(req.MultipartFiles()) > 0 {
		for _, f := range req.MultipartFiles() {
			if f.Reader != nil {
				err = protocol.WriteMultipartFormFile(w, f.ParamName, f.Name, f.Reader)
			} else {
				err = protocol.AddFile(w, f.ParamName, f.Name)
			}
			if err != nil {
				return err
			}
		}
	}

	if len(req.MultipartFields()) > 0 {
		for _, mf := range req.MultipartFields() {
			if err = protocol.AddMultipartFormField(w, mf); err != nil {
				return err
			}
		}
	}

	req.Header.Set(consts.HeaderContentType, w.FormDataContentType())
	if err = w.Close(); err != nil {
		return err
	}

	r := multipart.NewReader(bodyBuffer, w.Boundary())
	f, err := r.ReadForm(int64(bodyBuffer.Len()))
	if err != nil {
		return err
	}
	protocol.SetMultipartFormWithBoundary(req, f, w.Boundary())

	return nil
}

func write(req *protocol.Request, w network.Writer, usingProxy bool) error {
	if len(req.Header.Host()) == 0 || req.IsURIParsed() {
		uri := req.URI()
		host := uri.Host()
		if len(host) == 0 {
			return errRequestHostRequired
		}

		if len(req.Header.Host()) == 0 {
			req.Header.SetHostBytes(host)
		}

		ruri := uri.RequestURI()
		if bytes.Equal(req.Method(), bytestr.StrConnect) {
			ruri = uri.Host()
		} else if usingProxy {
			ruri = uri.FullURI()
		}

		req.Header.SetRequestURIBytes(ruri)

		// 若请求网址中用户名，则b64编码并写入 Authorization 请求头
		if len(uri.Username()) > 0 {
			nl := len(uri.Username()) + len(uri.Password()) + 1
			nb := nl + len(bytestr.StrBasicSpace)
			tl := nb + base64.StdEncoding.EncodedLen(nl)

			req.Header.InitBufValue(tl)
			buf := req.Header.GetBufValue()[:0]
			buf = append(buf, uri.Username()...)
			buf = append(buf, bytestr.StrColon...)
			buf = append(buf, uri.Password()...)
			buf = append(buf, bytestr.StrBasicSpace...)
			base64.StdEncoding.Encode(buf[nb:tl], buf[:nl])
			req.Header.SetBytesKV(bytestr.StrAuthorization, buf[nl:tl])
		}
	}

	if req.IsBodyStream() {
		return writeBodyStream(req, w)
	}

	body := req.BodyBytes()
	err := handleMultipart(req)
	if err != nil {
		return fmt.Errorf("处理多部分表单出错：%s", err)
	}
	if req.OnlyMultipartForm() {
		m, _ := req.MultipartForm()
		body, err = protocol.MarshalMultipartForm(m, req.MultipartFormBoundary())
		if err != nil {
			return fmt.Errorf("编码多部分表单出错：%s", err)
		}
		req.Header.SetMultipartFormBoundary(req.MultipartFormBoundary())
	}

	hasBody := false
	if len(body) == 0 {
		body = req.PostArgString()
	}
	if len(body) != 0 || !req.Header.IgnoreBody() {
		hasBody = true
		req.Header.SetContentLength(len(body))
	}

	header := req.Header.Header()
	if _, err = w.WriteBinary(header); err != nil {
		return err
	}

	// 写入正文
	if hasBody {
		w.WriteBinary(body)
	} else if len(body) > 0 {
		return fmt.Errorf("非 POST 请求存在未处理的非空正文 %q", body)
	}
	return nil
}

func writeBodyStream(req *protocol.Request, w network.Writer) error {
	var err error

	contentLength := req.Header.ContentLength()
	if contentLength < 0 {
		lrSize := ext.LimitedReaderSize(req.BodyStream())
		if lrSize >= 0 {
			contentLength = int(lrSize)
			if int64(contentLength) != lrSize {
				contentLength = -1
			}
			if contentLength >= 0 {
				req.Header.SetContentLength(contentLength)
			}
		}
	}
	if contentLength >= 0 {
		if err = WriteHeader(&req.Header, w); err == nil {
			err = ext.WriteBodyFixedSize(w, req.BodyStream(), int64(contentLength))
		}
	} else {
		req.Header.SetContentLength(-1)
		err = WriteHeader(&req.Header, w)
		if err == nil {
			err = ext.WriteBodyChunked(w, req.BodyStream())
		}
		if err == nil {
			err = ext.WriteTrailer(req.Header.Trailer(), w)
		}
	}
	err1 := req.CloseBodyStream()
	if err == nil {
		err = err1
	}
	return err
}
