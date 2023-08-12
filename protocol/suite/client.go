package suite

import "github.com/favbox/wind/protocol/client"

type ClientFactory interface {
	NewHostClient() (hc client.HostClient, err error)
}
