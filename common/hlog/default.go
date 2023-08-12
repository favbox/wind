package hlog

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

// Trace 调用默认记录器的 Trace 方法。
func Trace(v ...any) {
	logger.Trace(v...)
}

// Debug 调用默认记录器的 Debug 方法。
func Debug(v ...any) {
	logger.Debug(v...)
}

// Info 调用默认记录器的 Info 方法。
func Info(v ...any) {
	logger.Info(v...)
}

// Notice 调用默认记录器的 Notice 方法。
func Notice(v ...any) {
	logger.Notice(v...)
}

// Warn 调用默认记录器的 Warn 方法。
func Warn(v ...any) {
	logger.Warn(v...)
}

// Error 调用默认记录器的 Error 方法。
func Error(v ...any) {
	logger.Error(v...)
}

// Fatal 调用默认记录器的 Fatal 方法，然后 os.Exit(1)。
func Fatal(v ...any) {
	logger.Fatal(v...)
}

// Tracef 调用默认记录器的 Tracef 方法。
func Tracef(format string, v ...any) {
	logger.Tracef(format, v...)
}

// Debugf 调用默认记录器的 Debugf 方法。
func Debugf(format string, v ...any) {
	logger.Debugf(format, v...)
}

// Infof 调用默认记录器的 Infof 方法。
func Infof(format string, v ...any) {
	logger.Infof(format, v...)
}

// Noticef 调用默认记录器的 Noticef 方法。
func Noticef(format string, v ...any) {
	logger.Noticef(format, v...)
}

// Warnf 调用默认记录器的 Warnf 方法。
func Warnf(format string, v ...any) {
	logger.Warnf(format, v...)
}

// Errorf 调用默认记录器的 Errorf 方法。
func Errorf(format string, v ...any) {
	logger.Errorf(format, v...)
}

// Fatalf 调用默认记录器的 Fatalf 方法，然后 os.Exit(1)。
func Fatalf(format string, v ...any) {
	logger.Fatalf(format, v...)
}

// CtxTracef 调用默认记录器的 CtxTracef 方法。
func CtxTracef(ctx context.Context, format string, v ...any) {
	logger.CtxTracef(ctx, format, v...)
}

// CtxDebugf 调用默认记录器的 CtxDebugf 方法。
func CtxDebugf(ctx context.Context, format string, v ...any) {
	logger.CtxDebugf(ctx, format, v...)
}

// CtxInfof 调用默认记录器的 CtxInfof 方法。
func CtxInfof(ctx context.Context, format string, v ...any) {
	logger.CtxInfof(ctx, format, v...)
}

// CtxNoticef 调用默认记录器的 CtxNoticef 方法。
func CtxNoticef(ctx context.Context, format string, v ...any) {
	logger.CtxNoticef(ctx, format, v...)
}

// CtxWarnf 调用默认记录器的 CtxWarnf 方法。
func CtxWarnf(ctx context.Context, format string, v ...any) {
	logger.CtxWarnf(ctx, format, v...)
}

// CtxErrorf 调用默认记录器的 CtxErrorf 方法。
func CtxErrorf(ctx context.Context, format string, v ...any) {
	logger.CtxErrorf(ctx, format, v...)
}

// CtxFatalf 调用默认记录器的 CtxFatalf 方法，然后 os.Exit(1)。
func CtxFatalf(ctx context.Context, format string, v ...any) {
	logger.CtxFatalf(ctx, format, v...)
}

type defaultLogger struct {
	std   *log.Logger
	level Level
	depth int
}

func (l *defaultLogger) SetOutput(w io.Writer) {
	l.std.SetOutput(w)
}

func (l *defaultLogger) SetLevel(lv Level) {
	l.level = lv
}

func (l *defaultLogger) Trace(v ...any) {
	l.logf(LevelTrace, nil, v...)
}

func (l *defaultLogger) Debug(v ...any) {
	l.logf(LevelDebug, nil, v...)
}

func (l *defaultLogger) Info(v ...any) {
	l.logf(LevelInfo, nil, v...)
}

func (l *defaultLogger) Notice(v ...any) {
	l.logf(LevelNotice, nil, v...)
}

func (l *defaultLogger) Warn(v ...any) {
	l.logf(LevelWarn, nil, v...)
}

func (l *defaultLogger) Error(v ...any) {
	l.logf(LevelError, nil, v...)
}

func (l *defaultLogger) Fatal(v ...any) {
	l.logf(LevelFatal, nil, v...)
}

func (l *defaultLogger) Tracef(format string, v ...any) {
	l.logf(LevelTrace, &format, v...)
}

func (l *defaultLogger) Debugf(format string, v ...any) {
	l.logf(LevelDebug, &format, v...)
}

func (l *defaultLogger) Infof(format string, v ...any) {
	l.logf(LevelInfo, &format, v...)
}

func (l *defaultLogger) Noticef(format string, v ...any) {
	l.logf(LevelNotice, &format, v...)
}

func (l *defaultLogger) Warnf(format string, v ...any) {
	l.logf(LevelWarn, &format, v...)
}

func (l *defaultLogger) Errorf(format string, v ...any) {
	l.logf(LevelError, &format, v...)
}

func (l *defaultLogger) Fatalf(format string, v ...any) {
	l.logf(LevelFatal, &format, v...)
}

func (l *defaultLogger) CtxTracef(ctx context.Context, format string, v ...any) {
	l.logf(LevelTrace, &format, v...)
}

func (l *defaultLogger) CtxDebugf(ctx context.Context, format string, v ...any) {
	l.logf(LevelDebug, &format, v...)
}

func (l *defaultLogger) CtxInfof(ctx context.Context, format string, v ...any) {
	l.logf(LevelInfo, &format, v...)
}

func (l *defaultLogger) CtxNoticef(ctx context.Context, format string, v ...any) {
	l.logf(LevelNotice, &format, v...)
}

func (l *defaultLogger) CtxWarnf(ctx context.Context, format string, v ...any) {
	l.logf(LevelWarn, &format, v...)
}

func (l *defaultLogger) CtxErrorf(ctx context.Context, format string, v ...any) {
	l.logf(LevelError, &format, v...)
}

func (l *defaultLogger) CtxFatalf(ctx context.Context, format string, v ...any) {
	l.logf(LevelFatal, &format, v...)
}

func (l *defaultLogger) logf(lv Level, format *string, v ...any) {
	// 低于设置的日志级别，将不会输出。
	if l.level > lv {
		return
	}
	msg := lv.String()
	if format != nil {
		msg += fmt.Sprintf(*format, v...)
	} else {
		msg += fmt.Sprint(v...)
	}
	_ = l.std.Output(l.depth, msg)
	if lv == LevelFatal {
		os.Exit(1)
	}
}
