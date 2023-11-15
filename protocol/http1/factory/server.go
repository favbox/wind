package factory

import (
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/http1"
	"github.com/favbox/wind/protocol/suite"
)

var _ suite.ServerFactory = (*serverFactory)(nil)

// 实现了创建普通服务器的工厂方法。
type serverFactory struct {
	option *http1.Option
}

// New 在 engine.Run() 期间被 Wind 调用。
func (s *serverFactory) New(core suite.Core) (server protocol.Server, err error) {
	srv := http1.NewServer()
	srv.Option = *s.option
	srv.Core = core
	return srv, nil
}

// NewServerFactory 返回基于 HTTP/1.1 选项的服务器工厂。
func NewServerFactory(option *http1.Option) suite.ServerFactory {
	return &serverFactory{
		option: option,
	}
}
