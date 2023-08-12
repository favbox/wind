package registry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoopRegistry(t *testing.T) {
	reg := noopRegistry{}
	assert.Nil(t, reg.Register(&Info{}))
	assert.Nil(t, reg.Deregister(&Info{}))
}
