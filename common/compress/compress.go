package compress

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"

	"github.com/favbox/wind/common/bytebufferpool"
	"github.com/favbox/wind/common/stackless"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/network"
)

// CompressDefaultCompression 默认压缩率
const CompressDefaultCompression = 6

var gzipReaderPool sync.Pool

var (
	stacklessGzipWriterPoolMap = newCompressWriterPoolMap()
	realGzipWriterPoolMap      = newCompressWriterPoolMap()
)

// AppendGunzipBytes 解压 src 到 dst 并返回。
func AppendGunzipBytes(dst, src []byte) ([]byte, error) {
	w := &byteSliceWriter{dst}
	_, err := WriteGunzip(w, src)
	return w.b, err
}

// AppendGzipBytes 压缩 src 并附加到 dst，然后返回。
func AppendGzipBytes(dst, src []byte) []byte {
	return AppendGzipBytesLevel(dst, src, CompressDefaultCompression)
}

// AppendGzipBytesLevel 附加压缩后的 src 到 dst 并返回（使用指定的压缩级别）。
//
// 支持的压缩级别为：
//
//   - CompressNoCompression
//   - CompressBestSpeed
//   - CompressBestCompression
//   - CompressDefaultCompression
//   - CompressHuffmanOnly
func AppendGzipBytesLevel(dst, src []byte, level int) []byte {
	w := &byteSliceWriter{dst}
	_, _ = WriteGzipLevel(w, src, level)
	return w.b
}

// WriteGzipLevel 压缩 p 并写入 w（使用指定压缩级别），返回写入 w 的压缩量。
//
// 支持的压缩级别为：
//
//   - CompressNoCompression
//   - CompressBestSpeed
//   - CompressBestCompression
//   - CompressDefaultCompression
//   - CompressHuffmanOnly
func WriteGzipLevel(w io.Writer, p []byte, level int) (int, error) {
	switch w.(type) {
	case *byteSliceWriter,
		*bytes.Buffer,
		*bytebufferpool.ByteBuffer:
		// 这些写入器不能阻塞，所以我们可以 stackLessWriteGzip
		ctx := &compressCtx{
			w:     w,
			p:     p,
			level: level,
		}
		stacklessWriteGzip(ctx)
		return len(p), nil
	default:
		zw := AcquireStacklessGzipWriter(w, level)
		n, err := zw.Write(p)
		ReleaseStacklessGzipWriter(zw, level)
		return n, err
	}
}

// WriteGunzip 解压 src 到 w，并返回未写入的字节数。
func WriteGunzip(w io.Writer, p []byte) (int, error) {
	r := &byteSliceReader{p}
	zr, err := AcquireGzipReader(r)
	if err != nil {
		return 0, nil
	}
	zw := network.NewWriter(w)
	n, err := utils.CopyZeroAlloc(zw, zr)
	ReleaseGzipReader(zr)
	nn := int(n)
	if int64(nn) != n {
		return 0, fmt.Errorf("待解压数据过大: %d", n)
	}
	return nn, err
}

// AcquireStacklessGzipWriter 获取 io.Writer 的无堆栈压缩写入器。
//
// 用完记得调用 ReleaseStacklessGzipWriter 释放，以降低 GC，提高性能。
func AcquireStacklessGzipWriter(w io.Writer, level int) stackless.Writer {
	nLevel := normalizeCompressLevel(level)
	p := stacklessGzipWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		return stackless.NewWriter(w, func(w io.Writer) stackless.Writer {
			return acquireRealGzipWriter(w, level)
		})
	}
	sw := v.(stackless.Writer)
	sw.Reset(w)
	return sw
}

// ReleaseStacklessGzipWriter 释放无堆栈压缩写入器到指定级别池。
func ReleaseStacklessGzipWriter(sw stackless.Writer, level int) {
	_ = sw.Close()
	nLevel := normalizeCompressLevel(level)
	p := stacklessGzipWriterPoolMap[nLevel]
	p.Put(sw)
}

// AcquireGzipReader 获取压缩文件的 gzip 读取器，如果没有则新建一个。
//
// 记得用完调用 ReleaseGzipReader 释放并放回池中以减少内存开销。
func AcquireGzipReader(r io.Reader) (*gzip.Reader, error) {
	v := gzipReaderPool.Get()
	if v == nil {
		return gzip.NewReader(r)
	}
	zr := v.(*gzip.Reader)
	if err := zr.Reset(r); err != nil {
		return nil, err
	}
	return zr, nil
}

// ReleaseGzipReader 将不用的 zr 放回池中，以减少内存开销。
func ReleaseGzipReader(zr *gzip.Reader) {
	_ = zr.Close()
	gzipReaderPool.Put(zr)
}

// 返回基于标准库压缩级别的字典。
func newCompressWriterPoolMap() []*sync.Pool {
	// 按 https://pkg.go.dev/compress/flate#pkg-constants 的定义
	// 初始化 12 个压缩级别。
	var m []*sync.Pool
	for i := 0; i < 12; i++ {
		m = append(m, &sync.Pool{})
	}
	return m
}

// 标准化压缩级别为 [0..11]，以用作 *PoolMap 的索引。
func normalizeCompressLevel(level int) int {
	// -2 是最低压缩级别，仅哈夫曼 - CompressHuffmanOnly
	// 9 是最高压缩级别 - CompressBestCompression
	if level < -2 || level > 9 {
		level = CompressDefaultCompression
	}
	return level + 2
}

var stacklessWriteGzip = stackless.NewFunc(nonblockingWriteGzip)

func nonblockingWriteGzip(ctxv any) {
	ctx := ctxv.(*compressCtx)
	zw := acquireRealGzipWriter(ctx.w, ctx.level)

	_, err := zw.Write(ctx.p)
	if err != nil {
		panic(fmt.Sprintf("BUG: gzip.Writer.Write for len(p)=%d returned unexpected error: %s", len(ctx.p), err))
	}

	releaseRealGzipWriter(zw, ctx.level)
}

func releaseRealGzipWriter(zw *gzip.Writer, level int) {
	_ = zw.Close()
	nLevel := normalizeCompressLevel(level)
	p := realGzipWriterPoolMap[nLevel]
	p.Put(zw)
}

func acquireRealGzipWriter(w io.Writer, level int) *gzip.Writer {
	nLevel := normalizeCompressLevel(level)
	p := realGzipWriterPoolMap[nLevel]
	v := p.Get()
	if v == nil {
		zw, err := gzip.NewWriterLevel(w, level)
		if err != nil {
			panic(fmt.Sprintf("BUG: 来自 gzip.NewWriterLevel(%d) 的意外错误：%s", level, err))
		}
		return zw
	}
	zw := v.(*gzip.Writer)
	zw.Reset(w)
	return zw
}

type compressCtx struct {
	w     io.Writer
	p     []byte
	level int
}

type byteSliceWriter struct {
	b []byte
}

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

type byteSliceReader struct {
	b []byte
}

func (r *byteSliceReader) Read(p []byte) (int, error) {
	if len(r.b) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.b)
	r.b = r.b[n:]
	return n, nil
}
