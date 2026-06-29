package components

import (
	"strings"
	"testing"
)

func TestParseChangeFromToolResult(t *testing.T) {
	fc, ok := ParseChangeFromToolResult("edit_file", "edited internal/agent/loop.go (+12 -3)")
	if !ok || fc.Path != "internal/agent/loop.go" || fc.Added != 12 || fc.Removed != 3 {
		t.Fatalf("got %+v ok=%v", fc, ok)
	}
	fc, ok = ParseChangeFromToolResult("edit_file", "edited ./deliverables/foo.go (+5)")
	if !ok || fc.Path != "deliverables/foo.go" || fc.Added != 5 {
		t.Fatalf("normalized path: %+v ok=%v", fc, ok)
	}
}

func TestRecordFileChangeMergesPaths(t *testing.T) {
	tn := NewTurn(1, "x")
	tn.RecordFileChange(FileChange{Path: "./a.go", Added: 1})
	tn.RecordFileChange(FileChange{Path: "a.go", Added: 2})
	if len(tn.fileChanges) != 1 || tn.fileChanges[0].Added != 3 {
		t.Fatalf("changes=%+v", tn.fileChanges)
	}
}

func TestChangePanelLines(t *testing.T) {
	tn := NewTurn(1, "fix")
	tn.RecordFileChange(FileChange{Path: "a.go", Added: 10, Removed: 2})
	tn.RecordFileChange(FileChange{Path: "deliverables/snake-game/main.go", Removed: 422})
	tn.MarkDone()
	lines := ChangePanelLines(tn, 80)
	if len(lines) == 0 {
		t.Fatal("expected change panel")
	}
	body := strings.Join(lines, "\n")
	if strings.Contains(body, "(+") && !strings.Contains(body, "a.go") {
		t.Fatalf("orphaned stats in output: %q", body)
	}
}
