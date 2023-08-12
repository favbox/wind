package consts

import "math"

// AbortIndex 单个请求的最大处理器索引值。
//
// 超过该索引值，则终止后续的 app.HandlerFunc。
const AbortIndex int8 = math.MaxInt8 / 2
