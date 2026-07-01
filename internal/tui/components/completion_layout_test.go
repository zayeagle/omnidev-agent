package components

import (
	"strings"
	"testing"
)

func TestCompletionPanelLayout_ShowsTasksAndSummary(t *testing.T) {
	tn := NewTurn(1, "build snake")
	tn.SetTasks([]*TaskEntry{
		{ID: "1", Description: "backend", Status: StatusSuccess},
		{ID: "2", Description: "frontend", Status: StatusSuccess},
	})
	tn.SetCompletion("Changes & optimizations:\n- snake game complete\n\nNext steps:\n- polish UI", "/tmp/proj")
	panel := CompletionPanelLayout(tn, 80)
	body := strings.Join(panel.Lines, "\n")
	for _, want := range []string{"Completed sub-tasks", "backend", "Summary", "snake game", "Project location"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	idxSummary := strings.Index(body, "Summary")
	idxProject := strings.Index(body, "Project location")
	if idxSummary < 0 || idxProject < 0 || idxSummary > idxProject {
		t.Fatalf("expected Summary before Project location:\n%s", body)
	}
}
