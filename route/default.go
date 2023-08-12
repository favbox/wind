//go:build !windows

package route

import "github.com/favbox/wind/network/netpoll"

func init() {
	defaultTransporter = netpoll.NewTransporter
}
