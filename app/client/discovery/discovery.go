package discovery

import (
	"context"
	"net"

	"github.com/favbox/wind/app/server/registry"
	"github.com/favbox/wind/common/utils"
)

// TargetInfo 定义目标信息的结构。
type TargetInfo struct {
	Host string
	Tags map[string]string
}

// Result 包含服务发现的实例结果。
type Result struct {
	CacheKey  string     // 缓存键
	Instances []Instance // 缓存值
}

// Resolver 定义发现目标的解析器接口。
type Resolver interface {
	// Target 应返回可作为缓存键的描述。
	Target(ctx context.Context, info *TargetInfo) (desc string)

	// Resolve 返回给定目标描述的实例列表。
	Resolve(ctx context.Context, desc string) (Result, error)

	// Name 返回解析器的名称。
	Name() string
}

// SynthesizedResolver 合成解析器。用 resolve 函数合成解析器。
type SynthesizedResolver struct {
	TargetFunc  func(ctx context.Context, target *TargetInfo) string
	ResolveFunc func(ctx context.Context, key string) (Result, error)
	NameFunc    func() string
}

func (sr SynthesizedResolver) Target(ctx context.Context, target *TargetInfo) (desc string) {
	if sr.TargetFunc == nil {
		return ""
	}
	return sr.TargetFunc(ctx, target)
}

func (sr SynthesizedResolver) Resolve(ctx context.Context, desc string) (Result, error) {
	return sr.ResolveFunc(ctx, desc)
}

func (sr SynthesizedResolver) Name() string {
	if sr.NameFunc == nil {
		return ""
	}
	return sr.NameFunc()
}

// Instance 包含目标服务的实例信息。
type Instance interface {
	Address() net.Addr
	Weight() int
	Tag(key string) (value string, exist bool)
}

type instance struct {
	addr   net.Addr
	weight int
	tags   map[string]string
}

func (i *instance) Address() net.Addr {
	return i.addr
}

func (i *instance) Weight() int {
	if i.weight > 0 {
		return i.weight
	}
	return registry.DefaultWeight
}

func (i *instance) Tag(key string) (value string, exist bool) {
	value, exist = i.tags[key]
	return
}

// NewInstance 用给定网络、地址、权重和标签创建实例。
func NewInstance(network, address string, weight int, tags map[string]string) Instance {
	return &instance{
		addr:   utils.NewNetAddr(network, address),
		weight: weight,
		tags:   tags,
	}
}
