package stats

import (
	"sync"
	"sync/atomic"

	"github.com/favbox/gosky/wind/pkg/common/errors"
)

// EventIndex 表示一个唯一的事件。
type EventIndex int

// Level 设置记录级别。
type Level int

// Event 级别。
const (
	LevelDisabled Level = iota
	LevelBase
	LevelDetailed
)

// Event 用于表示一个特定事件。
type Event interface {
	Index() EventIndex
	Level() Level
}

type event struct {
	idx   EventIndex
	level Level
}

// Index 实现 Event 接口。
func (e event) Index() EventIndex {
	return e.idx
}

// Level 实现 Event 接口。
func (e event) Level() Level {
	return e.level
}

const (
	_ EventIndex = iota
	serverHandleStart
	serverHandleFinish
	httpStart
	httpFinish
	readHeaderStart
	readHeaderFinish
	readBodyStart
	readBodyFinish
	writeStart
	writeFinish
	predefinedEventNum
)

// 预定义的事件。
var (
	HTTPStart = newEvent(httpStart, LevelBase)

	ReadHeaderStart    = newEvent(readHeaderStart, LevelDetailed)    // 标头读取开始
	ReadHeaderFinish   = newEvent(readHeaderFinish, LevelDetailed)   // 标头读取结束
	ReadBodyStart      = newEvent(readBodyStart, LevelDetailed)      // 正文读取开始
	ReadBodyFinish     = newEvent(readBodyFinish, LevelDetailed)     // 正文读取结束
	ServerHandleStart  = newEvent(serverHandleStart, LevelDetailed)  // 处理开始
	ServerHandleFinish = newEvent(serverHandleFinish, LevelDetailed) // 处理结束
	WriteStart         = newEvent(writeStart, LevelDetailed)         // 写入开始
	WriteFinish        = newEvent(writeFinish, LevelDetailed)        // 写入结束

	HTTPFinish = newEvent(httpFinish, LevelBase)
)

// 错误
var (
	ErrNotAllowed = errors.NewPublic("初始化以后不允许再定义事件")
	ErrDuplicate  = errors.NewPublic("事件名称已被定义")
)

var (
	lock        sync.RWMutex
	initialized int32
	userDefined = make(map[string]Event)
	maxEventNum = int(predefinedEventNum)
)

// FinishInitialization 冻结所有定义的事件，并阻止进一步的定义。
func FinishInitialization() {
	atomic.StoreInt32(&initialized, 1)
}

// DefinedNewEvent 允许用户在程序初始化期间自定义事件。
func DefinedNewEvent(name string, level Level) (Event, error) {
	if atomic.LoadInt32(&initialized) == 1 {
		return nil, ErrNotAllowed
	}
	lock.Lock()
	defer lock.Unlock()
	evt, exist := userDefined[name]
	if exist {
		return evt, ErrDuplicate
	}
	userDefined[name] = newEvent(EventIndex(maxEventNum), level)
	maxEventNum++
	return userDefined[name], nil
}

// MaxEventNum 返回定义的事件数量。
func MaxEventNum() int {
	lock.RLock()
	defer lock.RUnlock()
	return maxEventNum
}

func newEvent(idx EventIndex, level Level) Event {
	return event{
		idx:   idx,
		level: level,
	}
}
