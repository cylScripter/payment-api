package initials

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type dailyLogWriter struct {
	mu   sync.Mutex
	ip   string
	date string
	file *os.File
}

func (w *dailyLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	date := time.Now().Format("2006-01-02")
	if w.file == nil || w.date != date {
		if err := w.rotate(date); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *dailyLogWriter) rotate(date string) error {
	path := filepath.Join("logs", date+"-"+w.ip+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	if w.file != nil {
		_ = w.file.Close()
	}
	w.file = file
	w.date = date
	return nil
}

func (w *dailyLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}
