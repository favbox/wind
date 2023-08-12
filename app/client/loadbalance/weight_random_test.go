package loadbalance

import (
	"math"
	"testing"

	"github.com/favbox/wind/app/client/discovery"
	"github.com/stretchr/testify/assert"
)

func TestNewWeightedBalancer(t *testing.T) {
	balancer := NewWeightedBalancer()

	// nil 实例
	ins := balancer.Pick(discovery.Result{})
	assert.Equal(t, nil, ins)

	// 空实例
	e := discovery.Result{
		CacheKey:  "a",
		Instances: make([]discovery.Instance, 0),
	}
	balancer.Rebalance(e)
	ins = balancer.Pick(e)
	assert.Equal(t, nil, ins)

	// 单实例
	insList := []discovery.Instance{
		discovery.NewInstance("tcp", "127.0.0.1:8888", 20, nil),
	}
	e = discovery.Result{
		CacheKey:  "b",
		Instances: insList,
	}
	balancer.Rebalance(e)
	for i := 0; i < 100; i++ {
		ins = balancer.Pick(e)
		assert.Equal(t, ins.Weight(), 20)
	}

	// 多实例，weightSum > 0
	insList = []discovery.Instance{
		discovery.NewInstance("tcp", "127.0.0.1:8881", 10, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8882", 20, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8883", 50, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8884", 100, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8885", 200, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8886", 500, nil),
	}

	var weightSum int
	for _, ins := range insList {
		weight := ins.Weight()
		weightSum += weight
	}

	n := 10000000
	pickedStat := map[int]int{}
	e = discovery.Result{
		CacheKey:  "c",
		Instances: insList,
	}
	balancer.Rebalance(e)
	for i := 0; i < n; i++ {
		ins = balancer.Pick(e)
		weight := ins.Weight()
		if pickedCnt, ok := pickedStat[weight]; ok {
			pickedStat[weight] = pickedCnt + 1
		} else {
			pickedStat[weight] = 1
		}
	}

	for _, ins := range insList {
		weight := ins.Weight()
		expect := float64(weight) / float64(weightSum) * float64(n)
		actual := float64(pickedStat[weight])
		delta := math.Abs(expect - actual)
		assert.Equal(t, true, delta/expect < 0.01)
	}

	// have instances that weight < 0
	insList = []discovery.Instance{
		discovery.NewInstance("tcp", "127.0.0.1:8881", 10, nil),
		discovery.NewInstance("tcp", "127.0.0.1:8882", -10, nil),
	}
	e = discovery.Result{
		Instances: insList,
		CacheKey:  "d",
	}
	balancer.Rebalance(e)
	for i := 0; i < 1000; i++ {
		ins = balancer.Pick(e)
		assert.Equal(t, 10, ins.Weight())
	}
}
