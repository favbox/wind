package dialer

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/favbox/wind/network"
)

// network.Dialer 声明全局拨号器。
var defaultDialer network.Dialer

// SetDialer 设置全局默认拨号器。
// Deprecated: 此函数仅用于测试，请使用 WithDialer。
func SetDialer(dialer network.Dialer) {
	defaultDialer = dialer
}

// DefaultDialer 返回全局拨号器。
func DefaultDialer() network.Dialer {
	return defaultDialer
}

// DialConnection 用全局拨号器拨打对端，获取网络连接。
func DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (network.Conn, error) {
	return defaultDialer.DialConnection(network, address, timeout, tlsConfig)
}

// DialTimeout 用全局拨号器：拨打对端，获取 net.Conn 连接。
// 注意：仅为兼容，不建议使用。
func DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (net.Conn, error) {
	return defaultDialer.DialTimeout(network, address, timeout, tlsConfig)
}

// AddTLS 用全局拨号器：将普通网络连接转为安全连接。
func AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return defaultDialer.AddTLS(conn, tlsConfig)
}
