package discovery

import (
	"context"
	"testing"

	"github.com/favbox/wind/app/server/registry"
	"github.com/stretchr/testify/assert"
)

func TestInstance(t *testing.T) {
	network := "192.168.1.1"
	addresss := "/hello"
	weight := 1
	ins := NewInstance(network, addresss, weight, nil)

	assert.Equal(t, network, ins.Address().Network())
	assert.Equal(t, addresss, ins.Address().String())
	v, ok := ins.Tag("name")
	assert.Equal(t, "", v)
	assert.False(t, ok)

	ins = NewInstance("", "", 0, nil)
	assert.Equal(t, registry.DefaultWeight, ins.Weight())
}

func TestSynthesizedResolver(t *testing.T) {
	targetFunc := func(ctx context.Context, target *TargetInfo) string {
		return "userService"
	}
	resolveFunc := func(ctx context.Context, key string) (Result, error) {
		return Result{CacheKey: "name"}, nil
	}
	nmeFunc := func() string {
		return "mike"
	}
	resolver := SynthesizedResolver{
		TargetFunc:  targetFunc,
		ResolveFunc: resolveFunc,
		NameFunc:    nmeFunc,
	}

	assert.Equal(t, "userService", resolver.Target(context.Background(), &TargetInfo{}))
	result, err := resolver.Resolve(context.Background(), "")
	assert.Nil(t, err)
	assert.Equal(t, "name", result.CacheKey)

	resolver = SynthesizedResolver{
		TargetFunc:  nil,
		ResolveFunc: nil,
		NameFunc:    nil,
	}
	assert.Equal(t, "", resolver.Target(context.Background(), &TargetInfo{}))
	assert.Equal(t, "", resolver.Name())
}
