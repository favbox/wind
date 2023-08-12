package utils

import "sync"

// CopyBufPool 拷贝缓冲池。默认长度为4KB。
var CopyBufPool = sync.Pool{
	New: func() any {
		return make([]byte, 4096)
	},
}
