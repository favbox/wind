package suite

import "github.com/favbox/gosky/wind/pkg/protocol/client"

type ClientFactory interface {
	NewHostClient() (hc client.HostClient, err error)
}
