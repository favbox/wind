package resp

import (
	"sync"

	"github.com/favbox/gosky/wind/pkg/network"
	"github.com/favbox/gosky/wind/pkg/protocol"
	"github.com/favbox/gosky/wind/pkg/protocol/http1/ext"
)

var _ network.ExtWriter = (*chunkedBodyWriter)(nil)

type chunkedBodyWriter struct {
	sync.Once
	finalizeErr error
	wroteHeader bool
	r           *protocol.Response
	w           network.Writer
}

// 将在写入之前对分块数据 p 进行编码。
// 若写入成功则返回 p 的长度。
//
// 注意：Write 将使用用户缓冲区进行刷新。
// 刷新成功之前，需确保缓冲区可用。
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

func (c *chunkedBodyWriter) Flush() error {
	return c.w.Flush()
}

// Finalize 将写入结束块和结尾部分并刷新写入器。
// 警告：不要自己调用该方法，除非你知道自己在做什么。
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
