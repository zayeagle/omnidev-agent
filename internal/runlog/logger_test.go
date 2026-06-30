package runlog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerWrites(t *testing.T) {
	dir := t.TempDir()
	orig := os.Args[0]
	// Use temp dir as fake executable path via direct file create
	path := filepath.Join(dir, "test.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	l := &Logger{path: path, f: f}
	l.Line("test", "hello %s", "world")
	_ = l.Close()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "hello world") {
		t.Fatalf("log: %s", data)
	}
	_ = orig
}
