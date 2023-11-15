package http2

// WriterScheduler 是 HTTP/2 写入调度器所需实现的接口。
// 方法不会被同时调用故不会产生并发问题。
type WriterScheduler interface {
	// OpenStream 在写入调度器中打开一个新的流。
	// 使用 streamID=0 或已打开的 StreamID 调用该方法是非法的 —— 可能导致程序恐慌。
	OpenStream(streamID uint32, options OpenStreamOptions)

	// CloseStream 关闭写调度器中的一个流。该流上排队的任何帧都应该被丢弃。
	// 在未打开的流上调用该方法是非法的，可能导致程序恐慌。
	CloseStream(streamID uint32)

	// AdjustStream 调整给定流的优先级。
	// 可在一个尚未打开或已关闭的流上调用。
	// 请注意，RFC 7540允许在任何状态的流上发送PRIORITY帧。详情请参考：
	// https://tools.ietf.org/html/rfc7540#section-5.1
	AdjustStream(streamID uint32, priority PriorityParam)
}

// OpenStreamOptions 指定 WriterScheduler.OpenStream 的额外选项。
type OpenStreamOptions struct {
	// 若流是客户端发起，PushID为零。否则，PusherID 代表推送新打开数据流的流名称。
	PusherID uint32
}
