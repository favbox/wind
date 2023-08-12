package hlog

import (
	"context"
	"fmt"
	"io"
)

// Logger 是一个记录器接口，提供分级记录的功能。
type Logger interface {
	Trace(v ...any)
	Debug(v ...any)
	Info(v ...any)
	Notice(v ...any)
	Warn(v ...any)
	Error(v ...any)
	Fatal(v ...any)
}

// FormatLogger 是一个记录器接口，提供按格式分级记录的功能。
type FormatLogger interface {
	Tracef(format string, v ...any)
	Debugf(format string, v ...any)
	Infof(format string, v ...any)
	Noticef(format string, v ...any)
	Warnf(format string, v ...any)
	Errorf(format string, v ...any)
	Fatalf(format string, v ...any)
}

// CtxLogger 是一个记录器接口，提供按上下文+按格式进行分级记录的功能。
type CtxLogger interface {
	CtxTracef(ctx context.Context, format string, v ...any)
	CtxDebugf(ctx context.Context, format string, v ...any)
	CtxInfof(ctx context.Context, format string, v ...any)
	CtxNoticef(ctx context.Context, format string, v ...any)
	CtxWarnf(ctx context.Context, format string, v ...any)
	CtxErrorf(ctx context.Context, format string, v ...any)
	CtxFatalf(ctx context.Context, format string, v ...any)
}

// Control 提供配置记录器的方法。
type Control interface {
	// SetLevel 小于等于该级别不输出。
	SetLevel(Level)
	// SetOutput 设置日志输出器。
	SetOutput(io.Writer)
}

// FullLogger 是 Logger、FormatLogger、CtxLogger 和 Control 的组合。
type FullLogger interface {
	Logger
	FormatLogger
	CtxLogger
	Control
}

// Level 定义日志消息的优先级。
type Level int

// 日志记录级别。
const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelNotice
	LevelWarn
	LevelError
	LevelFatal
)

var strLevels = []string{
	"[Trace] ",
	"[Debug] ",
	"[Info] ",
	"[Notice] ",
	"[Warn] ",
	"[Error] ",
	"[Fatal] ",
}

func (lv Level) String() string {
	if lv >= LevelTrace && lv <= LevelFatal {
		return strLevels[lv]
	}
	return fmt.Sprintf("[?%d] ", lv)
}
