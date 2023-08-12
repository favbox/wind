package render

import (
	"github.com/favbox/gosky/wind/pkg/protocol"
)

// Data 包含要渲染的二进制数据和自定义内容类型。
type Data struct {
	ContentType string
	Data        []byte
}

// Render 渲染字节切片和自定义内容类型。
func (r Data) Render(resp *protocol.Response) error {
	writeContentType(resp, r.ContentType)
	resp.AppendBody(r.Data)
	return nil
}

// WriteContentType 写入自定义内容类型。
func (r Data) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, r.ContentType)
}
