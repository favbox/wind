package stats

import (
	"github.com/favbox/wind/common/tracer/stats"
	"github.com/favbox/wind/common/tracer/traceinfo"
)

// Record 记录事件至 HTTPStats。
func Record(ti traceinfo.TraceInfo, event stats.Event, err error) {
	if ti == nil {
		return
	}
	if err != nil {
		ti.Stats().Record(event, stats.StatusError, err.Error())
	} else {
		ti.Stats().Record(event, stats.StatusInfo, "")
	}
}

// CalcEventCostUs 计算统计耗时，并以微秒为单位返回。
func CalcEventCostUs(start, end traceinfo.Event) uint64 {
	if start == nil || end == nil || start.IsNil() || end.IsNil() {
		return 0
	}
	return uint64(end.Time().Sub(start.Time()).Microseconds())
}
