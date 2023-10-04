package server

import (
	"testing"
	"time"

	"github.com/favbox/wind/app/server/registry"
	"github.com/favbox/wind/common/config"
	"github.com/favbox/wind/common/tracer/stats"
	"github.com/favbox/wind/common/utils"
	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	info := &registry.Info{
		ServiceName: "wind.test.api",
		Addr:        utils.NewNetAddr("", ""),
		Weight:      10,
	}
	opt := config.NewOptions([]config.Option{
		WithReadTimeout(time.Second),
		WithWriteTimeout(time.Second),
		WithIdleTimeout(time.Second),
		WithKeepAliveTimeout(time.Second),
		WithRedirectTrailingSlash(false),
		WithRedirectFixedPath(true),
		WithHandleMethodNotAllowed(true),
		WithUseRawPath(true),
		WithRemoveExtraSlash(true),
		WithUnescapePathValues(false),
		WithDisablePreParseMultipartForm(true),
		WithStreamBody(false),
		WithHostPorts(":8888"),
		WithBasePath("/"),
		WithMaxRequestBodySize(2),
		WithDisablePrintRoute(true),
		WithNetwork("unix"),
		WithExitWaitTime(time.Second),
		WithMaxKeepBodySize(500),
		WithGetOnly(true),
		WithKeepAlive(false),
		WithTLS(nil),
		WithH2C(true),
		WithReadBufferSize(100),
		WithALPN(true),
		WithTraceLevel(stats.LevelDisabled),
		WithRegistry(nil, info),
		WithAutoReloadRender(true, 5*time.Second),
		WithDisableHeaderNamesNormalizing(true),
	})

	assert.Equal(t, opt.ReadTimeout, time.Second)
	assert.Equal(t, opt.WriteTimeout, time.Second)
	assert.Equal(t, opt.IdleTimeout, time.Second)
	assert.Equal(t, opt.KeepAliveTimeout, time.Second)
	assert.Equal(t, opt.RedirectTrailingSlash, false)
	assert.Equal(t, opt.RedirectFixedPath, true)
	assert.Equal(t, opt.HandleMethodNotAllowed, true)
	assert.Equal(t, opt.UseRawPath, true)
	assert.Equal(t, opt.RemoveExtraSlash, true)
	assert.Equal(t, opt.UnescapePathValues, false)
	assert.Equal(t, opt.DisablePreParseMultipartForm, true)
	assert.Equal(t, opt.StreamRequestBody, false)
	assert.Equal(t, opt.Addr, ":8888")
	assert.Equal(t, opt.BasePath, "/")
	assert.Equal(t, opt.MaxRequestBodySize, 2)
	assert.Equal(t, opt.DisablePrintRoute, true)
	assert.Equal(t, opt.Network, "unix")
	assert.Equal(t, opt.ExitWaitTimeout, time.Second)
	assert.Equal(t, opt.MaxKeepBodySize, 500)
	assert.Equal(t, opt.GetOnly, true)
	assert.Equal(t, opt.DisableKeepalive, true)
	assert.Equal(t, opt.H2C, true)
	assert.Equal(t, opt.ReadBufferSize, 100)
	assert.Equal(t, opt.ALPN, true)
	assert.Equal(t, opt.TraceLevel, stats.LevelDisabled)
	assert.Equal(t, opt.RegistryInfo, info)
	assert.Equal(t, opt.Registry, nil)
	assert.Equal(t, opt.AutoReloadRender, true)
	assert.Equal(t, opt.AutoReloadInterval, 5*time.Second)
	assert.True(t, opt.DisableHeaderNamesNormalizing)
}

func TestDefaultOptions(t *testing.T) {
	opt := config.NewOptions([]config.Option{})
	assert.Equal(t, opt.ReadTimeout, time.Minute*3)
	assert.Equal(t, opt.IdleTimeout, time.Minute*3)
	assert.Equal(t, opt.KeepAliveTimeout, time.Minute)
	assert.Equal(t, opt.RedirectTrailingSlash, true)
	assert.Equal(t, opt.RedirectFixedPath, false)
	assert.Equal(t, opt.HandleMethodNotAllowed, false)
	assert.Equal(t, opt.UseRawPath, false)
	assert.Equal(t, opt.RemoveExtraSlash, false)
	assert.Equal(t, opt.UnescapePathValues, true)
	assert.Equal(t, opt.DisablePreParseMultipartForm, false)
	assert.Equal(t, opt.StreamRequestBody, false)
	assert.Equal(t, opt.Addr, ":8888")
	assert.Equal(t, opt.BasePath, "/")
	assert.Equal(t, opt.MaxRequestBodySize, 4*1024*1024)
	assert.Equal(t, opt.GetOnly, false)
	assert.Equal(t, opt.DisableKeepalive, false)
	assert.Equal(t, opt.DisablePrintRoute, false)
	assert.Equal(t, opt.Network, "tcp")
	assert.Equal(t, opt.ExitWaitTimeout, time.Second*5)
	assert.Equal(t, opt.MaxKeepBodySize, 4*1024*1024)
	assert.Equal(t, opt.H2C, false)
	assert.Equal(t, opt.ReadBufferSize, 4096)
	assert.Equal(t, opt.ALPN, false)
	assert.Equal(t, opt.Registry, registry.NoopRegistry)
	assert.Equal(t, opt.AutoReloadRender, false)
	assert.Nil(t, opt.RegistryInfo)
	assert.Equal(t, opt.AutoReloadRender, false)
	assert.Equal(t, opt.AutoReloadInterval, time.Duration(0))
	assert.False(t, opt.DisableHeaderNamesNormalizing)
}
