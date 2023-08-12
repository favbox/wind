package stats

import (
	"testing"
	"time"

	"github.com/favbox/wind/common/tracer/stats"
	"github.com/favbox/wind/common/tracer/traceinfo"
	"github.com/stretchr/testify/assert"
)

func TestUtil(t *testing.T) {
	assert.True(t, CalcEventCostUs(nil, nil) == 0)

	ti := traceinfo.NewTraceInfo()

	// nil context
	Record(ti, stats.HTTPStart, nil)
	Record(ti, stats.HTTPFinish, nil)

	st := ti.Stats()
	assert.NotNil(t, st)

	s, e := st.GetEvent(stats.HTTPStart), st.GetEvent(stats.HTTPFinish)
	assert.Nil(t, s)
	assert.Nil(t, e)

	// stats disabled
	Record(ti, stats.HTTPStart, nil)
	time.Sleep(time.Millisecond)
	Record(ti, stats.HTTPFinish, nil)

	st = ti.Stats()
	assert.NotNil(t, st)

	s, e = st.GetEvent(stats.HTTPStart), st.GetEvent(stats.HTTPFinish)
	assert.True(t, s == nil)
	assert.True(t, e == nil)

	// stats enabled
	st = ti.Stats()
	st.(interface{ SetLevel(stats.Level) }).SetLevel(stats.LevelBase)

	Record(ti, stats.HTTPStart, nil)
	time.Sleep(time.Millisecond)
	Record(ti, stats.HTTPFinish, nil)

	s, e = st.GetEvent(stats.HTTPStart), st.GetEvent(stats.HTTPFinish)
	assert.True(t, s != nil, s)
	assert.True(t, e != nil, e)
	assert.True(t, CalcEventCostUs(s, e) > 0)
}
