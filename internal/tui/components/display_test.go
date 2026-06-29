package components

import "testing"

func TestSplitToolDescription(t *testing.T) {
	tool, detail := SplitToolDescription("shell_exec: mkdir -p foo")
	if tool != "shell_exec" || detail != "mkdir -p foo" {
		t.Fatalf("got %q / %q", tool, detail)
	}
	tool, detail = SplitToolDescription("no colon here")
	if tool != "" || detail != "no colon here" {
		t.Fatalf("got %q / %q", tool, detail)
	}
}

func TestTruncateMiddle(t *testing.T) {
	long := "mkdir -p deliverables/hello-server/cmd deliverables/hello-server/internal/domain"
	got := TruncateMiddle(long, 40)
	if len(got) >= len(long) {
		t.Fatalf("expected shorter string, got %q", got)
	}
	if !containsSubstring(got, "…") {
		t.Fatalf("expected ellipsis in %q", got)
	}
}

func TestWrapCommandLinesCapsRows(t *testing.T) {
	text := "one two three four five six seven eight nine ten eleven twelve"
	lines := wrapCommandLines(text, 12, 2)
	if len(lines) > 2 {
		t.Fatalf("expected at most 2 lines, got %d: %v", len(lines), lines)
	}
}
