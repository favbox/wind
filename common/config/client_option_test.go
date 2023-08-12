package config

import (
	"testing"
	"time"

	"github.com/favbox/wind/protocol/consts"
	"github.com/stretchr/testify/assert"
)

// TestDefaultClientOptions 测试客户端选项的默认值
func TestDefaultClientOptions(t *testing.T) {
	options := NewClientOptions([]ClientOption{})

	assert.Equal(t, consts.DefaultDialTimeout, options.DialTimeout)
	assert.Equal(t, consts.DefaultMaxConnsPerHost, options.MaxConnsPerHost)
	assert.Equal(t, consts.DefaultMaxIdleConnDuration, options.MaxIdleConnDuration)
	assert.Equal(t, true, options.KeepAlive)
}

// TestCustomClientOptions 测试客户端选项的自定义值。
func TestCustomClientOptions(t *testing.T) {
	options := NewClientOptions([]ClientOption{})

	options.Apply([]ClientOption{
		{
			F: func(o *ClientOptions) {
				o.DialTimeout = 2 * time.Second
			},
		},
	})
	assert.Equal(t, 2*time.Second, options.DialTimeout)
}
