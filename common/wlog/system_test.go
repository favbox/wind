package wlog

import (
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func initTestSysLogger() {
	sysLogger = &systemLogger{
		logger: &defaultLogger{
			std:   log.New(os.Stderr, "", 0),
			depth: 4,
		},
		prefix: systemLogPrefix,
	}
}

func TestSystemLogger(t *testing.T) {
	initTestSysLogger()
	var w byteSliceWriter
	SetOutput(&w)

	sysLogger.Trace("跟踪工作")
	sysLogger.Debug("收到工作清单")
	sysLogger.Info("开始工作")
	sysLogger.Notice("工作中出现一些状况")
	sysLogger.Warn("工作可能失败")
	sysLogger.Error("工作失败")

	assert.Equal(t, "[Trace] WIND: 跟踪工作\n"+
		"[Debug] WIND: 收到工作清单\n"+
		"[Info] WIND: 开始工作\n"+
		"[Notice] WIND: 工作中出现一些状况\n"+
		"[Warn] WIND: 工作可能失败\n"+
		"[Error] WIND: 工作失败\n", string(w.b))
}

func TestSystemFormatLogger(t *testing.T) {
	initTestSysLogger()

	var w byteSliceWriter
	SetOutput(&w)

	item := "工作"
	sysLogger.Tracef("跟踪%s", item)
	sysLogger.Debugf("收到%s清单", item)
	sysLogger.Infof("开始%s", item)
	sysLogger.Noticef("%s中出现一些状况", item)
	sysLogger.Warnf("%s可能失败", item)
	sysLogger.Errorf("%s失败", item)

	assert.Equal(t, "[Trace] WIND: 跟踪工作\n"+
		"[Debug] WIND: 收到工作清单\n"+
		"[Info] WIND: 开始工作\n"+
		"[Notice] WIND: 工作中出现一些状况\n"+
		"[Warn] WIND: 工作可能失败\n"+
		"[Error] WIND: 工作失败\n", string(w.b))
}
