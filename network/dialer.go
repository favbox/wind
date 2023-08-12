package network

import (
	"crypto/tls"
	"net"
	"time"
)

// Dialer 定义连接拨号器接口。
type Dialer interface {
	// DialConnection 拨打对端，获取 Conn 连接。
	DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn Conn, err error)

	// DialTimeout 拨打对端，获取 net.Conn 连接。
	//
	// 注意：仅为兼容，不建议使用。
	DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error)

	// AddTLS 将普通网络连接转为安全连接。
	AddTLS(conn Conn, tlsConfig *tls.Config) (Conn, error)
}
