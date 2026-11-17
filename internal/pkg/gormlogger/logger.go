package gormlogger

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Logger adapts GORM logs to logrus.
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
		log.Infof(msg, data...)
	}
}

func (l *Logger) Warn(_ context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Warn {
		log.Warnf(msg, data...)
	}
}

func (l *Logger) Error(_ context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel >= logger.Error {
		log.Errorf(msg, data...)
	}
}

func (l *Logger) Trace(ctx context.Context, begin time.Time, query func() (string, int64), err error) {
	if l.config.LogLevel == logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	entry := log.WithContext(ctx)
	entry.Caller = callSite()
	switch {
	case err != nil && l.config.LogLevel >= logger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !l.config.IgnoreRecordNotFoundError):
		sql, rows := query()
		entry.Errorf("[GORM] query failed: %v; elapsed=%s; rows=%d; sql=%s", err, elapsed, rows, sql)
	case l.config.SlowThreshold > 0 && elapsed > l.config.SlowThreshold && l.config.LogLevel >= logger.Warn:
		sql, rows := query()
		entry.Warnf("[GORM] slow query: elapsed=%s; threshold=%s; rows=%d; sql=%s", elapsed, l.config.SlowThreshold, rows, sql)
	case l.config.LogLevel >= logger.Info:
		sql, rows := query()
		entry.Infof("[GORM] query: elapsed=%s; rows=%d; sql=%s", elapsed, rows, sql)
	}
}

func callSite() *runtime.Frame {
	var pcs [32]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if !strings.HasPrefix(frame.Function, "gorm.io/") &&
			!strings.HasPrefix(frame.Function, "payment-api/internal/pkg/gormlogger.") &&
			!strings.HasPrefix(frame.Function, "runtime.") {
			return &frame
		}
		if !more {
			break
		}
	}
	return nil
}

func (l *Logger) ParamsFilter(_ context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if l.config.ParameterizedQueries {
		return sql, nil
	}
	return sql, params
}

var _ logger.Interface = (*Logger)(nil)
