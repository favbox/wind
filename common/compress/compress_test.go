package compress

import (
	"io"
	"testing"
)

func TestCompressNewCompressWriterPoolMap(t *testing.T) {
	pool := newCompressWriterPoolMap()
	if len(pool) != 12 {
		t.Fatalf("不期望的 WriterPoolMap 数量：%d。 期望 12", len(pool))
	}
}

func TestCompressAppendGunzipBytes(t *testing.T) {
	dst1 := []byte("")
	// src 解压缩 -> "hello"。src 必须是已压缩后的字符串。
	src1 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 0, 0, 0, 255, 255}
	expectedRes1 := "hello"
	res1, err1 := AppendGunzipBytes(dst1, src1)
	// gzip will wrap io.EOF to io.ErrUnexpectedEOF
	// just ignore in this case
	if err1 != io.ErrUnexpectedEOF {
		t.Fatalf("Unexpected error: %s", err1)
	}
	if string(res1) != expectedRes1 {
		t.Fatalf("Unexpected : %s. Expecting : %s", res1, expectedRes1)
	}

	dst2 := []byte("!!!")
	src2 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 0, 0, 0, 255, 255}
	expectedRes2 := "!!!hello"
	res2, err2 := AppendGunzipBytes(dst2, src2)
	if err2 != io.ErrUnexpectedEOF {
		t.Fatalf("Unexpected error: %s", err2)
	}
	if string(res2) != expectedRes2 {
		t.Fatalf("Unexpected : %s. Expecting : %s", res2, expectedRes2)
	}

	dst3 := []byte("!!!")
	src3 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 0, 0, 0, 255, 255}
	expectedRes3 := "!!!"
	res3, err3 := AppendGunzipBytes(dst3, src3)
	if err3 != io.ErrUnexpectedEOF {
		t.Fatalf("Unexpected error: %s", err3)
	}
	if string(res3) != expectedRes3 {
		t.Fatalf("Unexpected : %s. Expecting : %s", res3, expectedRes3)
	}
}

func TestCompressAppendGzipBytesLevel(t *testing.T) {
	// 测试 WriteGzipLevel 的 byteSliceWriter 用例
	dst1 := []byte("")
	src1 := []byte("hello")
	res1 := AppendGzipBytesLevel(dst1, src1, 5)
	expectedRes1 := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 4, 0, 0, 255, 255, 134, 166, 16, 54, 5, 0, 0, 0}
	if string(res1) != string(expectedRes1) {
		t.Fatalf("不期待： %s. 期待：%s", res1, expectedRes1)
	}
}

func TestCompressWriteGzipLevel(t *testing.T) {
	// 测试 WriteGzipLevel 的默认用例
	var w defaultByteWriter
	p := []byte("hello")
	expectedW := []byte{31, 139, 8, 0, 0, 0, 0, 0, 0, 255, 202, 72, 205, 201, 201, 7, 4, 0, 0, 255, 255, 134, 166, 16, 54, 5, 0, 0, 0}
	num, err := WriteGzipLevel(&w, p, 5)
	if string(expectedW) != string(w.b) {
		t.Fatalf("不期待：%s. 期待：%s.", w.b, expectedW)
	}
	if num != len(p) {
		t.Fatalf("Unexpected number of compressed bytes: %d", num)
	}
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

type defaultByteWriter struct {
	b []byte
}

func (w *defaultByteWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}
