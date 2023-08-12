package dialer

import "github.com/favbox/wind/network/standard"

func init() {
	defaultDialer = standard.NewDialer()
}
