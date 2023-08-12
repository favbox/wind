package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const size1K = 1024

func TestConvertNetworkWriter(t *testing.T) {
	iw := &mockIOWriter{}
	w := NewWriter(iw)
	nw, _ := w.(*networkWriter)

	// 测试内存分配
	buf, _ := w.Malloc(size1K)
	assert.Equal(t, len(buf), size1K)
	assert.Equal(t, len(nw.caches), 1)
	err := w.Flush()
	assert.Nil(t, err)
	assert.Equal(t, size1K, iw.WriteNum)
	assert.Equal(t, len(nw.caches), 0)
	assert.Equal(t, cap(nw.caches), 1)

	// 测试分配更多内存
	buf, _ = w.Malloc(size1K + 1)
	assert.Equal(t, len(buf), size1K+1)
	assert.Equal(t, len(nw.caches), 1)
	assert.Equal(t, len(nw.caches[0].data), size1K+1)
	assert.Equal(t, cap(nw.caches[0].data), size1K*2)
	buf, _ = w.Malloc(size1K / 2)
	assert.Equal(t, len(buf), size1K/2)
	assert.Equal(t, len(nw.caches), 1)
	assert.Equal(t, len(nw.caches[0].data), size1K+1+size1K/2)
	assert.Equal(t, cap(nw.caches[0].data), size1K*2)
	buf, _ = w.Malloc(size1K / 2)
	assert.Equal(t, len(buf), size1K/2)
	assert.Equal(t, len(nw.caches), 2)
	assert.Equal(t, len(nw.caches[0].data), size1K+1+size1K/2)
	assert.Equal(t, cap(nw.caches[0].data), size1K*2)
	assert.Equal(t, len(nw.caches[1].data), size1K/2)
	assert.Equal(t, cap(nw.caches[1].data), size1K/2)
	err = w.Flush()
	assert.Nil(t, err)
	assert.Equal(t, size1K*3+1, iw.WriteNum)
	assert.Equal(t, len(nw.caches), 0)
	assert.Equal(t, cap(nw.caches), 2)

	// 测试将数据写入内存
	buf, _ = w.Malloc(size1K * 6)
	assert.Equal(t, len(buf), size1K*6)
	assert.Equal(t, len(nw.caches[0].data), size1K*6)
	b := make([]byte, size1K)
	w.WriteBinary(b)
	assert.Equal(t, size1K*3+1, iw.WriteNum)
	assert.Equal(t, len(nw.caches[0].data), size1K*7)
	assert.Equal(t, cap(nw.caches[0].data), size1K*8)

	b = make([]byte, size1K*4)
	w.WriteBinary(b)
	assert.Equal(t, len(nw.caches[1].data), size1K*4)
	assert.Equal(t, cap(nw.caches[1].data), size1K*4)
	assert.Equal(t, nw.caches[1].readOnly, true)
	w.Flush()
	assert.Equal(t, size1K*14+1, iw.WriteNum)
}

type mockIOWriter struct {
	WriteNum int
}

// 记录已写入字节切片的总长度，并返回当前写入切片的长度。
func (iw *mockIOWriter) Write(p []byte) (n int, err error) {
	iw.WriteNum += len(p)
	return len(p), nil
}
