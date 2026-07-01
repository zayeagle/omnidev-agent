package runlog

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestResolveLogDir_WindowsLocalAppData(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only")
	}
	t.Setenv("OMNIDEV_LOG_DIR", "")
	t.Setenv("LOCALAPPDATA", t.TempDir())
	dir, err := resolveLogDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(os.Getenv("LOCALAPPDATA"), "omnidev-agent", "logs")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}

func TestResolveLogDir_EnvOverride(t *testing.T) {
	custom := t.TempDir()
	t.Setenv("OMNIDEV_LOG_DIR", custom)
	dir, err := resolveLogDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != custom {
		t.Fatalf("got %q want %q", dir, custom)
	}
}
