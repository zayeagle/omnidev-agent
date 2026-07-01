package tui

import "testing"

func TestIsMajorProcessMessage(t *testing.T) {
	if !isMajorProcessMessage("Acceptance 2/5 not met — recovery round 1…") {
		t.Fatal("recovery should be major")
	}
	if isMajorProcessMessage("Working · calling model (turn 3/20)…") {
		t.Fatal("routine model call should not be major status line")
	}
}

func TestIsAgentProcessMessage(t *testing.T) {
	if isAgentProcessMessage("Working · calling model (turn 1/20)…") {
		t.Fatal("calling model should not appear in transcript status lines")
	}
	if !isAgentProcessMessage("Working · checking acceptance criteria…") {
		t.Fatal("major process should still route to status")
	}
}
