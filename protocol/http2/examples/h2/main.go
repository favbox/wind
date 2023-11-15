package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/favbox/wind/app"
	"github.com/favbox/wind/app/server"
	"github.com/favbox/wind/protocol/http2/config"
	"github.com/favbox/wind/protocol/http2/factory"
)

func main() {
	cfg := &tls.Config{
		MinVersion:       tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_AES_128_GCM_SHA256,
		},
	}
	cert, err := tls.LoadX509KeyPair("cert/server.crt", "cert/server.key")
	if err != nil {
		fmt.Println(err.Error())
	}
	cfg.Certificates = append(cfg.Certificates, cert)
	w := server.New(server.WithALPN(true), server.WithTLS(cfg))

	// 注册http2服务器工厂
	w.AddProtocol("h2", factory.NewServerFactory(
		config.WithReadTimeout(time.Minute),
		config.WithDisableKeepalive(false),
	))
	cfg.NextProtos = append(cfg.NextProtos, "h2")

	w.GET("/", func(_ context.Context, c *app.RequestContext) {
		c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	w.Spin()

}
