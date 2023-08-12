package proxy

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"time"

	"github.com/favbox/gosky/wind/internal/bytesconv"
	"github.com/favbox/gosky/wind/internal/bytestr"
	"github.com/favbox/gosky/wind/pkg/common/errors"
	"github.com/favbox/gosky/wind/pkg/network"
	"github.com/favbox/gosky/wind/pkg/protocol"
	"github.com/favbox/gosky/wind/pkg/protocol/consts"
	reqI "github.com/favbox/gosky/wind/pkg/protocol/http1/req"
	respI "github.com/favbox/gosky/wind/pkg/protocol/http1/resp"
)

// SetProxyAuthHeader 基于 proxyURI 为 h 设置代理授权标头。
func SetProxyAuthHeader(h *protocol.RequestHeader, proxyURI *protocol.URI) {
	if username := proxyURI.Username(); username != nil {
		password := proxyURI.Password()
		auth := base64.StdEncoding.EncodeToString(bytesconv.S2b(bytesconv.B2s(username) + ":" + bytesconv.B2s(password)))
		h.Set("Proxy-Authorization", "Basic "+auth)
	}
}

// SetupProxy 设置代理链接。
func SetupProxy(conn network.Conn, addr string, proxyURI *protocol.URI, tlsConfig *tls.Config, isTLS bool, dialer network.Dialer) (network.Conn, error) {
	var err error
	if bytes.Equal(proxyURI.Scheme(), bytestr.StrHTTPS) {
		conn, err = dialer.AddTLS(conn, tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	switch {
	case proxyURI == nil:
		// 啥也不干。没用代理。
	case isTLS: // 目标地址是 https， 则发一个 CONNECT 请求试试
		connectReq, connectResp := protocol.AcquireRequest(), protocol.AcquireResponse()
		defer func() {
			protocol.ReleaseRequest(connectReq)
			protocol.ReleaseResponse(connectResp)
		}()

		SetProxyAuthHeader(&connectReq.Header, proxyURI)
		connectReq.SetMethod(consts.MethodConnect)
		connectReq.SetHost(addr)

		// 发送 CONNECT 请求时，跳过响应体
		connectResp.SkipBody = true

		// 设置超时时长，以免永久阻塞造成协程泄露。
		connectCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		didReadResponse := make(chan struct{}) // 关闭于 CONNECT 请求读写完成或失败之后

		// 写入 CONNECT 请求，并读取响应。
		go func() {
			defer close(didReadResponse)

			err = reqI.Write(connectReq, conn)
			if err != nil {
				return
			}

			err = conn.Flush()
			if err != nil {
				return
			}

			err = respI.Read(connectResp, conn)
		}()
		select {
		case <-connectCtx.Done():
			conn.Close()
			<-didReadResponse

			return nil, connectCtx.Err()
		case <-didReadResponse:
		}

		if err != nil {
			conn.Close()
			return nil, err
		}

		if connectResp.StatusCode() != consts.StatusOK {
			conn.Close()

			return nil, errors.NewPublic(consts.StatusMessage(connectResp.StatusCode()))
		}
	}

	// 代理 + HTTPS，转为 TLS 连接
	if proxyURI != nil && isTLS {
		conn, err = dialer.AddTLS(conn, tlsConfig)
		if err != nil {
			return nil, err
		}
	}

	return conn, nil
}
