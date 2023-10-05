package binding

import (
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/route/param"
)

type Binder interface {
	Name() string
	Bind(*protocol.Request, any, param.Params) error
	BindAndValidate(*protocol.Request, param.Params) error
	BindPath(*protocol.Request, any, param.Params) error
	BindQuery(*protocol.Request, any) error
	BindHeader(*protocol.Request, any) error
	BindForm(*protocol.Request, any) error
	BindJSON(*protocol.Request, any) error
	BindProtobuf(*protocol.Request, any) error
}
