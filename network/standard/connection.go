package standard

import (
	"crypto/tls"
	"errors"
	"io"
	"net"
	"runtime"
	"syscall"
	"time"

	errs "github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/network"
)

const (
	block1k                  = 1024
	block4k                  = 4096
	block8k                  = 8192
	mallocMax                = 512 * block1k
	defaultMallocSize        = block4k
	maxConsecutiveEmptyReads = 100 // 最大连续空读取次数
)

// Conn 实现基于 net 的网络连接。
type Conn struct {
	c            net.Conn
	inputBuffer  *linkBuffer
	outputBuffer *linkBuffer
	caches       [][]byte // 跨包时由 Next 分配，不用时要释放
	maxSize      int      // 历史最大 malloc 大小

	err error
}

// --- 实现 network.ErrorNormalization ---

func (c *Conn) ToWindError(err error) error {
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ENOTCONN) {
		return errs.ErrConnectionClosed
	}

	// 统一超时错误
	if netErr, ok := err.(*net.OpError); ok && netErr.Timeout() {
		return errs.ErrTimeout
	}

	return err
}

// --- 实现 network.Conn ---

func (c *Conn) Read(b []byte) (n int, err error) {
	n = c.Len()
	// 若 inputBuffer 中有数据，则拷到 b 并返回
	if n > 0 {
		n = min(n, len(b))
		return n, c.next(n, b)
	}

	// 若剩余缓冲区小于 4k，则先 Peek(1) 填充缓冲区，然后将 min(c.Len, len(b)) 拷到 b。
	if len(b) <= block4k {
		// 若 c.fill(1) 出错，则 conn.Read 必须返回 0, err。
		// 故此无需检查 c.Len
		err = c.fill(1)
		if err != nil {
			return 0, err
		}
		n = min(c.Len(), len(b))
		return n, c.next(n, b)
	}

	// 调用标准库连接直接 Read 到缓冲区 b
	return c.c.Read(b)
}

// 调用标准库直接将数据写入连接。
func (c *Conn) Write(b []byte) (n int, err error) {
	if err = c.Flush(); err != nil {
		return
	}
	return c.c.Write(b)
}

// ReadFrom 实现 io.ReaderFrom。若底层写入器支持 ReadFrom 方法，
// 而且 c 尚无缓冲数据，则调用底层 ReadFrom 而不进行缓冲。
func (c *Conn) ReadFrom(r io.Reader) (n int64, err error) {
	if err = c.Flush(); err != nil {
		return
	}

	if w, ok := c.c.(io.ReaderFrom); ok {
		n, err = w.ReadFrom(r)
		return
	}

	var m int
	bufNode := c.outputBuffer.write

	// if there is no available buffer, create one.
	if !bufNode.recyclable() || cap(bufNode.buf) == 0 {
		c.Malloc(block4k)
		c.outputBuffer.write.Reset()
		c.outputBuffer.len = cap(c.outputBuffer.write.buf)
		bufNode = c.outputBuffer.write
	}

	for {
		if bufNode.Cap() == 0 {
			if err1 := c.Flush(); err1 != nil {
				return n, err1
			}
		}

		nr := 0
		for nr < maxConsecutiveEmptyReads {
			m, err = r.Read(bufNode.buf[bufNode.malloc:cap(bufNode.buf)])
			if m != 0 || err != nil {
				break
			}
			nr++
		}
		if nr == maxConsecutiveEmptyReads {
			return n, io.ErrNoProgress
		}
		bufNode.malloc += m
		n += int64(m)
		if err != nil {
			break
		}
	}
	if err == io.EOF {
		// If we filled the buffer exactly, flush preemptively.
		if bufNode.Cap() == 0 {
			err = c.Flush()
		} else {
			err = nil
			// Update buffer available length for next Malloc
			c.outputBuffer.len = bufNode.Cap()
		}
	}
	return
}

// Close 关闭连接。
func (c *Conn) Close() error {
	return c.c.Close()
}

// CloseNoResetBuffer 关闭连接但不重置缓冲区。
func (c *Conn) CloseNoResetBuffer() error {
	return c.c.Close()
}

func (c *Conn) HandleSpecificError(err error, rip string) (needIgnore bool) {
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		wlog.SystemLogger().Debugf("Go 网络库 错误=%s, 远程地址=%s", err.Error(), rip)
		return true
	}
	return false
}

func (c *Conn) LocalAddr() net.Addr {
	return c.c.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.c.RemoteAddr()
}

func (c *Conn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.c.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.c.SetWriteDeadline(t)
}

// Len 输入缓冲区的可读字节数
func (c *Conn) Len() int {
	return c.inputBuffer.len
}

// Peek 返回接下来的 n 个字节，而不移动读指针。
func (c *Conn) Peek(n int) (p []byte, err error) {
	node := c.inputBuffer.read
	// 填充 c.inputBuffer 以便有足够的数据
	err = c.fill(n)
	if err != nil {
		return
	}

	if c.Len() < n {
		n = c.Len()
		err = c.readErr()
	}

	l := node.Len()
	// 单节点中有足够数据，故直接返回节点的切片即可。
	if l >= n {
		return node.buf[node.off : node.off+n], err
	}

	// 单节点中没有足够数据
	if block1k < n && n <= mallocMax {
		// 小包 (1k, 512k] 由 mcache 进行池化分配
		p = malloc(n, n)
		c.caches = append(c.caches, p)
	} else {
		// 大包由 make 分配
		p = make([]byte, n)
	}
	c.peekBuffer(n, p)
	return p, err
}

func (c *Conn) Skip(n int) error {
	// 检查是否有足够字节
	if c.Len() < n {
		return errs.NewPrivatef("链式缓冲区的现有长度不足以跳过 %d 个字节", n)
	}
	c.inputBuffer.len -= n // 重算长度

	var l int
	for ack := n; ack > 0; ack = ack - l {
		l = c.inputBuffer.read.Len()
		if l >= ack {
			c.inputBuffer.read.off += ack
			break
		}
		c.inputBuffer.read = c.inputBuffer.read.next
	}
	return nil
}

func (c *Conn) ReadByte() (byte, error) {
	b, err := c.Peek(1)
	if err != nil {
		return ' ', err
	}
	err = c.Skip(1)
	if err != nil {
		return ' ', err
	}
	return b[0], nil
}

func (c *Conn) ReadBinary(n int) ([]byte, error) {
	out := make([]byte, n)
	b, err := c.Peek(n)
	if err != nil {
		return nil, err
	}
	copy(out, b)
	err = c.Skip(n)
	return out, err
}

// Release 释放链式缓冲区。
//
// 注意：该函数只用于 inputBuffer。
func (c *Conn) Release() error {
	// 用 c.Len() 来检查数据是否已被完全读取。
	// 若 inputBuffer 还有数据，不能用 head 和 write 来检查当前节点是否可释放。
	// 应当用 head 和 read 来判断连接。
	if c.Len() == 0 {
		// 重置缓冲，以便重用
		// 该情况下，请求可保存在一个节点中。我们只需重置该节点来保存下一个请求。
		//
		// 注意：每个链接都将绑定一个缓冲区。我们需要关心内存的用况。
		if c.inputBuffer.head == c.inputBuffer.write {
			c.inputBuffer.write.Reset()
			return nil
		}

		// 缓冲区是否足够大到容纳整个请求的关键条件。
		// 该情况下，head 保存最后一个请求，且当前请求已保存在 write 节点。
		// 所以我们只需释放 head 节点并重置 write 节点。
		if c.inputBuffer.head.next == c.inputBuffer.write {
			// 重算 maxSize
			size := c.inputBuffer.head.malloc
			node := c.inputBuffer.head
			node.Release()
			size += c.inputBuffer.write.malloc
			if size > mallocMax {
				size = mallocMax
			}
			if size > c.maxSize {
				c.maxSize = size
			}
			c.handleTail()
			c.inputBuffer.head, c.inputBuffer.read = c.inputBuffer.write, c.inputBuffer.write
			c.releaseCaches()
			return nil
		}
	}

	// 若缓冲区还有数据，表明请求还没未完全处理。
	// 或者请求太大而无法保存在单个节点中。
	// 跨多个节点。
	size := 0
	for c.inputBuffer.head != c.inputBuffer.read {
		node := c.inputBuffer.head
		c.inputBuffer.head = c.inputBuffer.head.next
		size += c.inputBuffer.head.malloc
		node.Release()
	}
	// readOnly 字段仅用于 malloc 一个新节点以便保存下一个请求。
	// 它与释放逻辑无关。
	c.inputBuffer.write.readOnly = true
	if size > mallocMax {
		size = mallocMax
	}
	if size > c.maxSize {
		c.maxSize = size
	}
	c.releaseCaches()
	return nil
}

func (c *Conn) Malloc(n int) (buf []byte, err error) {
	if n == 0 {
		return
	}

	// 若当前缓冲区的容量大于我们所需，
	// 就没有必要再去申请新节点了
	if c.outputBuffer.len > n {
		node := c.outputBuffer.write
		m := node.malloc
		node.malloc += n
		node.buf = node.buf[:node.malloc]
		c.outputBuffer.len -= n
		return node.buf[m:node.malloc], nil
	}

	mallocSize := n
	if n < defaultMallocSize {
		mallocSize = defaultMallocSize
	}
	node := newBufferNode(mallocSize)
	node.malloc = n
	c.outputBuffer.len = cap(node.buf) - n
	c.outputBuffer.write.next = node
	c.outputBuffer.write = c.outputBuffer.write.next
	return node.buf[:n], nil
}

func (c *Conn) WriteBinary(b []byte) (n int, err error) {
	// 若数据少于 4k，只需拷到 outputBuffer。
	if len(b) < block4k {
		buf, err := c.Malloc(len(b))
		if err != nil {
			return 0, err
		}
		return copy(buf, b), nil
	}

	// 用缓冲区 b 构建一个新节点
	node := newBufferNode(0)
	node.malloc = len(b)
	node.readOnly = true
	node.buf = b
	c.outputBuffer.write.next = node
	c.outputBuffer.write = c.outputBuffer.write.next
	c.outputBuffer.len = 0
	return len(b), nil
}

func (c *Conn) Flush() error {
	// 没待刷数据
	if c.outputBuffer.head == c.outputBuffer.write && c.outputBuffer.head.Len() == 0 {
		return nil
	}

	// 当前节点是最后一个请求的尾节点，就移到下一个节点。
	if c.outputBuffer.head.Len() == 0 {
		node := c.outputBuffer.head
		c.outputBuffer.head = c.outputBuffer.head.next
		node.Release()
	}

	for {
		n, err := c.c.Write(c.outputBuffer.head.buf[c.outputBuffer.head.off:c.outputBuffer.head.malloc])
		if err != nil {
			return err
		}
		c.outputBuffer.head.off += n
		if c.outputBuffer.head == c.outputBuffer.write {
			// 若缓冲区容量小于 8k，就重置节点
			if c.outputBuffer.head.recyclable() {
				c.outputBuffer.head.Reset()
				c.outputBuffer.len = cap(c.outputBuffer.head.buf)
			}
			break
		}
		// 冲刷下一个节点
		node := c.outputBuffer.head
		c.outputBuffer.head = c.outputBuffer.head.next
		node.Release()
	}
	return nil
}

func (c *Conn) SetReadTimeout(t time.Duration) error {
	if t <= 0 {
		return c.c.SetReadDeadline(time.Time{})
	}
	return c.c.SetReadDeadline(time.Now().Add(t))
}

func (c *Conn) SetWriteTimeout(t time.Duration) error {
	if t <= 0 {
		return c.c.SetWriteDeadline(time.Time{})
	}
	return c.c.SetWriteDeadline(time.Now().Add(t))
}

// 读取大小为 n 的数据给 b，而不移动读指针。
func (c *Conn) peekBuffer(n int, b []byte) {
	l, pIdx, node := 0, 0, c.inputBuffer.read
	for ack := n; ack > 0; ack = ack - l {
		l = node.Len()
		if l >= ack {
			copy(b[pIdx:], node.buf[node.off:node.off+ack])
			break
		} else if l > 0 {
			pIdx += copy(b[pIdx:], node.buf[node.off:node.off+l])
		}
		node = node.next
	}
}

// 读取大小为 n 的数据给 b，然后移动读指针并释放链式缓冲区。
func (c *Conn) next(n int, b []byte) error {
	c.peekBuffer(n, b)
	err := c.Skip(n)
	if err != nil {
		return err
	}
	return c.Release()
}

func (c *Conn) fill(i int) (err error) {
	// 检查 inputBuffer 中是否有足够数据
	if c.Len() >= i {
		return nil
	}
	// 检查连接先前是否已返回错误
	if err = c.readErr(); err != nil {
		if c.Len() > 0 {
			c.err = err
			return nil
		}
		return
	}
	node := c.inputBuffer.write
	node.buf = node.buf[:cap(node.buf)]
	left := cap(node.buf) - node.malloc

	// 若剩余容量不足预期数据的长度，或是一个新请求，
	// 我们将分配一个足够的节点来保存数据
	if left < i-c.Len() || node.readOnly {
		// 无足够容量
		size := i
		if i < c.maxSize {
			size = c.maxSize
		}
		c.inputBuffer.write.next = newBufferNode(size)
		c.inputBuffer.write = c.inputBuffer.write.next
		// 将节点 readOnly 标记为 false，以便回收。
		node.readOnly = false
	}

	i -= c.Len()
	node = c.inputBuffer.write
	node.buf = node.buf[:cap(node.buf)]

	// 循环读取数据，以便节点保存足够的数据
	for i > 0 {
		n, err := c.c.Read(c.inputBuffer.write.buf[node.malloc:])
		if n > 0 {
			node.malloc += n
			c.inputBuffer.len += n
			i -= n
			if err != nil {
				c.err = err
				return nil
			}
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Conn) readErr() error {
	err := c.err
	c.err = nil
	return err
}

// 预防尾部节点过大，以确保内存使用率。
func (c *Conn) handleTail() {
	if cap(c.inputBuffer.write.buf) > mallocMax {
		node := c.inputBuffer.write
		c.inputBuffer.write.next = newBufferNode(c.maxSize)
		c.inputBuffer.write = c.inputBuffer.write.next
		node.Release()
		return
	}
	c.inputBuffer.write.Reset()
}

func (c *Conn) releaseCaches() {
	for i := range c.caches {
		free(c.caches[i])
		c.caches[i] = nil
	}
	c.caches = c.caches[:0]
}

type TLSConn struct {
	Conn
}

func (c *TLSConn) ConnectionState() tls.ConnectionState {
	return c.c.(network.ConnTLSer).ConnectionState()
}

func (c *TLSConn) Handshake() error {
	return c.c.(network.ConnTLSer).Handshake()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func newConn(c net.Conn, size int) network.Conn {
	maxSize := defaultMallocSize
	if size > maxSize {
		maxSize = size
	}

	node := newBufferNode(maxSize)
	inputBuffer := &linkBuffer{
		head:  node,
		read:  node,
		write: node,
	}
	runtime.SetFinalizer(inputBuffer, (*linkBuffer).release)

	outputNode := newBufferNode(0)
	outputBuffer := &linkBuffer{
		head:  outputNode,
		write: outputNode,
	}
	runtime.SetFinalizer(outputBuffer, (*linkBuffer).release)

	return &Conn{
		c:            c,
		inputBuffer:  inputBuffer,
		outputBuffer: outputBuffer,
		maxSize:      maxSize,
	}
}

func newTLSConn(c net.Conn, size int) network.Conn {
	maxSize := defaultMallocSize
	if size > maxSize {
		maxSize = size
	}

	node := newBufferNode(maxSize)
	inputBuffer := &linkBuffer{
		head:  node,
		read:  node,
		write: node,
	}
	runtime.SetFinalizer(inputBuffer, (*linkBuffer).release)

	outputNode := newBufferNode(0)
	outputBuffer := &linkBuffer{
		head:  outputNode,
		write: outputNode,
	}
	runtime.SetFinalizer(outputBuffer, (*linkBuffer).release)

	return &TLSConn{
		Conn{
			c:            c,
			inputBuffer:  inputBuffer,
			outputBuffer: outputBuffer,
			maxSize:      maxSize,
		},
	}
}
