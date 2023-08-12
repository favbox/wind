package utils

import "net"

var _ net.Addr = (*NetAddr)(nil)

// NetAddr 实现 net.Addr 接口。
type NetAddr struct {
	network string
	address string
}

// NewNetAddr 创建给定网络和地址的 NetAddr 对象。
func NewNetAddr(network, address string) net.Addr {
	return &NetAddr{
		network: network,
		address: address,
	}
}

func (na *NetAddr) Network() string {
	return na.network
}

func (na *NetAddr) String() string {
	return na.address
}
