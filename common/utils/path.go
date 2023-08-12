package utils

import (
	"strings"
)

// AddMissingPort 若 addr 中没有端口的话，则 TLS 添加 :443，非 TLS 添加 :80。
// 主机端口中的 IPv6 地址必须用括号括起来，如 "[::1]:80", "[::1%lo0]:80"。
func AddMissingPort(addr string, isTLS bool) string {
	if strings.IndexByte(addr, ':') >= 0 {
		endOfV6 := strings.IndexByte(addr, ']')
		// 我们不关心地址的有效性，只需检查“]”之后是否有更多字节
		if endOfV6 < len(addr)-1 {
			return addr
		}
	}
	if !isTLS {
		return addr + ":80"
	}
	return addr + ":443"
}

// CleanPath 是 URL 版本的 path.Clean。
// 它返回 p 的规范化 URL 路径，消除 . 和 .. 元素。
//
// 如下规则将被反复使用直至无需进一步处理：
//
//  1. 替换多个斜杠为一个斜杠。
//  2. 消除每一个 . 路径名元素（即当前目录）。
//  3. 消除每一个 .. 路径名元素（即父级目录）。
//  4. 消除根路径开头的 .. 即用 "/" 替换 "/.."。
//
// 若该法处理结果为空，则返回 "/"。
func CleanPath(p string) string {
	const stackBufSize = 128

	// 将空字符串转为 "/"
	if p == "" {
		return "/"
	}

	// 合理的堆栈缓冲区大小，可避免常见情况下的分配。
	buf := make([]byte, 0, stackBufSize)

	n := len(p)

	// 不变量：
	//	读自路径：r 是下一个待处理字节的索引。
	//	写至缓冲：w 是写一个待写入字节的索引。

	// 路径必以 '/' 开头
	r := 1
	w := 1

	if p[0] != '/' {
		r = 0

		if n+1 > stackBufSize {
			buf = make([]byte, n+1)
		} else {
			buf = buf[:n+1]
		}
		buf[0] = '/'
	}

	trailing := n > 1 && p[n-1] == '/'

	// 没有 path 包那样的 'lazybuf' 该循环会有些笨拙，但它会完全内联（bufApp 调用）。
	// 因此，与 path 包相比，该循环无安规的函数调用（按需的 make 除外）
	for r < n {
		switch {
		case p[r] == '/':
			// 空路径，尾部有斜杠
			r++

		case p[r] == '.' && r+1 == n:
			trailing = true
			r++

		case p[r] == '.' && p[r+1] == '/':
			// . 元素
			r += 2

		case p[r] == '.' && p[r+1] == '.' && (r+2 == n || p[r+2] == '/'):
			// .. 元素：删至最后一个 /
			r += 3

			if w > 1 {
				// 可以回溯
				w--

				if len(buf) == 0 {
					for w > 1 && p[w] != '/' {
						w--
					}
				} else {
					for w > 1 && buf[w] != '/' {
						w--
					}
				}
			}

		default:
			// 真实路径元素。
			// 按需加斜杠。
			if w > 1 {
				bufApp(&buf, p, w, '/')
				w++
			}

			// Copy element
			for r < n && p[r] != '/' {
				bufApp(&buf, p, w, p[r])
				w++
				r++
			}
		}
	}

	// 重新追加尾部斜杠
	if trailing && w > 1 {
		bufApp(&buf, p, w, '/')
		w++
	}

	// 若原始字符串未被修改（或仅缩短末尾），则返回其相应子串。
	if len(buf) == 0 {
		return p[:w]
	}
	// 否则从缓冲区返回一个新字符串。
	return string(buf[:w])
}

// 按需创建缓冲区的惰性助手。
// 对该函数的调用是内联的。
func bufApp(buf *[]byte, s string, w int, c byte) {
	b := *buf
	if len(b) == 0 {
		// 暂未修改原字符串。
		// 若下个字符与原始字符串中的字符相同，暂无需分配缓冲区。
		if s[w] == c {
			return
		}

		// 否则，若堆栈缓冲区够大就用，或在堆上穿件一个新的缓冲区，并复制所有以前的字符。
		if l := len(s); l > cap(b) {
			*buf = make([]byte, len(s))
		} else {
			*buf = (*buf)[:l]
		}
		b = *buf

		copy(b, s[:w])
	}
	b[w] = c
}
