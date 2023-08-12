package factory

import (
	"github.com/favbox/gosky/wind/pkg/protocol/client"
	"github.com/favbox/gosky/wind/pkg/protocol/http1"
	"github.com/favbox/gosky/wind/pkg/protocol/suite"
)

var _ suite.ClientFactory = (*clientFactory)(nil)

type clientFactory struct {
	option *http1.ClientOptions
}

func (c *clientFactory) NewHostClient() (hc client.HostClient, err error) {
	return http1.NewHostClient(c.option), nil
}

func NewClientFactory(option *http1.ClientOptions) suite.ClientFactory {
	return &clientFactory{
		option: option,
	}
}
