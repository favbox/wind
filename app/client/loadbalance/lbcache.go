package loadbalance

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/favbox/wind/app/client/discovery"
	"github.com/favbox/wind/common/errors"
	"github.com/favbox/wind/common/wlog"
	"github.com/favbox/wind/protocol"
	"golang.org/x/sync/singleflight"
)

var (
	balancerFactories    sync.Map // key: resolver name + load-balancer name
	balancerFactoriesSfg singleflight.Group
)

type cacheResult struct {
	res         atomic.Value // newest and previous discovery result
	expire      int32        // 0 = normal, 1 = expire and collect next ticker
	serviceName string       // service psm
}

func cacheKey(resolver, balancer string, opts Options) string {
	return fmt.Sprintf("%s|%s|{%s %s}", resolver, balancer, opts.RefreshInterval, opts.ExpireInterval)
}

type BalancerFactory struct {
	opts     Options
	cache    sync.Map // key -> LoadBalancer
	resolver discovery.Resolver
	balancer LoadBalancer
	sfg      singleflight.Group
}

type Config struct {
	Resolver discovery.Resolver
	Balancer LoadBalancer
	LbOpts   Options
}

// NewBalancerFactory 获取或创建一个给定目标的负载均衡器。
// 相同缓存键（resolver.Target(target)）的均衡器将被缓存或重用。
func NewBalancerFactory(config Config) *BalancerFactory {
	config.LbOpts.Check()
	uniqueKey := cacheKey(config.Resolver.Name(), config.Balancer.Name(), config.LbOpts)
	val, ok := balancerFactories.Load(uniqueKey)
	if ok {
		return val.(*BalancerFactory)
	}
	val, _, _ = balancerFactoriesSfg.Do(uniqueKey, func() (interface{}, error) {
		b := &BalancerFactory{
			opts:     config.LbOpts,
			resolver: config.Resolver,
			balancer: config.Balancer,
		}
		go b.watcher()
		go b.refresh()
		balancerFactories.Store(uniqueKey, b)
		return b, nil
	})
	return val.(*BalancerFactory)
}

// 监控过期均衡器
func (b *BalancerFactory) watcher() {
	for range time.Tick(b.opts.ExpireInterval) {
		b.cache.Range(func(key, value interface{}) bool {
			cache := value.(*cacheResult)
			if atomic.CompareAndSwapInt32(&cache.expire, 0, 1) {
				// 1. 设置过期标志
				// 2. 等待下一个计时器收集，可能负载均衡器被再次使用了
				// （避免立即删除最近创建的付贼均衡器）
			} else {
				b.cache.Delete(key)
				b.balancer.Delete(key.(string))
			}
			return true
		})
	}
}

// refresh 用于定期更新服务发现信息。
func (b *BalancerFactory) refresh() {
	for range time.Tick(b.opts.RefreshInterval) {
		b.cache.Range(func(key, value interface{}) bool {
			res, err := b.resolver.Resolve(context.Background(), key.(string))
			if err != nil {
				wlog.SystemLogger().Warnf("解析器刷新失败, 缓存建=%s 错误=%s", key, err.Error())
				return true
			}
			renameResultCacheKey(&res, b.resolver.Name())
			cache := value.(*cacheResult)
			cache.res.Store(res)
			atomic.StoreInt32(&cache.expire, 0)
			b.balancer.Rebalance(res)
			return true
		})
	}
}

// 使用解析器名称前缀的缓存键可避免均衡器冲突
func renameResultCacheKey(res *discovery.Result, resolverName string) {
	res.CacheKey = resolverName + ":" + res.CacheKey
}

// GetInstance 获取给定请求的服务实例。
func (b *BalancerFactory) GetInstance(ctx context.Context, req *protocol.Request) (discovery.Instance, error) {
	cacheRes, err := b.getCacheResult(ctx, req)
	if err != nil {
		return nil, err
	}
	atomic.StoreInt32(&cacheRes.expire, 0)
	ins := b.balancer.Pick(cacheRes.res.Load().(discovery.Result))
	if ins == nil {
		wlog.SystemLogger().Errorf("null instance. serviceName: %s, options: %v", string(req.Host()), req.Options())
		return nil, errors.NewPublic("instance not found")
	}
	return ins, nil
}

func (b *BalancerFactory) getCacheResult(ctx context.Context, req *protocol.Request) (*cacheResult, error) {
	target := b.resolver.Target(ctx, &discovery.TargetInfo{Host: string(req.Host()), Tags: req.Options().Tags()})
	cr, existed := b.cache.Load(target)
	if existed {
		return cr.(*cacheResult), nil
	}
	cr, err, _ := b.sfg.Do(target, func() (interface{}, error) {
		cache := &cacheResult{
			serviceName: string(req.Host()),
		}
		res, err := b.resolver.Resolve(ctx, target)
		if err != nil {
			return cache, err
		}
		renameResultCacheKey(&res, b.resolver.Name())
		cache.res.Store(res)
		atomic.StoreInt32(&cache.expire, 0)
		b.balancer.Rebalance(res)
		b.cache.Store(target, cache)
		return cache, nil
	})
	if err != nil {
		return nil, err
	}
	return cr.(*cacheResult), nil
}
