package stats

// Status 用于指示 Event 的状态。
type Status int8

// 预定义的状态。
const (
	StatusInfo  Status = 1
	StatusWarn  Status = 2
	StatusError Status = 3
)
