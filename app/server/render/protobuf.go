package render

import (
	"github.com/favbox/gosky/wind/pkg/protocol"
	"google.golang.org/protobuf/proto"
)

var protobufContentType = "application/x-protobuf"

// ProtoBuf 包含要渲染的 pb 数据。
type ProtoBuf struct {
	Data any
}

func (r ProtoBuf) Render(resp *protocol.Response) error {
	writeContentType(resp, protobufContentType)
	pbBytes, err := proto.Marshal(r.Data.(proto.Message))
	if err != nil {
		return err
	}

	resp.AppendBody(pbBytes)
	return nil
}

func (r ProtoBuf) WriteContentType(resp *protocol.Response) {
	writeContentType(resp, protobufContentType)
}
