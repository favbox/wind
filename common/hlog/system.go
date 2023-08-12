package hlog

import (
	"context"
	"io"
	"strings"
	"sync"
)

var silentMode = false

// SetSilentMode 设置系统日志的静默开关。
// 例如：当读取请求头错误时，如果开启静默模式，则不会输出系统日志。
func SetSilentMode(s bool) {
	silentMode = s
}

var builderPool = sync.Pool{New: func() any {
	return &strings.Builder{}
}}

type systemLogger struct {
	logger FullLogger
	prefix string // 日志前缀
}

func (l *systemLogger) SetOutput(w io.Writer) {
	l.logger.SetOutput(w)
}

func (l *systemLogger) SetLevel(lv Level) {
	l.logger.SetLevel(lv)
}

func (l *systemLogger) Trace(v ...any) {
	v = append([]any{l.prefix}, v...)
	l.logger.Trace(v...)
}

func (l *systemLogger) Debug(v ...any) {
	v = append([]any{l.prefix}, v...)
	l.logger.Debug(v...)
}

func (l *systemLogger) Info(v ...any) {
	v = append([]any{l.prefix}, v...)
	l.logger.Info(v...)
}

func (l *systemLogger) Notice(v ...any) {
	v = append([]any{l.prefix}, v...)
	l.logger.Notice(v...)
}

func (l *systemLogger) Warn(v ...any) {
	v = append([]any{l.prefix}, v...)
	l.logger.Warn(v...)
}

func (l *systemLogger) Error(v ...any) {
	v = append([]any{l.prefix}, v...)
	l.logger.Error(v...)
}

func (l *systemLogger) Fatal(v ...any) {
	v = append([]any{l.prefix}, v...)
	l.logger.Fatal(v...)
}

func (l *systemLogger) Tracef(format string, v ...any) {
	l.logger.Tracef(l.addPrefix(format), v...)
}

func (l *systemLogger) Debugf(format string, v ...any) {
	l.logger.Debugf(l.addPrefix(format), v...)
}

func (l *systemLogger) Infof(format string, v ...any) {
	l.logger.Infof(l.addPrefix(format), v...)
}

func (l *systemLogger) Noticef(format string, v ...any) {
	l.logger.Noticef(l.addPrefix(format), v...)
}

func (l *systemLogger) Warnf(format string, v ...any) {
	l.logger.Warnf(l.addPrefix(format), v...)
}

func (l *systemLogger) Errorf(format string, v ...any) {
	if silentMode && format == EngineErrorFormat {
		return
	}
	l.logger.Errorf(l.addPrefix(format), v...)
}

func (l *systemLogger) Fatalf(format string, v ...any) {
	l.logger.Fatalf(l.addPrefix(format), v...)
}

func (l *systemLogger) CtxTracef(ctx context.Context, format string, v ...any) {
	l.logger.CtxTracef(ctx, l.addPrefix(format), v...)
}

func (l *systemLogger) CtxDebugf(ctx context.Context, format string, v ...any) {
	l.logger.CtxDebugf(ctx, l.addPrefix(format), v...)
}

func (l *systemLogger) CtxInfof(ctx context.Context, format string, v ...any) {
	l.logger.CtxInfof(ctx, l.addPrefix(format), v...)
}

func (l *systemLogger) CtxNoticef(ctx context.Context, format string, v ...any) {
	l.logger.CtxNoticef(ctx, l.addPrefix(format), v...)
}

func (l *systemLogger) CtxWarnf(ctx context.Context, format string, v ...any) {
	l.logger.CtxWarnf(ctx, l.addPrefix(format), v...)
}

func (l *systemLogger) CtxErrorf(ctx context.Context, format string, v ...any) {
	l.logger.CtxErrorf(ctx, l.addPrefix(format), v...)
}

func (l *systemLogger) CtxFatalf(ctx context.Context, format string, v ...any) {
	l.logger.CtxFatalf(ctx, l.addPrefix(format), v...)
}

func (l *systemLogger) addPrefix(format string) string {
	builder := builderPool.Get().(*strings.Builder)
	defer func() {
		builder.Reset()
		builderPool.Put(builder)
	}()

	builder.Grow(len(l.prefix) + len(format))
	builder.WriteString(l.prefix)
	builder.WriteString(format)
	s := builder.String()

	return s
}
