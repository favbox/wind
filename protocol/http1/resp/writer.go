package resp

import (
	"sync"

	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/http1/ext"
)

var _ network.ExtWriter = (*chunkedBodyWriter)(nil)

// 分块正文写入器。
type chunkedBodyWriter struct {
	sync.Once
	finalizeErr error
	wroteHeader bool
	r           *protocol.Response
	w           network.Writer
}

// 分块写入 p。
func (c *chunkedBodyWriter) Write(p []byte) (n int, err error) {
	if !c.wroteHeader {
		c.r.Header.SetContentLength(-1) // -1 意为分块传输
		if err = WriteHeader(&c.r.Header, c.w); err != nil {
			return
		}
		c.wroteHeader = true
	}
	if err = ext.WriteChunk(c.w, p, false); err != nil {
		return
	}
	return len(p), nil
}

// Flush 将数据刷新至对端。
func (c *chunkedBodyWriter) Flush() error {
	return c.w.Flush()
}

// Finalize 将写入结束块和结尾部分并刷新写入器。警告：不懂别用。
func (c *chunkedBodyWriter) Finalize() error {
	c.Do(func() {
		c.finalizeErr = ext.WriteChunk(c.w, nil, true)
		if c.finalizeErr != nil {
			return
		}
		c.finalizeErr = ext.WriteTrailer(c.r.Header.Trailer(), c.w)
	})
	return c.finalizeErr
}

func NewChunkedBodyWriter(r *protocol.Response, w network.Writer) network.ExtWriter {
	return &chunkedBodyWriter{
		r: r,
		w: w,
	}
}
