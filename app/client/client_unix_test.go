//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package client

import (
	"crypto/tls"
	"math/rand"
	"time"

	"github.com/favbox/wind/network"
	"github.com/favbox/wind/network/netpoll"
	"github.com/favbox/wind/network/standard"
)

func newMockDialerWithCustomFunc(network, address string, timeout time.Duration, customDialerFunc func(network, address string, timeout time.Duration, tlsConfig *tls.Config)) network.Dialer {
	dialer := standard.NewDialer()
	if rand.Intn(2) == 0 {
		dialer = netpoll.NewDialer()
	}
	return &mockDialer{
		Dialer:           dialer,
		customDialerFunc: customDialerFunc,
		network:          network,
		address:          address,
		timeout:          timeout,
	}
}
