package runlog

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// Logger appends timestamped execution lines to a log file.
type Logger struct {
	mu   sync.Mutex
	path string
	f    *os.File
}

// resolveLogDir picks a writable log directory (not the binary install folder).
func resolveLogDir() (string, error) {
	if v := os.Getenv("OMNIDEV_LOG_DIR"); v != "" {
		return v, nil
	}
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "omnidev-agent", "logs"), nil
		}
	}
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return filepath.Join(h, ".local", "share", "omnidev-agent", "logs"), nil
	}
	return executableDir()
}

// NewInExecutableDir creates omnidev-agent-YYYYMMDD-HHMMSS.log in the default log directory.
// Logs live under %LOCALAPPDATA%/omnidev-agent/logs on Windows (writable, survives binary updates).
// Set OMNIDEV_LOG_DIR to override. Falls back to the executable directory when needed.
func NewInExecutableDir() (*Logger, error) {
	dir, err := resolveLogDir()
	if err != nil {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fallback, fbErr := executableDir()
		if fbErr != nil {
			return nil, fmt.Errorf("log dir %q: %w", dir, err)
		}
		dir = fallback
	}
	name := fmt.Sprintf("omnidev-agent-%s.log", time.Now().Format("20060102-150405"))
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log %q: %w", path, err)
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
