package render

import (
	"fmt"

	"github.com/favbox/gosky/wind/pkg/protocol"
)

var plainContentType = "text/plain; charset=utf-8"

// String 包含要渲染的字符串格式和数据。
type String struct {
	Format string
	Data   []any
}

// Render 渲染纯文本。
func (r String) Render(resp *protocol.Response) error {
	writeContentType(resp, plainContentType)
	output := r.Format
	if len(r.Data) > 0 {
		output = fmt.Sprintf(r.Format, r.Data...)
	}
	resp.AppendBodyString(output)
	return nil
}

// WriteContentType 写入纯文本内容类型。
func (r String) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, plainContentType)
}
