package ext

import (
	"bytes"
	"io"
	"sync"

	"github.com/favbox/wind/common/bytebufferpool"
	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/utils"
	"github.com/favbox/wind/internal/bytestr"
	"github.com/favbox/wind/network"
	"github.com/favbox/wind/protocol"
)

var (
	errChunkedStream = errs.New(errs.ErrChunkedStream, errs.ErrorTypePublic, nil)

	bodyStreamPool = sync.Pool{
		New: func() any {
			return &bodyStream{}
		},
	}
)

type bodyStream struct {
	prefetchedBytes *bytes.Reader // 预取的字节
	reader          network.Reader
	trailer         *protocol.Trailer
	offset          int
	contentLength   int
	chunkLeft       int
	chunkEOF        bool // 块是否已触底
}

func (bs *bodyStream) Read(p []byte) (int, error) {
	defer func() {
		if bs.reader != nil {
			bs.reader.Release()
		}
	}()

	if bs.contentLength == -1 {
		if bs.chunkEOF {
			return 0, io.EOF
		}

		if bs.chunkLeft == 0 {
			chunkSize, err := utils.ParseChunkSize(bs.reader)
			if err != nil {
				return 0, err
			}
			if chunkSize == 0 {
				err = ReadTrailer(bs.trailer, bs.reader)
				if err == nil {
					bs.chunkEOF = true
					err = io.EOF
				}
				return 0, err
			}

			bs.chunkLeft = chunkSize
		}
		bytesToRead := len(p)

		if bytesToRead > bs.chunkLeft {
			bytesToRead = bs.chunkLeft
		}

		src, err := bs.reader.Peek(bytesToRead)
		copied := copy(p, src)
		bs.reader.Skip(copied)
		bs.chunkLeft -= copied

		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return copied, err
		}

		if bs.chunkLeft == 0 {
			err = utils.SkipCRLF(bs.reader)
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
		}

		return copied, err
	}

	if bs.offset == bs.contentLength {
		return 0, io.EOF
	}

	var n int
	var err error
	// 从预读缓冲区读取
	if int(bs.prefetchedBytes.Size()) > bs.offset {
		n, err = bs.prefetchedBytes.Read(p)
		bs.offset += n
		if bs.offset == bs.contentLength {
			return n, io.EOF
		}
		if err != nil || len(p) == n {
			return n, err
		}
	}

	// 从 wire 读取
	m := len(p) - n
	remain := bs.contentLength - bs.offset
	if m > remain {
		m = remain
	}

	if conn, ok := bs.reader.(io.Reader); ok {
		m, err = conn.Read(p[n:])
	} else {
		var tmp []byte
		tmp, err = bs.reader.Peek(m)
		m = copy(p[n:], tmp)
		bs.reader.Skip(m)
	}
	bs.offset += m
	n += m

	if err != nil {
		// 流数据可能不完整
		if err == io.EOF {
			if bs.offset != bs.contentLength && bs.contentLength != -2 {
				err = io.ErrUnexpectedEOF
			}
			// 确保 skipREset 好使
			bs.offset = bs.contentLength
		}
		return n, err
	}

	if bs.offset == bs.contentLength {
		err = io.EOF
	}
	return n, err
}

func (bs *bodyStream) skipRest() error {
	// 正文长度未超过 maxContentLengthInStream 或正文流已跳过剩余部分。
	if bs.prefetchedBytes == nil {
		return nil
	}

	// 请求为分块编码
	if bs.contentLength == -1 {
		if bs.chunkEOF {
			return nil
		}

		strCRLFLen := len(bytestr.StrCRLF)
		for {
			chunkSize, err := utils.ParseChunkSize(bs.reader)
			if err != nil {
				return err
			}

			if chunkSize == 0 {
				bs.chunkEOF = true
				return SkipTrailer(bs.reader)
			}

			err = bs.reader.Skip(chunkSize)
			if err != nil {
				return err
			}

			crlf, err := bs.reader.Peek(strCRLFLen)
			if err != nil {
				return err
			}

			if !bytes.Equal(crlf, bytestr.StrCRLF) {
				return errBrokenChunk
			}

			err = bs.reader.Skip(strCRLFLen)
			if err != nil {
				return err
			}
		}
	}

	// pSize 最大值为 8193，它是安全的。
	pSize := int(bs.prefetchedBytes.Size())
	if bs.contentLength <= pSize || bs.offset == bs.contentLength {
		return nil
	}

	needSkipLen := 0
	if bs.offset > pSize {
		needSkipLen = bs.contentLength - bs.offset
	} else {
		needSkipLen = bs.contentLength - pSize
	}

	// 必须跳过的尺寸
	for {
		skip := bs.reader.Len()
		if skip == 0 {
			_, err := bs.reader.Peek(1)
			if err != nil {
				return err
			}
			skip = bs.reader.Len()
		}
		if skip > needSkipLen {
			skip = needSkipLen
		}
		bs.reader.Skip(skip)
		needSkipLen -= skip
		if needSkipLen == 0 {
			return nil
		}
	}
}

func (bs *bodyStream) reset() {
	bs.prefetchedBytes = nil
	bs.offset = 0
	bs.reader = nil
	bs.trailer = nil
	bs.chunkEOF = false
	bs.chunkLeft = 0
	bs.contentLength = 0
}

// ReadBodyWithStreaming 从网络读取器流式读取数据并返回。
//
// 注意：
//
//  1. 内容长度 == -1 表明分块错误
//  2. 内容长度  >  0 取最小字节数  maxBodySize < contentLength < 8KB，若maxBodySize最小但contentLength>maxBodySize则报错过大
//  3. 内容长度 <  -1 表明完整阅读
func ReadBodyWithStreaming(zr network.Reader, contentLength, maxBodySize int, dst []byte) (b []byte, err error) {
	if contentLength == -1 {
		return b, errChunkedStream
	}
	dst = dst[:0]

	if maxBodySize <= 0 {
		maxBodySize = maxContentLengthInStream
	}

	readN := maxBodySize
	if readN > contentLength {
		readN = contentLength
	}
	if readN > maxContentLengthInStream {
		readN = maxContentLengthInStream
	}

	if contentLength >= 0 && maxBodySize >= contentLength {
		b, err = appendBodyFixedSize(zr, dst, readN)
	} else {
		b, err = readBodyIdentity(zr, readN, dst)
	}

	if err != nil {
		return b, err
	}
	if contentLength > maxBodySize {
		return b, errBodyTooLarge
	}
	return b, nil
}

// AcquireBodyStream 从正文流的池中获取一个指定信息的实例。
func AcquireBodyStream(b *bytebufferpool.ByteBuffer, r network.Reader, t *protocol.Trailer, contentLength int) io.Reader {
	bs := bodyStreamPool.Get().(*bodyStream)
	bs.prefetchedBytes = bytes.NewReader(b.B)
	bs.reader = r
	bs.contentLength = contentLength
	bs.trailer = t
	bs.chunkEOF = false

	return bs
}

// ReleaseBodyStream 释放正文流。
// 如果存在 skipRest 错误则会返回。
//
// 注意：除非你知道该函数的用途，否则要小心使用。
func ReleaseBodyStream(requestReader io.Reader) (err error) {
	if bs, ok := requestReader.(*bodyStream); ok {
		err = bs.skipRest()
		bs.reset()
		bodyStreamPool.Put(bs)
	}
	return
}
