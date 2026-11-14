package gormlogger

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Logger adapts GORM logs to Kitex klog.
type Logger struct {
	config logger.Config
}

// New creates a GORM logger that prints SQL statements at Info level.
func New() *Logger {
	return NewWithConfig(logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		IgnoreRecordNotFoundError: true,
		LogLevel:                  logger.Info,
	})
}

// NewWithConfig creates a GORM logger with the provided configuration.
func NewWithConfig(config logger.Config) *Logger {
	return &Logger{config: config}
}

func (l *Logger) LogMode(level logger.LogLevel) logger.Interface {
	configured := *l
	configured.config.LogLevel = level
	return &configured
}

func (l *Logger) Info(_ context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Info {
		klog.Infof(msg, data...)
	}
}

func (l *Logger) Warn(_ context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Warn {
		klog.Warnf(msg, data...)
	}
}

func (l *Logger) Error(_ context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Error {
		klog.Errorf(msg, data...)
	}
}

func (l *Logger) Trace(_ context.Context, begin time.Time, query func() (string, int64), err error) {
	if l.config.LogLevel == logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	caller := callSite()
	switch {
	case err != nil && l.config.LogLevel >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.config.IgnoreRecordNotFoundError):
		sql, rows := query()
		klog.Errorf("%s GORM query failed: %v; elapsed=%s; rows=%d; sql=%s", caller, err, elapsed, rows, sql)
	case l.config.SlowThreshold > 0 && elapsed > l.config.SlowThreshold && l.config.LogLevel >= logger.Warn:
		sql, rows := query()
		klog.Warnf("%s GORM slow query: elapsed=%s; threshold=%s; rows=%d; sql=%s", caller, elapsed, l.config.SlowThreshold, rows, sql)
	case l.config.LogLevel >= logger.Info:
		sql, rows := query()
		klog.Infof("%s GORM query: elapsed=%s; rows=%d; sql=%s", caller, elapsed, rows, sql)
	}
}

func callSite() string {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.Contains(frame.File, "/gorm.io/gorm@") &&
			!strings.Contains(frame.File, "/agent-contracts/pkg/gormlogger/") &&
			!strings.HasPrefix(frame.Function, "runtime.") {
			return frame.File + ":" + fmt.Sprint(frame.Line)
		}
		if !more {
			break
		}
	}
	return "unknown"
}

func (l *Logger) ParamsFilter(_ context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if l.config.ParameterizedQueries {
		return sql, nil
	}
	return sql, params
}

var _ logger.Interface = (*Logger)(nil)
