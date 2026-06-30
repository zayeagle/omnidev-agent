package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyProjectWorkspace_BuildOnly(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/test\n\ngo 1.24\n")
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc main() {}\n")

	summary, ok := VerifyProjectWorkspace(t.Context(), dir)
	if !ok {
		t.Fatalf("expected build ok, got %q", summary)
	}
	if summary == "" || !strings.Contains(summary, "Build check passed") {
		t.Fatalf("unexpected summary: %q", summary)
	}
}

func TestVerifyProjectWorkspace_SkipsWithoutGoMod(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.py"), "print('hi')\n")
	summary, ok := VerifyProjectWorkspace(t.Context(), dir)
	if !ok {
		t.Fatalf("expected python compile ok or skip, got %q", summary)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
