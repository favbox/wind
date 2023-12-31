//go:build stdjson || !(amd64 && (linux || windows || darwin))

package json

import "encoding/json"

// Name 是有效的JSON 包名。
const Name = "encoding/json"

var (
	// Marshal 用于 JSON 编码而导出的标准库实现。
	Marshal = json.Marshal
	// Unmarshal 用于 JSON 解码而导出的标准库实现。
	Unmarshal = json.Unmarshal
	// MarshalIndent 用于编码为带缩进格式的 JSON 而导出的标准库实现。
	MarshalIndent = json.MarshalIndent
	// NewDecoder 用于读取 io.Reader 而导出的 JSON 读取器。
	NewDecoder = json.NewDecoder
	// NewEncoder 用于写入 io.Writer 而导出的 JSON 编码器。
	NewEncoder = json.NewEncoder
)
