package initials

import (
	"io"
	stdlog "log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mattn/go-isatty"
	log "github.com/sirupsen/logrus"
)

var logFile dailyLogWriter

func InitLogger() error {
	log.SetFormatter(&log.JSONFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000",
		CallerPrettyfier: func(frame *runtime.Frame) (string, string) {
			return frame.Function, filepath.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
		},
	})
	log.SetReportCaller(true)
	log.SetOutput(os.Stderr)
	log.SetLevel(log.InfoLevel)

	logFile.ip = os.Getenv("POD_IP")
	if logFile.ip == "" {
		addresses, err := net.InterfaceAddrs()
		if err != nil {
			return err
		}
		for _, address := range addresses {
			ip, ok := address.(*net.IPNet)
			if !ok || ip.IP.IsLoopback() {
				continue
			}
			logFile.ip = ip.IP.String()
			if ip.IP.To4() != nil {
				break
			}
		}
		if logFile.ip == "" {
			logFile.ip = "127.0.0.1"
		}
	}
	if err := os.MkdirAll("logs", 0755); err != nil {
		return err
	}
	if err := logFile.rotate(time.Now().Format("2006-01-02")); err != nil {
		return err
	}
	log.SetOutput(io.MultiWriter(consoleWriter{
		ip:    logFile.ip,
		color: isatty.IsTerminal(os.Stderr.Fd()),
	}, &logFile))

	writer := logWriter{level: log.InfoLevel}
	gin.DefaultWriter = writer
	gin.DefaultErrorWriter = logWriter{level: log.ErrorLevel}
	stdlog.SetFlags(0)
	stdlog.SetPrefix("")
	stdlog.SetOutput(writer)
	return nil
}

type logWriter struct {
	level log.Level
}

func (w logWriter) Write(p []byte) (int, error) {
	log.StandardLogger().Log(w.level, strings.TrimSpace(string(p)))
	return len(p), nil
}
