package req

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/favbox/gosky/wind/internal/bytestr"
	errs "github.com/favbox/gosky/wind/pkg/common/errors"
	"github.com/favbox/gosky/wind/pkg/common/utils"
	"github.com/favbox/gosky/wind/pkg/network"
	"github.com/favbox/gosky/wind/pkg/protocol"
	"github.com/favbox/gosky/wind/pkg/protocol/consts"
	"github.com/favbox/gosky/wind/pkg/protocol/http1/ext"
)

var errEOFReadHeader = errs.NewPublic("读取请求标头错出错：EOF")

// WriteHeader 写入请求头 h 至 w。
func WriteHeader(h *protocol.RequestHeader, w network.Writer) error {
	header := h.Header()
	_, err := w.WriteBinary(header)
	return err
}

// ReadHeader 读取 r 至 请求头 h。
func ReadHeader(h *protocol.RequestHeader, r network.Reader) error {
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

// 先尝试读取 n 个字节，若无误再读取全部字节至请求头。
func tryRead(h *protocol.RequestHeader, r network.Reader, n int) error {
	h.ResetSkipNormalize()
	b, err := r.Peek(n)
	if len(b) == 0 {
		if err != io.EOF {
			return err
		}

		// 读取请求的第1个字节
		if n == 1 {
			// 我们不读取单字节。
			return errs.New(errs.ErrNothingRead, errs.ErrorTypePrivate, err)
		}

		return errEOFReadHeader
	}
	b = ext.MustPeekBuffered(r)
	headersLen, errParse := parse(h, b)
	if errParse != nil {
		return ext.HeaderError("request", err, errParse, b)
	}
	ext.MustDiscard(r, headersLen)
	return nil
}

func parse(h *protocol.RequestHeader, buf []byte) (int, error) {
	m, err := parseFirstLine(h, buf)
	if err != nil {
		return 0, err
	}

	rawHeaders, _, err := ext.ReadRawHeaders(h.RawHeaders()[0:], buf[m:])
	h.SetRawHeaders(rawHeaders)
	if err != nil {
		return 0, err
	}

	var n int
	n, err = parseHeaders(h, buf[m:])
	if err != nil {
		return 0, err
	}

	return m + n, nil
}

// 解析请求头的首行信息 - 请求方法、网址、协议
func parseFirstLine(h *protocol.RequestHeader, buf []byte) (int, error) {
	bNext := buf
	var b []byte
	var err error
	for len(b) == 0 {
		if b, bNext, err = utils.NextLine(bNext); err != nil {
			return 0, err
		}
	}

	// 解析方法
	n := bytes.IndexByte(b, ' ')
	if n <= 0 {
		return 0, fmt.Errorf("无法找到 http 请求方法 %q", ext.BufferSnippet(buf))
	}
	h.SetMethodBytes(b[:n])
	b = b[n+1:]

	// 设置默认协议
	h.SetProtocol(consts.HTTP11)

	// 设置请求协议和网址
	n = bytes.LastIndexByte(b, ' ')
	if n < 0 {
		h.SetProtocol(consts.HTTP10)
		n = len(b)
	} else if n == 0 {
		return 0, fmt.Errorf("请求网址不能为空 %q", buf)
	} else if !bytes.Equal(b[n+1:], bytestr.StrHTTP11) {
		h.SetProtocol(consts.HTTP10)
	}
	h.SetRequestURIBytes(b[:n])

	return len(buf) - len(bNext), nil
}

func parseHeaders(h *protocol.RequestHeader, buf []byte) (int, error) {
	h.InitContentLengthWithValue(-2)

	var s ext.HeaderScanner
	s.B = buf
	s.DisableNormalizing = h.IsDisableNormalizing()
	var err error
	for s.Next() {
		if len(s.Key) > 0 {
			// 标头键名和冒号之间不允许有空格。
			// 详见 RFC 7230, Section 3.2.4.
			if bytes.IndexByte(s.Key, ' ') != -1 || bytes.IndexByte(s.Key, '\t') != -1 {
				err = fmt.Errorf("无效的标头键名 %q", s.Key)
				continue
			}

			switch s.Key[0] | 0x20 {
			case 'h':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrHost) {
					h.SetHostBytes(s.Value)
					continue
				}
			case 'u':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrUserAgent) {
					h.SetUserAgentBytes(s.Value)
					continue
				}
			case 'c':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentType) {
					h.SetContentTypeBytes(s.Value)
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrContentLength) {
					if h.ContentLength() != -1 {
						var nErr error
						var contentLength int
						if contentLength, nErr = protocol.ParseContentLength(s.Value); nErr != nil {
							if err == nil {
								err = nErr
							}
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
			case 't':
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTransferEncoding) {
					if !bytes.Equal(s.Value, bytestr.StrIdentity) {
						h.InitContentLengthWithValue(-1)
						h.SetArgBytes(bytestr.StrTransferEncoding, bytestr.StrChunked, protocol.ArgsHasValue)
					}
					continue
				}
				if utils.CaseInsensitiveCompare(s.Key, bytestr.StrTrailer) {
					if nErr := h.Trailer().SetTrailers(s.Value); nErr != nil {
						if err == nil {
							err = nErr
						}
					}
					continue
				}
			}
		}
		h.AddArgBytes(s.Key, s.Value, protocol.ArgsHasValue)
	}

	if s.Err != nil && err == nil {
		err = s.Err
	}
	if err != nil {
		h.SetConnectionClose(true)
		return 0, err
	}

	if h.ContentLength() < 0 {
		h.SetContentLengthBytes(h.ContentLengthBytes()[:0])
	}
	if !h.IsHTTP11() && !h.ConnectionClose() {
		// 除非设置了 'Connection: keep-alive' 否则关闭非 http/1.1 请求的连接
		v := h.PeekArgBytes(bytestr.StrConnection)
		h.SetConnectionClose(!ext.HasHeaderValue(v, bytestr.StrKeepAlive))
	}
	return s.HLen, nil
}
