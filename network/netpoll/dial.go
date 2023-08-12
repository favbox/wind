package netpoll

import (
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/cloudwego/netpoll"
	"github.com/favbox/wind/network"
)

var errTLSNotSupported = errors.New("netpoll 不支持 TLS")

type dialer struct {
	netpoll.Dialer
}

func (d dialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	if tlsConfig != nil {
		return nil, errTLSNotSupported
	}
	connection, err := d.Dialer.DialConnection(network, address, timeout)
	if err != nil {
		return nil, err
	}

	conn = newConn(connection)
	return
}

func (d dialer) DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error) {
	if tlsConfig != nil {
		return nil, errTLSNotSupported
	}
	conn, err = d.Dialer.DialTimeout(network, address, timeout)
	if err != nil {
		return nil, err
	}
	return
}

func (d dialer) AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	return nil, errTLSNotSupported
}

func NewDialer() network.Dialer {
	return dialer{Dialer: netpoll.NewDialer()}
}
