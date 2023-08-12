package render

import "github.com/favbox/gosky/wind/pkg/protocol"

// Render 渲染接口将通过 JSON, HTML, XML 等实现。
type Render interface {
	// Render 写入数据和 ContentType。
	// 不要在该方法内 panic，RequestContext 会处理。
	Render(resp *protocol.Response) error
	// WriteContentType 写入自定义的 ContentType。
	WriteContentType(resp *protocol.Response)
}

var (
	_ Render = Data{}
	_ Render = String{}
	_ Render = JSONRender{}
)

// 设置响应的内容类型。
func writeContentType(resp *protocol.Response, value string) {
	resp.Header.SetContentType(value)
}
