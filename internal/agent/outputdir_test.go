package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDisplayOutputDirRelative(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(cwd, "deliverables", "snake-game")
	got := displayOutputDir(abs)
	want := "deliverables/snake-game"
	if got != want {
		t.Fatalf("displayOutputDir = %q, want %q", got, want)
	}
}

func TestDisplayOutputDirOutsideCWD(t *testing.T) {
	got := displayOutputDir("/tmp/outside")
	if got != "/tmp/outside" {
		t.Fatalf("displayOutputDir = %q", got)
	}
}
