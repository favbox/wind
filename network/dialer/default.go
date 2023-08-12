//go:build !windows

package dialer

import "github.com/favbox/wind/network/netpoll"

func init() {
	// mac+linux 默认全局拨号器为 netpoll.dialer
	defaultDialer = netpoll.NewDialer()
}
