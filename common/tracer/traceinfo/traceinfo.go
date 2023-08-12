package traceinfo

// TraceInfo 包含 Wind 中的跟踪信息。
type TraceInfo interface {
	// Stats 获取 HTTP 统计信息。
	Stats() HTTPStats
	// Reset 重置 HTTP 统计信息。
	Reset()
}

type traceInfo struct {
	stats HTTPStats
}

func (t *traceInfo) Stats() HTTPStats {
	return t.stats
}

func (t *traceInfo) Reset() {
	t.stats.Reset()
}

// NewTraceInfo 创建一个跟踪信息的实例。
func NewTraceInfo() TraceInfo {
	return &traceInfo{stats: NewHTTPStats()}
}
