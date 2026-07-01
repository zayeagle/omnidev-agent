package components

import (
	"strings"
	"testing"
)

func TestCompletionPanelLayout_ShowsFullAcceptance(t *testing.T) {
	tn := NewTurn(1, "build snake")
	tn.SetTasks([]*TaskEntry{
		{ID: "1", Description: "implement snake", Status: StatusSuccess},
	})
	tn.SetCompletionAcceptance(
		"Changes & optimizations:\n- implemented snake game\n\nNext steps:\n- add tests",
		"/tmp/proj",
		"── Acceptance verification ──\n[PASS] builds\n[PASS] tests\n",
		true, 2, 2,
	)
	panel := CompletionPanelLayout(tn, 80)
	body := strings.Join(panel.Lines, "\n")
	for _, want := range []string{
		"Task completed",
		"Completed sub-tasks",
		"Acceptance results",
		"[PASS] builds",
		"Summary",
		"implemented snake",
		"Project location",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestCompletionPanelLayout_FailedShowsReasonAndSummary(t *testing.T) {
	tn := NewTurn(1, "build")
	tn.SetTasks([]*TaskEntry{{ID: "1", Description: "work", Status: StatusFailed}})
	tn.SetCompletionReport(
		"Recommended solution:\n- fix build errors and resume",
		"/tmp/proj",
		"[FAIL] build\n",
		false, 0, 1,
		true,
		"verification failed",
	)
	panel := CompletionPanelLayout(tn, 80)
	body := strings.Join(panel.Lines, "\n")
	for _, want := range []string{"Task incomplete", "Failure reason", "verification failed", "Acceptance results", "Summary"} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
