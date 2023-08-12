package ext

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
)

const maxContentLengthInStream = 8 * 1024

var errBrokenChunk = errs.NewPublic("无法在分块数据结尾找到 crlf").SetMeta("发生于 readBodyChunked")

// MustPeekBuffered 必须返回 r 中全部数据，若无数据或出错就触发恐慌。
func MustPeekBuffered(r network.Reader) []byte {
	l := r.Len()
	buf, err := r.Peek(l)
	if len(buf) == 0 || err != nil {
		panic(fmt.Sprintf("bufio.Reader.Peek() 返回异常数据 (%q, %v)", buf, err))
	}

	return buf
}

// MustDiscard 必须跳过 r 的前 n 个字节，否则就触发恐慌。
func MustDiscard(r network.Reader, n int) {
	if err := r.Skip(n); err != nil {
		panic(fmt.Sprintf("bufio.Reader.Discard(%d) failed: %s", n, err))
	}
}

// BufferSnippet 返回字节切片的片段。
//
// 形如: <前缀 20 位>...<后缀=总长度-20位>
//
// 若前缀长 >= 后缀长，则直接返回原始切片。
func BufferSnippet(b []byte) string {
	n := len(b)
	start := 20
	end := n - start
	if start >= end {
		start = n
		end = n
	}
	bStart, bEnd := b[:start], b[end:]
	if len(bEnd) == 0 {
		return fmt.Sprintf("%q", b)
	}
	return fmt.Sprintf("%q...%q", bStart, bEnd)
}

// ReadRawHeaders 读取原始标头（即排除请求正文）。
func ReadRawHeaders(dst, buf []byte) ([]byte, int, error) {
	n := bytes.IndexByte(buf, '\n')
	if n < 0 {
		return dst[:0], 0, errNeedMore
	}
	if (n == 1 && buf[0] == '\r') || n == 0 {
		// 空标头
		return dst, n + 1, nil
	}

	n++
	b := buf
	m := n
	for {
		b = b[m:]
		m = bytes.IndexByte(b, '\n')
		if m < 0 {
			return dst, 0, errNeedMore
		}
		m++
		n += m
		if (m == 2 && b[0] == '\r') || m == 1 {
			dst = append(dst, buf[:n]...)
			return dst, n, nil
		}
	}
}

// ReadBody 从网络读取器读取数据并返回。
func ReadBody(r network.Reader, contentLength, maxBodySize int, dst []byte) ([]byte, error) {
	dst = dst[:0]
	// >= 0 固定大小读取
	if contentLength >= 0 {
		if maxBodySize > 0 && contentLength > maxBodySize {
			return dst, errBodyTooLarge
		}
		return appendBodyFixedSize(r, dst, contentLength)
	}

	// -1 分块读取
	if contentLength == -1 {
		return readBodyChunked(r, maxBodySize, dst)
	}

	// 按自身长度完整读取
	return readBodyIdentity(r, maxBodySize, dst)
}

// ReadTrailer 从网络读取器 r 中读取标头挂车 到 t。
func ReadTrailer(t *protocol.Trailer, r network.Reader) error {
	n := 1
	for {
		err := tryReadTrailer(t, r, n)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errs.ErrNeedMore) {
			t.ResetSkipNormalize()
			return err
		}
		// 无更多可用数据，尝试阻塞 peek(通过 netpoll)
		if n == r.Len() {
			n++

			continue
		}
		n = r.Len()
	}
}

func SkipTrailer(r network.Reader) error {
	n := 1
	for {
		err := trySkipTrailer(r, n)
		if err == nil {
			return nil
		}
		if !errors.Is(err, errs.ErrNeedMore) {
			return err
		}
		// 无更多可用数据，尝试阻塞 peek(通过 netpoll)
		if n == r.Len() {
			n++

			continue
		}
		n = r.Len()
	}
}

// LimitedReaderSize 返回定量读取器的定量值。
func LimitedReaderSize(r io.Reader) int64 {
	lr, ok := r.(*io.LimitedReader)
	if !ok {
		return -1
	}
	return lr.N
}

// WriteBodyFixedSize 从 r 中拷贝 size 个字节到 w。
func WriteBodyFixedSize(w network.Writer, r io.Reader, size int64) error {
	if size == 0 {
		return nil
	}
	if size > consts.MaxSmallFileSize {
		if err := w.Flush(); err != nil {
			return err
		}
	}

	if size > 0 {
		r = io.LimitReader(r, size)
	}

	n, err := utils.CopyZeroAlloc(w, r)
	if n != size && err == nil {
		err = fmt.Errorf("从正文流中拷贝了 %d 个字节而不是 %d 个字节", n, size)
	}
	return err
}

// WriteBodyChunked 将 r 分块写入 w。
func WriteBodyChunked(w network.Writer, r io.Reader) error {
	vBuf := utils.CopyBufPool.Get()
	buf := vBuf.([]byte)

	var err error
	var n int
	for {
		n, err = r.Read(buf)
		if n == 0 {
			if err == nil {
				panic("BUG: io.Reader 返回了 (0, nil)")
			}
			if err == io.EOF {
				if err = WriteChunk(w, buf[:0], true); err != nil {
					break
				}
				err = nil
			}
			break
		}
		if err = WriteChunk(w, buf[:n], true); err != nil {
			break
		}
	}

	utils.CopyBufPool.Put(vBuf)
	return err
}

// WriteChunk 将数据 b 分块写入 w 。
func WriteChunk(w network.Writer, b []byte, withFlush bool) (err error) {
	n := len(b)
	if err = bytesconv.WriteHexInt(w, n); err != nil {
		return err
	}

	w.WriteBinary(bytestr.StrCRLF)
	if _, err = w.WriteBinary(b); err != nil {
		return err
	}

	// 若是区块末尾，则在写入尾部后写入 CRLF
	if n > 0 {
		w.WriteBinary(bytestr.StrCRLF)
	}

	if !withFlush {
		return nil
	}
	err = w.Flush()
	return
}

// WriteTrailer 将响应的挂车标头 t 写入 w。
func WriteTrailer(t *protocol.Trailer, w network.Writer) error {
	_, err := w.WriteBinary(t.Header())
	return err
}

func normalizeHeaderValue(ov, ob []byte, headerLength int) (nv, nb []byte, nhl int) {
	nv = ov
	length := len(ov)
	if length <= 0 {
		return
	}
	write := 0
	shrunk := 0
	lineStart := false
	for read := 0; read < length; read++ {
		c := ov[read]
		if c == '\r' || c == '\n' {
			shrunk++
			if c == '\n' {
				lineStart = true
			}
			continue
		} else if lineStart && c == '\t' {
			c = ' '
		} else {
			lineStart = false
		}
		nv[write] = c
		write++
	}

	nv = nv[:write]
	copy(ob[write:], ob[write+shrunk:])

	// 检查我们需要跳过 \r\n 还是 \n
	skip := 0
	if ob[write] == '\r' {
		if ob[write+1] == '\n' {
			skip += 2
		} else {
			skip++
		}
	} else if ob[write] == '\n' {
		skip++
	}

	nb = ob[write+skip : len(ob)-shrunk]
	nhl = headerLength - shrunk
	return
}

func stripSpace(b []byte) []byte {
	for len(b) > 0 && b[0] == ' ' {
		b = b[1:]
	}
	for len(b) > 0 && b[len(b)-1] == ' ' {
		b = b[:len(b)-1]
	}
	return b
}

func isOnlyCRLF(b []byte) bool {
	for _, ch := range b {
		if ch != '\r' && ch != '\n' {
			return false
		}
	}
	return true
}

func readBodyIdentity(r network.Reader, maxBodySize int, dst []byte) ([]byte, error) {
	dst = dst[:cap(dst)]
	if len(dst) == 0 {
		dst = make([]byte, 1024)
	}
	offset := 0
	for {
		nn := r.Len()

		if nn == 0 {
			_, err := r.Peek(1)
			if err != nil {
				return dst[:offset], nil
			}
			nn = r.Len()
		}
		if nn >= (len(dst) - offset) {
			nn = len(dst) - offset
		}

		buf, err := r.Peek(nn)
		if err != nil {
			return dst[:offset], err
		}
		copy(dst[offset:], buf)
		r.Skip(nn)

		offset += nn
		if maxBodySize > 0 && offset > maxBodySize {
			return dst[:offset], errBodyTooLarge
		}
		if len(dst) == offset {
			n := round2(2 * offset)
			if maxBodySize > 0 && n > maxBodySize {
				n = maxBodySize + 1
			}
			b := make([]byte, n)
			copy(b, dst)
			dst = b
		}
	}
}

// 将 r 分块读取至 dst。
func readBodyChunked(r network.Reader, maxBodySize int, dst []byte) ([]byte, error) {
	if len(dst) > 0 {
		panic("BUG: 期望零长度缓冲区")
	}

	strCRLFLen := len(bytestr.StrCRLF)
	for {
		chunkSize, err := utils.ParseChunkSize(r)
		if err != nil {
			return dst, err
		}
		// 若是块尾，在读取 trailer 之后读取 CRLF
		if chunkSize == 0 {
			return dst, nil
		}
		if maxBodySize > 0 && len(dst)+chunkSize > maxBodySize {
			return dst, errBodyTooLarge
		}
		dst, err = appendBodyFixedSize(r, dst, chunkSize+strCRLFLen)
		if err != nil {
			return dst, err
		}
		if !bytes.Equal(dst[len(dst)-strCRLFLen:], bytestr.StrCRLF) {
			return dst, errBrokenChunk
		}
		dst = dst[:len(dst)-strCRLFLen]
	}
}

func appendBodyFixedSize(r network.Reader, dst []byte, n int) ([]byte, error) {
	if n == 0 {
		return dst, nil
	}

	offset := len(dst)
	dstLen := offset + n
	// 容量不足，则两倍扩容
	if cap(dst) < dstLen {
		b := make([]byte, round2(dstLen))
		copy(b, dst)
		dst = b
	}
	dst = dst[:dstLen]

	// Peek 可获所有数据，否则会出错
	buf, err := r.Peek(n)
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return dst[:offset], err
	}
	copy(dst[offset:], buf)
	r.Skip(len(buf))
	return dst, nil
}

func round2(n int) int {
	if n <= 0 {
		return 0
	}
	n--
	x := uint(0)
	for n > 0 {
		n >>= 1
		x++
	}
	return 1 << x
}

func tryReadTrailer(t *protocol.Trailer, r network.Reader, n int) error {
	b, err := r.Peek(n)
	if len(b) == 0 {
		// 若超时则返回 ErrTimeout
		if err != nil && strings.Contains(err.Error(), "timeout") {
			return errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "读取响应头")
		}

		if n == 1 || err == io.EOF {
			return io.EOF
		}

		return errs.NewPublicf("读取请求挂车失败: %w", err)
	}

	b = MustPeekBuffered(r)
	headersLen, errParse := parseTrailer(t, b)
	if errParse != nil {
		if err == io.EOF {
			return err
		}
		return HeaderError("response", err, errParse, b)
	}
	MustDiscard(r, headersLen)
	return nil
}

func parseTrailer(t *protocol.Trailer, buf []byte) (int, error) {
	// 跳过任何长度为 0 的区块
	if buf[0] == '0' {
		skip := len(bytestr.StrCRLF) + 1
		if len(buf) < skip {
			return 0, io.EOF
		}
		buf = buf[skip:]
	}

	var s HeaderScanner
	s.B = buf
	s.DisableNormalizing = t.IsDisableNormalizing()
	var err error
	for s.Next() {
		if len(s.Key) > 0 {
			// 键名不能包含空格和制表符
			if bytes.IndexByte(s.Key, ' ') != -1 || bytes.IndexByte(s.Key, '\t') != -1 {
				err = fmt.Errorf("trailer 键名不能包含空格和制表符 %s", s.Key)
				continue
			}
			err = t.UpdateArgBytes(s.Key, s.Value)
		}
	}
	if s.Err != nil {
		return 0, s.Err
	}
	if err != nil {
		return 0, err
	}
	return s.HLen, nil
}

func trySkipTrailer(r network.Reader, n int) error {
	b, err := r.Peek(n)
	if len(b) == 0 {
		// Return ErrTimeout on any timeout.
		if err != nil && strings.Contains(err.Error(), "timeout") {
			return errs.New(errs.ErrTimeout, errs.ErrorTypePublic, "读取响应头")
		}

		if n == 1 || err == io.EOF {
			return io.EOF
		}

		return errs.NewPublicf("error when reading request trailer: %w", err)
	}
	b = MustPeekBuffered(r)
	headersLen, errParse := skipTrailer(b)
	if errParse != nil {
		if err == io.EOF {
			return err
		}
		return HeaderError("response", err, errParse, b)
	}
	MustDiscard(r, headersLen)
	return nil
}

func skipTrailer(buf []byte) (int, error) {
	skip := 0
	strCRLFLen := len(bytestr.StrCRLF)
	for {
		index := bytes.Index(buf, bytestr.StrCRLF)
		if index == -1 {
			return 0, errs.ErrNeedMore
		}

		buf = buf[index+strCRLFLen:]
		skip += index + strCRLFLen

		if index == 0 {
			return skip, nil
		}
	}
}
