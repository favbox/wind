package resp

import (
	"runtime"
	"sync"

	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/http1/ext"
)

var _ network.ExtWriter = (*chunkedBodyWriter)(nil)

var chunkReaderPool sync.Pool

func init() {
	chunkReaderPool = sync.Pool{
		New: func() any {
			return &chunkedBodyWriter{}
		},
	}
}

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
		// 对于没有实际数据的情况
		if !c.wroteHeader {
			c.r.Header.SetContentLength(-1)
			if c.finalizeErr = WriteHeader(&c.r.Header, c.w); c.finalizeErr != nil {
				return
			}
			c.wroteHeader = true
		}
		c.finalizeErr = ext.WriteChunk(c.w, nil, true)
		if c.finalizeErr != nil {
			return
		}
		c.finalizeErr = ext.WriteTrailer(c.r.Header.Trailer(), c.w)
	})
	return c.finalizeErr
}

func (c *chunkedBodyWriter) release() {
	c.r = nil
	c.w = nil
	c.finalizeErr = nil
	c.wroteHeader = false
	chunkReaderPool.Put(c)
}

// NewChunkedBodyWriter 创建一个分块响应体写入器。
func NewChunkedBodyWriter(r *protocol.Response, w network.Writer) network.ExtWriter {
	extWriter := chunkReaderPool.Get().(*chunkedBodyWriter)
	extWriter.r = r
	extWriter.w = w
	extWriter.Once = sync.Once{}
	runtime.SetFinalizer(extWriter, (*chunkedBodyWriter).release)
	return extWriter
}
