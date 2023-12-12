//go:build !windows

package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetScheme(t *testing.T) {
	scheme, path := getScheme([]byte("https://foo.com"))
	assert.Equal(t, "https", string(scheme))
	assert.Equal(t, "//foo.com", string(path))

	scheme, path = getScheme([]byte(":"))
	assert.Equal(t, "", string(scheme))
	assert.Equal(t, "", string(path))

	scheme, path = getScheme([]byte("ws://127.0.0.1"))
	assert.Equal(t, "ws", string(scheme))
	assert.Equal(t, "//127.0.0.1", string(path))

	scheme, path = getScheme([]byte("/hertz/demo"))
	assert.Equal(t, "", string(scheme))
	assert.Equal(t, "/hertz/demo", string(path))
}
