package render

import (
	"encoding/xml"

	"github.com/favbox/gosky/wind/pkg/protocol"
)

var xmlContentType = "application/xml; charset=utf-8"

// XML 包含要渲染的 XML 数据。
type XML struct {
	Data any
}

func (r XML) Render(resp *protocol.Response) error {
	writeContentType(resp, xmlContentType)
	xmlBytess, err := xml.Marshal(r.Data)
	if err != nil {
		return err
	}

	resp.AppendBody(xmlBytess)
	return nil
}

func (r XML) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, xmlContentType)
}
