package hlog

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func initTestLogger() {
	logger = &defaultLogger{
		std:   log.New(os.Stderr, "", 0),
		depth: 4,
	}
}

type byteSliceWriter struct {
	b []byte
}

func (w *byteSliceWriter) Write(p []byte) (int, error) {
	w.b = append(w.b, p...)
	return len(p), nil
}

func TestDefaultLogger(t *testing.T) {
	initTestLogger()

	var w byteSliceWriter
	SetOutput(&w)

	Trace("跟踪工作")
	Debug("收到工作清单")
	Info("开始工作")
	Notice("工作中出现一些状况")
	Warn("工作可能失败")
	Error("工作失败")

	assert.Equal(t, "[Trace] 跟踪工作\n"+
		"[Debug] 收到工作清单\n"+
		"[Info] 开始工作\n"+
		"[Notice] 工作中出现一些状况\n"+
		"[Warn] 工作可能失败\n"+
		"[Error] 工作失败\n", string(w.b))
}

func TestDefaultFormatLogger(t *testing.T) {
	initTestLogger()

	var w byteSliceWriter
	SetOutput(&w)

	item := "工作"
	Tracef("跟踪%s", item)
	Debugf("收到%s清单", item)
	Infof("开始%s", item)
	Noticef("%s中出现一些状况", item)
	Warnf("%s可能失败", item)
	Errorf("%s失败", item)

	assert.Equal(t, "[Trace] 跟踪工作\n"+
		"[Debug] 收到工作清单\n"+
		"[Info] 开始工作\n"+
		"[Notice] 工作中出现一些状况\n"+
		"[Warn] 工作可能失败\n"+
		"[Error] 工作失败\n", string(w.b))
}

func TestCtxLogger(t *testing.T) {
	initTestLogger()

	var w byteSliceWriter
	SetOutput(&w)

	ctx := context.Background()
	item := "工作"
	CtxTracef(ctx, "跟踪%s", item)
	CtxDebugf(ctx, "收到%s清单", item)
	CtxInfof(ctx, "开始%s", item)
	CtxNoticef(ctx, "%s中出现一些状况", item)
	CtxWarnf(ctx, "%s可能失败", item)
	CtxErrorf(ctx, "%s失败", item)

	assert.Equal(t, "[Trace] 跟踪工作\n"+
		"[Debug] 收到工作清单\n"+
		"[Info] 开始工作\n"+
		"[Notice] 工作中出现一些状况\n"+
		"[Warn] 工作可能失败\n"+
		"[Error] 工作失败\n", string(w.b))
}

func TestSetLevel(t *testing.T) {
	stdLogger := &defaultLogger{
		std:   log.New(os.Stderr, "", log.LstdFlags|log.Lshortfile|log.Lmicroseconds),
		depth: 4,
	}

	stdLogger.SetLevel(LevelTrace)
	assert.Equal(t, LevelTrace, stdLogger.level)
	assert.Equal(t, LevelTrace, stdLogger.level)

	stdLogger.SetLevel(LevelDebug)
	assert.Equal(t, LevelDebug, stdLogger.level)
	assert.Equal(t, LevelDebug, stdLogger.level)

	stdLogger.SetLevel(LevelInfo)
	assert.Equal(t, LevelInfo, stdLogger.level)
	assert.Equal(t, LevelInfo, stdLogger.level)

	stdLogger.SetLevel(LevelNotice)
	assert.Equal(t, LevelNotice, stdLogger.level)
	assert.Equal(t, LevelNotice, stdLogger.level)

	stdLogger.SetLevel(LevelWarn)
	assert.Equal(t, LevelWarn, stdLogger.level)
	assert.Equal(t, LevelWarn, stdLogger.level)

	stdLogger.SetLevel(LevelError)
	assert.Equal(t, LevelError, stdLogger.level)
	assert.Equal(t, LevelError, stdLogger.level)

	stdLogger.SetLevel(LevelFatal)
	assert.Equal(t, LevelFatal, stdLogger.level)
	assert.Equal(t, LevelFatal, stdLogger.level)

	stdLogger.SetLevel(7)
	assert.Equal(t, 7, int(stdLogger.level))
	assert.Equal(t, "[?7] ", stdLogger.level.String())
}
