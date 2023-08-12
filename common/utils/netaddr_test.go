package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetAddr(t *testing.T) {
	networkAddr := NewNetAddr("tcp", "127.0.0.1")

	assert.Equal(t, networkAddr.Network(), "tcp")
	assert.Equal(t, networkAddr.String(), "127.0.0.1")
}
