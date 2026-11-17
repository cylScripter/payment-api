package initials

import (
	stdlog "log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

func InitLogger() {
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		CallerPrettyfier: func(frame *runtime.Frame) (string, string) {
			return frame.Function, filepath.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
		},
	})
	log.SetReportCaller(true)
	log.SetOutput(os.Stderr)
	log.SetLevel(log.InfoLevel)

	writer := logWriter{level: log.InfoLevel}
	stdlog.SetFlags(0)
	stdlog.SetPrefix("")
	stdlog.SetOutput(writer)
}

type logWriter struct {
	level log.Level
}

func (w logWriter) Write(p []byte) (int, error) {
	log.StandardLogger().Log(w.level, strings.TrimSpace(string(p)))
	return len(p), nil
}
