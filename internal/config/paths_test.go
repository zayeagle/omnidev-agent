package config

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultGlobalConfigPath(t *testing.T) {
	p := DefaultGlobalConfigPath()
	if p == "" {
		t.Fatal("expected non-empty path")
	}
	if !strings.HasSuffix(filepath.ToSlash(p), "/.omnidev-agent/config.json") {
		t.Fatalf("unexpected path: %s", p)
	}
	if runtime.GOOS == "windows" {
		if !strings.Contains(p, `\`) && !strings.Contains(p, "/") {
			t.Fatalf("expected windows path separators in %s", p)
		}
	}
}
