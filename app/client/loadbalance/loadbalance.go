package loadbalance

import (
	"time"

	"github.com/favbox/wind/app/client/discovery"
)

const (
	DefaultRefreshInterval = 5 * time.Second
	DefaultExpireInterval  = 15 * time.Second
)

var DefaultLbOpts = Options{
	RefreshInterval: DefaultRefreshInterval,
	ExpireInterval:  DefaultExpireInterval,
}

// LoadBalancer 负载均衡器，按服务发现结果选择实例。
type LoadBalancer interface {
	// Pick 根据发现结果选择实例。
	Pick(result discovery.Result) discovery.Instance

	// Rebalance 用于刷新负载均衡的缓存信息。
	Rebalance(e discovery.Result)

	// Delete 删除过期负载均衡的缓存。
	Delete(cacheKey string)

	// Name 返回负载均衡器的名称。
	Name() string
}

// Options 负载均衡选项。
type Options struct {
	// 及时刷新服务发现结果
	RefreshInterval time.Duration

	// 均衡器的过期检查间隔。
	// 我们需要移除空闲的均衡器已节约资源。
	ExpireInterval time.Duration
}

// Check 检查默认参数。
func (o *Options) Check() {
	if o.RefreshInterval <= 0 {
		o.RefreshInterval = DefaultRefreshInterval
	}
	if o.ExpireInterval <= 0 {
		o.ExpireInterval = DefaultExpireInterval
	}
}
