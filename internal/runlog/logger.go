package runlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Logger appends timestamped execution lines to a file under the executable directory.
type Logger struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// NewInExecutableDir creates omnidev-agent-YYYYMMDD-HHMMSS.log next to the running binary.
func NewInExecutableDir() (*Logger, error) {
	dir, err := executableDir()
	if err != nil {
		dir = "."
	}
	name := fmt.Sprintf("omnidev-agent-%s.log", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	l := &Logger{path: path, f: f}
	l.Line("init", "run log started path=%s", path)
	return l, nil
}

func executableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exe), nil
}

// Path returns the log file path.
func (l *Logger) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}

// Line writes one log record.
func (l *Logger) Line(category, format string, args ...interface{}) {
	if l == nil || l.f == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.f, "%s [%s] %s\n", ts, category, msg)
}

// LineDuration writes a log record with elapsed time since start.
func (l *Logger) LineDuration(category string, start time.Time, format string, args ...interface{}) {
	if l == nil {
		return
	}
	elapsed := time.Since(start).Round(time.Millisecond)
	msg := fmt.Sprintf(format, args...)
	l.Line(category, "%s duration=%s", msg, elapsed)
}

// Close flushes and closes the log file.
func (l *Logger) Close() error {
	if l == nil || l.f == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.f.Close()
	l.f = nil
	return err
}
