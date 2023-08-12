package bytebufferpool

import (
	"io"
)

// ByteBuffer 提供字节缓冲区，以最小化内存分配。使用 Get 获取。
type ByteBuffer struct {
	// B 是字节缓冲区。
	B []byte
}

// ReadFrom 向 ByteBuffer.B 追加从 r 读取的数据。
func (b *ByteBuffer) ReadFrom(r io.Reader) (int64, error) {
	p := b.B
	nStart := int64(len(p))
	nMax := int64(cap(p))
	n := nStart
	if nMax == 0 {
		nMax = 64
		p = make([]byte, nMax)
	} else {
		p = p[:nMax]
	}
	for {
		if n == nMax {
			nMax *= 2
			bNew := make([]byte, nMax)
			copy(bNew, p)
			p = bNew
		}
		nn, err := r.Read(p[n:])
		n += int64(nn)
		if err != nil {
			b.B = p[:n]
			n -= nStart
			if err == io.EOF {
				return n, nil
			}
			return n, err
		}
	}
}

// WriteTo 将 ByteBuffer.B 写入 w。
func (b *ByteBuffer) WriteTo(w io.Writer) (int64, error) {
	n, err := w.Write(b.B) // 写入 len(b.P) 个字节
	return int64(n), err
}

// Write 向 ByteBuffer.B 追加 p。
func (b *ByteBuffer) Write(p []byte) (int, error) {
	b.B = append(b.B, p...)
	return len(p), nil
}

// WriteByte 向 ByteBuffer.B 追加 c。
func (b *ByteBuffer) WriteByte(c byte) error {
	b.B = append(b.B, c)
	return nil
}

// WriteString 向 ByteBuffer.B 追加 s。
func (b *ByteBuffer) WriteString(s string) (int, error) {
	b.B = append(b.B, s...)
	return len(s), nil
}

// SetString 将 ByteBuffer.B 设为 s。
func (b *ByteBuffer) SetString(s string) {
	b.B = append(b.B[:0], s...)
}

// Set 将 ByteBuffer.B 设为 p。
func (b *ByteBuffer) Set(p []byte) {
	b.B = append(b.B[:0], p...)
}

// Reset 让 ByteBuffer.B 为空。
func (b *ByteBuffer) Reset() {
	b.B = b.B[:0]
}

// Bytes 返回 b.B，即缓冲区累计的所有字节。
func (b *ByteBuffer) Bytes() []byte {
	return b.B
}

// Len 返回字节缓冲区的大小。
func (b *ByteBuffer) Len() int {
	return len(b.B)
}

// 返回 ByteBuffer.B 的字符串表示形式。
func (b *ByteBuffer) String() string {
	//return bytesconv.B2s(b.B)
	return string(b.B)
}
