package stats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefinedNewEvent(t *testing.T) {
	num0 := MaxEventNum()

	event1, err1 := DefinedNewEvent("myevent", LevelDetailed)
	num1 := MaxEventNum()

	assert.Nil(t, err1)
	assert.NotNil(t, event1)
	assert.Equal(t, num1, num0+1)
	assert.Equal(t, LevelDetailed, event1.Level())

	event2, err2 := DefinedNewEvent("myevent", LevelBase)
	num2 := MaxEventNum()
	assert.Equal(t, ErrDuplicate, err2)
	assert.Equal(t, event1, event2)
	assert.Equal(t, num1, num2)

	FinishInitialization()

	event3, err3 := DefinedNewEvent("another", LevelDetailed)
	num3 := MaxEventNum()
	assert.Equal(t, ErrNotAllowed, err3)
	assert.Nil(t, event3)
	assert.Equal(t, num1, num3)
}
