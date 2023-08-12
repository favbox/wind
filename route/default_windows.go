package route

import "github.com/favbox/wind/network/standard"

func init() {
	defaultTransporter = standard.NewTransporter
}
