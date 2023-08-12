package loadbalance

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/favbox/wind/app/client/discovery"
	"github.com/favbox/wind/protocol"
	"github.com/stretchr/testify/assert"
)

func TestBuilder(t *testing.T) {
	ins := discovery.NewInstance("tcp", "127.0.0.1:8888", 10, map[string]string{"a": "b"})
	r := &discovery.SynthesizedResolver{
		ResolveFunc: func(ctx context.Context, key string) (discovery.Result, error) {
			return discovery.Result{CacheKey: key, Instances: []discovery.Instance{ins}}, nil
		},
		TargetFunc: func(ctx context.Context, target *discovery.TargetInfo) string {
			return "mockRoute"
		},
		NameFunc: func() string { return t.Name() },
	}
	lb := mockLoadBalancer{
		rebalanceFunc: nil,
		deleteFunc:    nil,
		pickFunc: func(res discovery.Result) discovery.Instance {
			assert.True(t, res.CacheKey == t.Name()+":mockRoute", res.CacheKey)
			assert.True(t, len(res.Instances) == 1)
			assert.True(t, len(res.Instances) == 1)
			assert.True(t, res.Instances[0].Address().String() == "127.0.0.1:8888")
			return res.Instances[0]
		},
		nameFunc: func() string { return "Synthesized" },
	}
	NewBalancerFactory(Config{
		Balancer: lb,
		LbOpts:   DefaultLbOpts,
		Resolver: r,
	})
	b, ok := balancerFactories.Load(cacheKey(t.Name(), "Synthesized", DefaultLbOpts))
	assert.True(t, ok)
	assert.True(t, b != nil)
	req := &protocol.Request{}
	req.SetHost("wind.api.test")
	ins1, err := b.(*BalancerFactory).GetInstance(context.TODO(), req)
	assert.True(t, err == nil)
	assert.True(t, ins1.Address().String() == "127.0.0.1:8888")
	assert.True(t, ins1.Weight() == 10)
	value, exists := ins1.Tag("a")
	assert.True(t, value == "b")
	assert.True(t, exists == true)
}

func TestBalancerCache(t *testing.T) {
	count := 10
	insList := make([]discovery.Instance, 0, count)
	for i := 0; i < count; i++ {
		insList = append(insList, discovery.NewInstance("tcp", fmt.Sprint(i), 10, nil))
	}
	r := &discovery.SynthesizedResolver{
		TargetFunc: func(ctx context.Context, target *discovery.TargetInfo) string {
			return target.Host
		},
		ResolveFunc: func(ctx context.Context, key string) (discovery.Result, error) {
			return discovery.Result{CacheKey: "svc", Instances: insList}, nil
		},
		NameFunc: func() string { return t.Name() },
	}
	lb := NewWeightedBalancer()
	for i := 0; i < count; i++ {
		blf := NewBalancerFactory(Config{
			Balancer: lb,
			LbOpts:   Options{},
			Resolver: r,
		})
		req := &protocol.Request{}
		req.SetHost("svc")
		for a := 0; a < count; a++ {
			addr, err := blf.GetInstance(context.TODO(), req)
			assert.True(t, err == nil, err)
			t.Logf("count: %d addr: %s\n", i, addr.Address().String())
		}
	}
}

func TestBalancerRefresh(t *testing.T) {
	var ins atomic.Value
	ins.Store(discovery.NewInstance("tcp", "127.0.0.1:8888", 10, nil))
	r := &discovery.SynthesizedResolver{
		TargetFunc: func(ctx context.Context, target *discovery.TargetInfo) string {
			return target.Host
		},
		ResolveFunc: func(ctx context.Context, key string) (discovery.Result, error) {
			return discovery.Result{CacheKey: "svc1", Instances: []discovery.Instance{ins.Load().(discovery.Instance)}}, nil
		},
		NameFunc: func() string { return t.Name() },
	}
	blf := NewBalancerFactory(Config{
		Balancer: NewWeightedBalancer(),
		LbOpts:   DefaultLbOpts,
		Resolver: r,
	})
	req := &protocol.Request{}
	req.SetHost("svc1")
	addr, err := blf.GetInstance(context.Background(), req)
	assert.True(t, err == nil, err)
	assert.True(t, addr.Address().String() == "127.0.0.1:8888")
	ins.Store(discovery.NewInstance("tcp", "127.0.0.1:8889", 10, nil))
	addr, err = blf.GetInstance(context.Background(), req)
	assert.True(t, err == nil, err)
	assert.True(t, addr.Address().String() == "127.0.0.1:8888")
	time.Sleep(6 * time.Second)
	addr, err = blf.GetInstance(context.Background(), req)
	assert.True(t, err == nil, err)
	assert.True(t, addr.Address().String() == "127.0.0.1:8889")
}

func TestCacheKey(t *testing.T) {
	uniqueKey := cacheKey("hello", "world", Options{RefreshInterval: 15 * time.Second, ExpireInterval: 5 * time.Minute})
	assert.True(t, uniqueKey == "hello|world|{15s 5m0s}")
}

type mockLoadBalancer struct {
	rebalanceFunc func(ch discovery.Result)
	deleteFunc    func(key string)
	pickFunc      func(discovery.Result) discovery.Instance
	nameFunc      func() string
}

// Rebalance implements the LoadBalancer interface.
func (m mockLoadBalancer) Rebalance(ch discovery.Result) {
	if m.rebalanceFunc != nil {
		m.rebalanceFunc(ch)
	}
}

// Delete implements the LoadBalancer interface.
func (m mockLoadBalancer) Delete(ch string) {
	if m.deleteFunc != nil {
		m.deleteFunc(ch)
	}
}

// Name implements the LoadBalancer interface.
func (m mockLoadBalancer) Name() string {
	if m.nameFunc != nil {
		return m.nameFunc()
	}
	return ""
}

// Pick implements the LoadBalancer interface.
func (m mockLoadBalancer) Pick(d discovery.Result) discovery.Instance {
	if m.pickFunc != nil {
		return m.pickFunc(d)
	}
	return nil
}
