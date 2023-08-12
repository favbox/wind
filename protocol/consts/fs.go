package consts

import "time"

const (
	// MaxSmallFileSize 文件大于这个尺寸就用 sendfile 进行发送
	MaxSmallFileSize = 2 * 4096

	// FSCompressedFileSuffix 是 FS 另存压缩文件时追加到原始文件名的后缀。
	// 详见 app.FS。
	FSCompressedFileSuffix    = ".wind.gz"
	FSMinCompressRatio        = 0.8
	FsMaxCompressibleFileSize = 8 * 1024 * 1024 // 最大可压缩文件字节数

	// FSHandlerCacheDuration FS 打开的不活跃文件处理器的默认缓存时长。
	FSHandlerCacheDuration = 10 * time.Second
)
