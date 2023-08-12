package network

import (
	"io"
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
)

const size4K = 4 * 1024

// 表示一个写入器的缓存节点。
type node struct {
	data     []byte
	readOnly bool
}

// 定义一个节点池。
var nodePool = sync.Pool{}

func init() {
	nodePool.New = func() any {
		return &node{}
	}
}

// 带缓存节点的写入器
type networkWriter struct {
	caches []*node // 缓存节点切片
	w      io.Writer
}

func (w *networkWriter) Malloc(length int) (buf []byte, err error) {
	// 有缓存节点
	idx := len(w.caches)
	if idx > 0 {
		idx -= 1
		inUse := len(w.caches[idx].data)
		// nodePool分配的可写节点，且有余量
		if !w.caches[idx].readOnly && cap(w.caches[idx].data)-inUse >= length {
			end := inUse + length
			w.caches[idx].data = w.caches[idx].data[:end]
			return w.caches[idx].data[inUse:end], nil
		}
	}

	// 无缓存节点，从 nodePool 中获取并分配 mcache 里的数据
	buf = mcache.Malloc(length)
	n := nodePool.Get().(*node)
	n.data = buf

	// 附加到缓存节点切片
	w.caches = append(w.caches, n)

	return
}

// WriteBinary 写入数据至缓存。
func (w *networkWriter) WriteBinary(b []byte) (length int, err error) {
	length = len(b)

	// 小切片：写入 mcache 并附加到缓存节点切片
	if length < size4K {
		buf, _ := w.Malloc(length)
		copy(buf, b)
		return
	}

	// 大切片：写入从 sync.Pool 获取的节点并附加到缓存节点切片
	n := nodePool.Get().(*node)
	n.readOnly = true
	n.data = b
	w.caches = append(w.caches, n)
	return
}

// Flush 将所有缓存节点的数据写入底层数据流。
func (w *networkWriter) Flush() (err error) {
	// 写入底层数据流。
	for _, n := range w.caches {
		_, err = w.w.Write(n.data)
		if err != nil {
			break
		}
	}
	// 释放内存、重置缓存切片。
	w.release()
	return
}

// 释放内存并重置缓存节点切片。
func (w *networkWriter) release() {
	for _, n := range w.caches {
		// 处理 mcache 分配的内存块
		if !n.readOnly {
			mcache.Free(n.data)
		}
		n.data = nil
		n.readOnly = false
		// 将节点放回池中以供复用
		nodePool.Put(n)
	}

	// 清空缓存节点
	w.caches = w.caches[:0]
}

// NewWriter 将 io.Writer 转为缓冲写入器。
func NewWriter(w io.Writer) Writer {
	return &networkWriter{
		w: w,
	}
}

type ExtWriter interface {
	io.Writer
	Flush() error

	// Finalize 将在 Writer 被释放之前被框架调用。
	// 实现方必须保证 Finalize 对于多次调用是安全的。
	Finalize() error
}
