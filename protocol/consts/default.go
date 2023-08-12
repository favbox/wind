package consts

import "time"

const (
	// DefaultDialTimeout 用于拨号器建立 TCP 连接的超时时间。
	DefaultDialTimeout = 1 * time.Second

	// DefaultMaxConnsPerHost 客户端为每个主机建立连接的默认并发连接数。
	DefaultMaxConnsPerHost = 512

	// DefaultMaxIdleConnDuration 闲置长连接超过此时长后会被关闭。
	DefaultMaxIdleConnDuration = 10 * time.Second

	// DefaultMaxInMemoryFileSize 定义解析多部分表单使用的内存文件大小，若超此值，则写入磁盘。
	DefaultMaxInMemoryFileSize = 16 * 1024 * 1024

	// DefaultMaxRetryTimes 默认重试次数。
	DefaultMaxRetryTimes = 1
)
