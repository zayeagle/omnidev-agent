package components

import (
	"strings"
	"testing"
)

func TestCompletionPanelLayout_CollapsedAcceptance(t *testing.T) {
	tn := NewTurn(1, "build snake")
	tn.SetCompletionAcceptance("验收通过 6/6", "/tmp/proj", "── Acceptance verification ──\n[PASS] builds\n", true, 6, 6)
	panel := CompletionPanelLayout(tn, 80)
	body := strings.Join(panel.Lines, "\n")
	if strings.Contains(body, "[PASS] builds") {
		t.Fatalf("detail should be hidden when collapsed: %q", body)
	}
	if panel.AcceptanceToggleLine < 0 {
		t.Fatal("expected acceptance toggle")
	}
	if !strings.Contains(body, "验收通过 6/6 · 查看详细") {
		t.Fatalf("expected collapsed summary: %q", body)
	}
	tn.ToggleAcceptanceExpanded()
	panel = CompletionPanelLayout(tn, 80)
	body = strings.Join(panel.Lines, "\n")
	if !strings.Contains(body, "[PASS] builds") {
		t.Fatalf("expected expanded detail: %q", body)
	}
}
