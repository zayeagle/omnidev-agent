package tools

import "testing"

func TestFileLineChange_NewFile(t *testing.T) {
	added, removed := FileLineChange("", "line1\nline2\nline3")
	if added != 3 || removed != 0 {
		t.Fatalf("got +%d -%d, want +3 -0", added, removed)
	}
}

func TestSnippetLineChange(t *testing.T) {
	added, removed := SnippetLineChange("a\nb", "a\nb\nc\nd")
	if added != 2 || removed != 0 {
		t.Fatalf("got +%d -%d, want +2 -0", added, removed)
	}
}

func TestFormatChange(t *testing.T) {
	got := FormatChange("edited", "main.go", 5, 2)
	if got != "edited main.go (+5 -2)" {
		t.Fatalf("unexpected: %q", got)
	}
}
