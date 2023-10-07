package binding

import (
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

// Binder 表示一个请求参数的绑定器接口。
type Binder interface {
	Name() string
	BindAndValidate(*protocol.Request, any, param.Params) error
	Bind(*protocol.Request, any, param.Params) error
	BindPath(*protocol.Request, any, param.Params) error
	BindQuery(*protocol.Request, any) error
	BindHeader(*protocol.Request, any) error
	BindForm(*protocol.Request, any) error
	BindJSON(*protocol.Request, any) error
	BindProtobuf(*protocol.Request, any) error
}
