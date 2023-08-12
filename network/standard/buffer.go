package standard

import (
	"sync"

	"github.com/bytedance/gopkg/lang/mcache"
)

var bufferPool = sync.Pool{}

func init() {
	bufferPool.New = func() any {
		return &linkBufferNode{}
	}
}

// 链式缓冲区的节点
type linkBufferNode struct {
	buf      []byte          // 缓冲区
	off      int             // 读偏移量
	malloc   int             // 写偏移量
	next     *linkBufferNode // 链式缓冲区的下一个节点
	readOnly bool
}

// 链式缓冲区
type linkBuffer struct {
	head  *linkBufferNode // 头节点（用于释放）
	read  *linkBufferNode // 读节点（当前读）
	write *linkBufferNode // 尾节点
	len   int
}

func (l *linkBuffer) release() {
	for l.head != nil {
		node := l.head
		l.head = l.head.next
		node.Release()
	}
}

// 创建指定缓冲大小的节点。
func newBufferNode(size int) *linkBufferNode {
	node := bufferPool.Get().(*linkBufferNode)
	node.buf = malloc(size, size)
	return node
}

// Reset 重置该节点。
//
// 注意：不会回收节点的缓冲区。
func (n *linkBufferNode) Reset() {
	n.buf = n.buf[:0]
	n.off, n.malloc = 0, 0
	n.readOnly = false
}

// Release 回收节点的缓冲区。
func (n *linkBufferNode) Release() {
	if !n.readOnly {
		free(n.buf)
	}
	n.readOnly = false
	n.buf = nil
	n.next = nil
	n.malloc, n.off = 0, 0
	bufferPool.Put(n)
}

func (n *linkBufferNode) recyclable() bool {
	return cap(n.buf) <= block8k && !n.readOnly
}

// Len 计算该节点的字节数。
func (n *linkBufferNode) Len() int {
	return n.malloc - n.off
}

// Cap 返回节点缓冲区的容量。
func (n *linkBufferNode) Cap() int {
	return cap(n.buf) - n.malloc
}

// 分配初始 size 和最小容量 capacity 的字节切片。
// 若容量 capacity 超过了最大的分配大小 mallocMax 则不适用 mcache 缓存池。
func malloc(size, capacity int) []byte {
	if capacity > mallocMax {
		return make([]byte, size, capacity)
	}
	return mcache.Malloc(size, capacity)
}

// 释放 mcache 创建的缓冲区。
func free(buf []byte) {
	// 非缓存池分配的 buf
	if cap(buf) > mallocMax {
		return
	}

	// 释放缓存池分配的 buf
	mcache.Free(buf)
}
