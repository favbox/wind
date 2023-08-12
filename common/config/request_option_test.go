package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRequestOptions 使用自定义值测试请求选项。
func TestRequestOptions(t *testing.T) {
	opt := NewRequestOptions([]RequestOption{
		WithTag("a", "b"),
		WithTag("c", "d"),
		WithTag("e", "f"),
		WithSD(true),
		WithDialTimeout(time.Second),
		WithReadTimeout(time.Second),
		WithWriteTimeout(time.Second),
	})
	assert.Equal(t, "b", opt.Tag("a"))
	assert.Equal(t, "d", opt.Tag("c"))
	assert.Equal(t, "f", opt.Tag("e"))
	assert.Equal(t, time.Second, opt.DialTimeout())
	assert.Equal(t, time.Second, opt.ReadTimeout())
	assert.Equal(t, time.Second, opt.WriteTimeout())
	assert.True(t, opt.IsSD())
}

// TestRequestOptionsWithDefaultOpts 使用默认值测试请求选项。
func TestRequestOptionsWithDefaultOpts(t *testing.T) {
	SetPreDefinedOpts(WithTag("pre-defined", "blablabla"), WithTag("a", "default-value"), WithSD(true))
	opt := NewRequestOptions([]RequestOption{
		WithTag("a", "b"),
		WithSD(false),
	})
	assert.Equal(t, "b", opt.Tag("a"))
	assert.Equal(t, "blablabla", opt.Tag("pre-defined"))
	assert.Equal(t, map[string]string{
		"a":           "b",
		"pre-defined": "blablabla",
	}, opt.Tags())
	assert.False(t, opt.IsSD())
	SetPreDefinedOpts()
	assert.Nil(t, preDefinedOpts)
	assert.Equal(t, time.Duration(0), opt.WriteTimeout())
	assert.Equal(t, time.Duration(0), opt.ReadTimeout())
	assert.Equal(t, time.Duration(0), opt.DialTimeout())
}

// TestRequestOptions_CopyTo 测试拷贝请求选项。
func TestRequestOptions_CopyTo(t *testing.T) {
	opt := NewRequestOptions([]RequestOption{
		WithTag("a", "b"),
		WithSD(false),
	})
	var copyOpt RequestOptions
	opt.CopyTo(&copyOpt)
	assert.Equal(t, opt.Tags(), copyOpt.Tags())
	assert.Equal(t, opt.IsSD(), copyOpt.IsSD())
}
