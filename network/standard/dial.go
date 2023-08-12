package standard

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/favbox/wind/network"
)

type dialer struct{}

func (d *dialer) DialConnection(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn network.Conn, err error) {
	c, err := net.DialTimeout(network, address, timeout)
	if tlsConfig != nil {
		cTLS := tls.Client(c, tlsConfig)
		conn = newTLSConn(cTLS, defaultMallocSize)
		return
	}
	conn = newConn(c, defaultMallocSize)
	return
}

func (d *dialer) DialTimeout(network, address string, timeout time.Duration, tlsConfig *tls.Config) (conn net.Conn, err error) {
	conn, err = net.DialTimeout(network, address, timeout)
	return
}

func (d *dialer) AddTLS(conn network.Conn, tlsConfig *tls.Config) (network.Conn, error) {
	cTLS := tls.Client(conn, tlsConfig)
	err := cTLS.Handshake()
	if err != nil {
		return nil, err
	}
	conn = newTLSConn(cTLS, defaultMallocSize)
	return conn, nil
}

func NewDialer() network.Dialer {
	return &dialer{}
}
