package components

import (
	"strings"
	"testing"
)

func TestCompletionPanelLayout_CollapsedTasks(t *testing.T) {
	tn := NewTurn(1, "build snake")
	tn.SetTasks([]*TaskEntry{
		{ID: "1", Description: "backend", Status: StatusSuccess},
		{ID: "2", Description: "frontend", Status: StatusSuccess},
	})
	tn.SetCompletion("Snake game is feature-complete. Build passed.", "/tmp/proj")
	panel := CompletionPanelLayout(tn, 80)
	body := strings.Join(panel.Lines, "\n")
	if !strings.Contains(body, "Snake game is feature-complete") {
		t.Fatalf("expected conclusion in panel: %q", body)
	}
	if idx := strings.Index(body, "Snake game"); idx < 0 || strings.Index(body, "Project location:") < idx {
		t.Fatalf("conclusion should appear before project location: %q", body)
	}
	if panel.TasksToggleLine < 0 {
		t.Fatal("expected tasks toggle line")
	}
	if !strings.Contains(body, "▸ To-dos") {
		t.Fatalf("expected collapsed toggle: %q", body)
	}
	tn.ToggleTasksExpanded()
	panel = CompletionPanelLayout(tn, 80)
	body = strings.Join(panel.Lines, "\n")
	if !strings.Contains(body, "▾ To-dos") || !strings.Contains(body, "backend") {
		t.Fatalf("expected expanded tasks: %q", body)
	}
}
