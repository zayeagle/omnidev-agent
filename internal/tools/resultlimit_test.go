package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeliverOutputWithinBudget(t *testing.T) {
	SetResultLimits(ResultLimits{MaxChars: 1000})
	got := DeliverOutput(DeliverOpts{ToolName: "test", Content: "hello"})
	if got != "hello" {
		t.Fatalf("expected passthrough, got %q", got)
	}
}

func TestDeliverOutputPartialWithSpool(t *testing.T) {
	dir := t.TempDir()
	SetResultLimits(ResultLimits{MaxChars: 800, SpoolDir: dir})

	content := strings.Repeat("x", 3000) + "\nTAIL_MARKER\n"
	got := DeliverOutput(DeliverOpts{ToolName: "shell_exec", Content: content})

	if !strings.HasPrefix(got, "[PARTIAL shell_exec:") {
		t.Fatalf("missing PARTIAL banner: %q", got[:min(80, len(got))])
	}
	if !strings.Contains(got, "Continue:") {
		t.Fatal("missing continuation hint")
	}
	if !strings.Contains(got, "TAIL_MARKER") {
		t.Fatal("missing head/tail tail section")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 spool file, got %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Fatal("spool content mismatch")
	}
}

func TestDeliverOutputFileSourceNoDuplicateSpool(t *testing.T) {
	dir := t.TempDir()
	SetResultLimits(ResultLimits{MaxChars: 200, SpoolDir: dir})

	source := filepath.Join(dir, "source.txt")
	content := strings.Repeat("line\n", 100)
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	got := DeliverOutput(DeliverOpts{
		ToolName:   "read_file",
		Content:    content,
		SourcePath: source,
		Hint:       "paginate",
	})

	if !strings.Contains(got, source) {
		t.Fatalf("expected source path in banner, got %q", got[:120])
	}
	spoolEntries, _ := os.ReadDir(dir)
	for _, e := range spoolEntries {
		if strings.HasSuffix(e.Name(), "_read_file.txt") {
			t.Fatal("should not duplicate spool for file reads")
		}
	}
}

func TestReadFileSlice(t *testing.T) {
	data := []byte("a\nb\nc\nd")
	slice, total := readFileSlice(data, 2, 2)
	if total != 4 {
		t.Fatalf("total=%d want 4", total)
	}
	if slice != "b\nc" {
		t.Fatalf("slice=%q want b\\nc", slice)
	}
}

func TestOkLimitedFilePaginationHint(t *testing.T) {
	SetResultLimits(ResultLimits{MaxChars: 100000})
	r := okLimitedFile("read_file", "line1\nline2", "/tmp/f.go", 1, 2, 10)
	if !strings.Contains(r.Data, "offset=3") {
		t.Fatalf("missing next-page hint: %q", r.Data)
	}
}
