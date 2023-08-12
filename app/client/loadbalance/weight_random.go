package loadbalance

import (
	"sync"

	"github.com/bytedance/gopkg/lang/fastrand"
	"github.com/favbox/wind/app/client/discovery"
	"github.com/favbox/wind/common/wlog"
	"golang.org/x/sync/singleflight"
)

// 负载均衡实例的权重信息
type weightInfo struct {
	instances []discovery.Instance
	entries   []int
	weightSum int
}

// 加权随机平衡器。
type weightedBalancer struct {
	cachedWeightInfo sync.Map
	sfg              singleflight.Group
}

func (wb *weightedBalancer) Pick(e discovery.Result) discovery.Instance {
	wi, ok := wb.cachedWeightInfo.Load(e.CacheKey)
	if !ok {
		wi, _, _ = wb.sfg.Do(e.CacheKey, func() (any, error) {
			return wb.calcWeightInfo(e), nil
		})
		wb.cachedWeightInfo.Store(e.CacheKey, wi)
	}

	w := wi.(*weightInfo)
	if w.weightSum <= 0 {
		return nil
	}

	weight := fastrand.Intn(w.weightSum)
	for i := 0; i < len(w.instances); i++ {
		weight -= w.entries[i]
		if weight < 0 {
			return w.instances[i]
		}
	}

	return nil
}

func (wb *weightedBalancer) Rebalance(e discovery.Result) {
	wb.cachedWeightInfo.Store(e.CacheKey, wb.calcWeightInfo(e))
}

func (wb *weightedBalancer) Delete(cacheKey string) {
	wb.cachedWeightInfo.Delete(cacheKey)
}

func (wb *weightedBalancer) Name() string {
	return "weight_random"
}

func (wb *weightedBalancer) calcWeightInfo(e discovery.Result) *weightInfo {
	w := &weightInfo{
		instances: make([]discovery.Instance, len(e.Instances)),
		entries:   make([]int, len(e.Instances)),
		weightSum: 0,
	}

	var cnt int
	for idx := range e.Instances {
		weight := e.Instances[idx].Weight()
		if weight > 0 {
			w.instances[cnt] = e.Instances[idx]
			w.entries[cnt] = weight
			w.weightSum += weight
			cnt++
		} else {
			wlog.SystemLogger().Warnf("实例地址=%s 上的权重=%d 无效", e.Instances[idx].Address(), weight)
		}
	}
	w.instances = w.instances[:cnt]

	return w
}

// NewWeightedBalancer 使用加权随机算法创建负载均衡器。
func NewWeightedBalancer() LoadBalancer {
	lb := &weightedBalancer{}
	return lb
}
