package resp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytesconv"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/consts"
	"github.com/favbox/wind/protocol/http1/ext"
)

var errTimeout = errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "读取响应头")

// WriteHeader 写入响应头 h 到 w。
func WriteHeader(h *protocol.ResponseHeader, w network.Writer) error {
	header := h.Header()
	h.SetContentLength(len(header))
	_, err := w.WriteBinary(header)
	return err
}

// ReadHeader 读取 r 至响应头 h。
//
// 若 r 已关闭则返回 io.EOF。
func ReadHeader(h *protocol.ResponseHeader, r network.Reader) error {
	n := 1
	for {
		err := tryRead(h, r, n)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errs.ErrNeedMore) {
			h.ResetSkipNormalize()
			return err
		}

		// 无更多可用数据，尝试阻断 peek
		if n == r.Len() {
			n++

			continue
		}
		n = r.Len()
	}
}

// ConnectionUpgrade 判断连接是否需升级（即设置标头：'Connection: Upgrade'）
func ConnectionUpgrade(h *protocol.ResponseHeader) bool {
	return ext.HasHeaderValue(h.Peek(consts.HeaderConnection), bytestr.StrKeepAlive)
}

// 先尝试读取 n 个字节，若无误再读取全部字节至响应头。
func tryRead(h *protocol.ResponseHeader, r network.Reader, n int) error {
	h.ResetSkipNormalize()
	b, err := r.Peek(n)
	if len(b) == 0 {
		// 若超时则返回 ErrTimeout
		if err != nil && strings.Contains(err.Error(), "timeout") {
			return errTimeout
		}

		// 把只读 1 个字节时出的错也当做 EOF
		if n == 1 || err == io.EOF {
			return io.EOF
		}

		return fmt.Errorf("错误发生于读取响应头：%s", err)
	}
	b = ext.MustPeekBuffered(r)
	headersLen, errParse := parse(h, b)
	if errParse != nil {
		return ext.HeaderError("response", err, errParse, b)
	}
	ext.MustDiscard(r, headersLen)
	return nil
}

// 解析 buf 至 h。
func parse(h *protocol.ResponseHeader, buf []byte) (int, error) {
	m, err := parseFirstLine(h, buf)
	if err != nil {
		return 0, err
	}
	n, err := parseHeaders(h, buf[m:])
	if err != nil {
		return 0, err
	}
	return m + n, nil
}

func parseFirstLine(h *protocol.ResponseHeader, buf []byte) (int, error) {
	bNext := buf
	var b []byte
	var err error
	for len(b) == 0 {
		if b, bNext, err = utils.NextLine(bNext); err != nil {
			return 0, err
		}
	}

	// 解析协议
	n := bytes.IndexByte(b, ' ')
	if n < 0 {
		return 0, fmt.Errorf("无法在响应的第一行中找到空白字符 %q", buf)
	}

	isHTTP11 := bytes.Equal(b[:n], bytestr.StrHTTP11)
	if !isHTTP11 {
		h.SetProtocol(consts.HTTP10)
	} else {
		h.SetProtocol(consts.HTTP11)
	}

	// 解析状态码
	b = b[n+1:]
	var statusCode int
	statusCode, n, err = bytesconv.ParseUintBuf(b)
	h.SetStatusCode(statusCode)
	if err != nil {
		return 0, fmt.Errorf("无法解析响应状态码：%s。响应 %q", err, buf)
	}
	if len(b) > n && b[n] != ' ' {
		return 0, fmt.Errorf("异常字符出现于状态码的尾部。响应 %q", buf)
	}

	return len(buf) - len(bNext), nil
}

func parseHeaders(h *protocol.ResponseHeader, buf []byte) (int, error) {
	// 默认内容长度为自身
	h.InitContentLengthWithValue(-2)

	var s ext.HeaderScanner
	s.B = buf
	s.DisableNormalizing = h.IsDisableNormalizing()
	var err error
	for s.Next() {
		if len(s.Key) > 0 {
			switch s.Key[0] | 0x20 {
			case 'c':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentType) {
					h.SetContentTypeBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentEncoding) {
					h.SetContentEncodingBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentLength) {
					var contentLength int
					if h.ContentLength() != -1 {
						if contentLength, err = protocol.ParseContentLength(s.Value); err != nil {
							h.InitContentLengthWithValue(-2)
						} else {
							h.InitContentLengthWithValue(contentLength)
							h.SetContentLengthBytes(s.Value)
						}
					}
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrConnection) {
					if bytes.Equal(s.Value, bytestr.StrClose) {
						h.SetConnectionClose(true)
					} else {
						h.SetConnectionClose(false)
						h.AddArgBytes(s.Key, s.Value, protocol.ArgsHasValue)
					}
					continue
				}
			case 's':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrServer) {
					h.SetServerBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrSetCookie) {
					h.ParseSetCookie(s.Value)
					continue
				}
			case 't':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTransferEncoding) {
					if !bytes.Equal(s.Value, bytestr.StrIdentity) {
						h.InitContentLengthWithValue(-1)
						h.SetArgBytes(bytestr.StrTransferEncoding, bytestr.StrChunked, protocol.ArgsHasValue)
					}
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTrailer) {
					err = h.Trailer().SetTrailers(s.Value)
					continue
				}
			}
			h.AddArgBytes(s.Key, s.Value, protocol.ArgsHasValue)
		}
	}
	if s.Err != nil {
		h.SetConnectionClose(true)
		return 0, s.Err
	}

	if h.ContentLength() < 0 {
		h.SetContentLengthBytes(h.ContentLengthBytes()[:0])
	}
	if h.ContentLength() == -2 && !ConnectionUpgrade(h) && !h.MustSkipContentLength() {
		h.SetArgBytes(bytestr.StrTransferEncoding, bytestr.StrIdentity, protocol.ArgsHasValue)
		h.SetConnectionClose(true)
	}
	if !h.IsHTTP11() && !h.ConnectionClose() {
		// 关闭非 HTTP/1.1 连接（除非设置了长连接）
		v := h.PeekArgBytes(bytestr.StrConnection)
		h.SetConnectionClose(!ext.HasHeaderValue(v, bytestr.StrKeepAlive))
	}

	return len(buf) - len(s.B), err
}
