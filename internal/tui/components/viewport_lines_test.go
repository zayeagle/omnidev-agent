package components

import (
	"strings"
	"testing"
)

func TestFlattenViewportLines_ExpandsEmbeddedNewlines(t *testing.T) {
	in := []string{"one", "two\nthree", "", "four"}
	got := FlattenViewportLines(in)
	if len(got) != 5 {
		t.Fatalf("len=%d want 5: %v", len(got), got)
	}
	if got[1] != "two" || got[2] != "three" {
		t.Fatalf("split: %v", got)
	}
}

func TestViewScroll_MultiLineSuffixUsesFlatRowCount(t *testing.T) {
	tl := NewTurnList(10)
	suffix := []string{"head", "box\nline2\nline3", "tail"}
	flat := FlattenViewportLines(suffix)
	if len(flat) != 5 {
		t.Fatalf("flat len=%d want 5", len(flat))
	}
	view, start := tl.ViewScroll(3, nil, suffix)
	if start != len(flat)-3 {
		t.Fatalf("pinned start=%d want %d", start, len(flat)-3)
	}
	if !strings.Contains(view, "tail") || !strings.Contains(view, "line2") {
		t.Fatalf("view missing suffix rows:\n%s", view)
	}
}

func TestCompletionPanelToggleLines_StableAcrossExpandCollapse(t *testing.T) {
	tn := NewTurn(1, "x")
	tn.SetTasks([]*TaskEntry{
		{ID: "1", Description: "implement", Status: StatusSuccess},
		{ID: "2", Description: "verify: go build", Status: StatusSuccess},
	})
	tn.SetCompletionAcceptance("summary", "E:\\proj",
		"── Acceptance verification ──\n[PASS] builds\n[PASS] tests\n",
		true, 2, 2)

	panel := CompletionPanelLayout(tn, 80)
	flat := FlattenViewportLines(panel.Lines)
	if !strings.Contains(strings.Join(flat, "\n"), "Completed sub-tasks") {
		t.Fatalf("expected tasks section: %v", flat)
	}
	if !strings.Contains(strings.Join(flat, "\n"), "[PASS] builds") {
		t.Fatal("acceptance detail should always be visible")
	}
}
