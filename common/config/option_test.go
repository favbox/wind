package config

import (
	"testing"
	"time"

	"github.com/favbox/wind/app/server/registry"
	"github.com/stretchr/testify/assert"
)

// TestDefaultOptions 使用默认值测试配置项
func TestDefaultOptions(t *testing.T) {
	options := NewOptions([]Option{})

	assert.Equal(t, defaultKeepAliveTimeout, options.KeepAliveTimeout)
	assert.Equal(t, defaultReadTimeout, options.ReadTimeout)
	assert.Equal(t, defaultReadTimeout, options.IdleTimeout)
	assert.Equal(t, time.Duration(0), options.WriteTimeout)
	assert.True(t, options.RedirectTrailingSlash)
	assert.True(t, options.RedirectTrailingSlash)
	assert.False(t, options.HandleMethodNotAllowed)
	assert.False(t, options.UseRawPath)
	assert.False(t, options.RemoveExtraSlash)
	assert.True(t, options.UnescapePathValues)
	assert.False(t, options.DisablePreParseMultipartForm)
	assert.Equal(t, defaultNetwork, options.Network)
	assert.Equal(t, defaultAddr, options.Addr)
	assert.Equal(t, defaultMaxRequestBodySize, options.MaxRequestBodySize)
	assert.False(t, options.GetOnly)
	assert.False(t, options.DisableKeepalive)
	assert.False(t, options.NoDefaultServerHeader)
	assert.Equal(t, defaultWaitExitTimeout, options.ExitWaitTimeout)
	assert.Nil(t, options.TLS)
	assert.Equal(t, defaultReadBufferSize, options.ReadBufferSize)
	assert.False(t, options.ALPN)
	assert.False(t, options.H2C)
	assert.Equal(t, []any{}, options.Tracers)
	assert.Equal(t, new(any), options.TraceLevel)
	assert.Equal(t, registry.NoopRegistry, options.Registry)
	assert.False(t, options.DisableHeaderNamesNormalizing)
	assert.Nil(t, options.BindConfig)
	assert.Nil(t, options.CustomBinder)
	assert.Nil(t, options.CustomValidator)
}

// TestApplyCustomOptions 初始化后使用自定义值测试配置项应用函数
func TestApplyCustomOptions(t *testing.T) {
	options := NewOptions([]Option{})
	options.Apply([]Option{
		{F: func(o *Options) {
			o.Network = "unix"
		}},
	})
	assert.Equal(t, "unix", options.Network)
}
