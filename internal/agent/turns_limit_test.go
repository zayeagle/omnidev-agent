package agent

import "testing"

func TestFormatTurnCounter(t *testing.T) {
	if got := formatTurnCounter(3, 50); got != "3/50" {
		t.Fatalf("got %q", got)
	}
	if got := formatTurnCounter(3, 0); got != "3/∞" {
		t.Fatalf("got %q", got)
	}
}

func TestScaleSubAgentLimitsUnlimited(t *testing.T) {
	turns, _ := scaleSubAgentLimits(Task{ID: "1", Description: repeatString("x", 300)}, 0, defaultScaledTimeout())
	if turns != 0 {
		t.Fatalf("expected unlimited (0), got %d", turns)
	}
}

func TestAgentTurnsUnlimited(t *testing.T) {
	a := New(nil, nil, nil, nil)
	a.SetMaxTurns(0)
	if !a.turnsUnlimited() {
		t.Fatal("expected unlimited")
	}
	a.SetMaxTurns(50)
	if a.turnsUnlimited() {
		t.Fatal("expected capped")
	}
}
