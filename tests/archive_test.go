package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestListArchives(t *testing.T) {
	dir := t.TempDir()
	sessions := filepath.Join(dir, "sessions")
	logs := filepath.Join(dir, "logs")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(logs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sessions, "20260101-test.md"), []byte("# test"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := session.ListArchives(sessions, logs, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 archive, got %d", len(files))
	}
	if files[0].Kind != "session" {
		t.Fatalf("expected session kind, got %s", files[0].Kind)
	}
}
