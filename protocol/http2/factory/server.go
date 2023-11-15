package factory

import (
	"github.com/favbox/wind/protocol"
	"github.com/favbox/wind/protocol/http2"
	"github.com/favbox/wind/protocol/http2/config"
	"github.com/favbox/wind/protocol/suite"
)

var _ suite.ServerFactory = (*serverFactory)(nil)

// 实现了创建普通服务器的工厂方法。
type serverFactory struct {
	option *config.Config
}

type tracer interface {
	IsTraceEnable() bool
}

// New 在 engine.Run() 期间被 Wind 调用。
func (s *serverFactory) New(core suite.Core) (server protocol.Server, err error) {
	if cc, ok := core.(tracer); ok {
		s.option.EnableTrace = cc.IsTraceEnable()
	}
	return &http2.Server{
		BaseEngine: http2.BaseEngine{
			Config: *s.option,
			Core:   core,
		},
	}, nil
}

func NewServerFactory(opts ...config.Option) suite.ServerFactory {
	option := config.NewConfig(opts...)
	return &serverFactory{
		option: option,
	}
}
