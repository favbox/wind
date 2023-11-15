package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	conf := NewConfig()
	assert.Equal(t, time.Duration(0), conf.ReadTimeout)
	assert.Equal(t, false, conf.DisableKeepalive)
	assert.Equal(t, uint32(0), conf.MaxConcurrentStreams)
	assert.Equal(t, uint32(0), conf.MaxReadFrameSize)
	assert.Equal(t, false, conf.PermitProhibitedCipherSuites)
	assert.Equal(t, 10*time.Second, conf.IdleTimeout)
	assert.Equal(t, int32(0), conf.MaxUploadBufferPerConnection)
	assert.Equal(t, int32(0), conf.MaxUploadBufferPerStream)

	conf = NewConfig(
		WithReadTimeout(1*time.Second),
		WithDisableKeepalive(true),
		WithMaxConcurrentStreams(2),
		WithMaxReadFrameSize(3),
		WithPermitProhibitedCipherSuites(true),
		WithIdleTimeout(4*time.Second),
		WithMaxUploadBufferPerConnection(5),
		WithMaxUploadBufferPerStream(6),
	)
	assert.Equal(t, time.Second, conf.ReadTimeout)
	assert.Equal(t, true, conf.DisableKeepalive)
	assert.Equal(t, uint32(2), conf.MaxConcurrentStreams)
	assert.Equal(t, uint32(3), conf.MaxReadFrameSize)
	assert.Equal(t, true, conf.PermitProhibitedCipherSuites)
	assert.Equal(t, 4*time.Second, conf.IdleTimeout) // has default value
	assert.Equal(t, int32(5), conf.MaxUploadBufferPerConnection)
	assert.Equal(t, int32(6), conf.MaxUploadBufferPerStream)
}
