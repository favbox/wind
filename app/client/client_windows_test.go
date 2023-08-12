//go:build windows

package client

import (
	"crypto/tls"
	"time"

	"github.com/favbox/wind/network"
	"github.com/favbox/wind/network/standard"
)

func newMockDialerWithCustomFunc(network, address string, timeout time.Duration, f func(network, address string, timeout time.Duration, tlsConfig *tls.Config)) network.Dialer {
	dialer := standard.NewDialer()
	return &mockDialer{
		Dialer:           dialer,
		customDialerFunc: f,
		network:          network,
		address:          address,
		timeout:          timeout,
	}
}
