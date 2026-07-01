package agent

import "testing"

func TestEffectiveAcceptanceLimitsDefaults(t *testing.T) {
	a := New(nil, nil, nil, nil)
	if a.effectiveMaxAcceptanceCycles() != 10 {
		t.Fatalf("cycles=%d want 10", a.effectiveMaxAcceptanceCycles())
	}
	if a.effectiveAcceptanceMaxTurns() != 10 {
		t.Fatalf("turns=%d want 10", a.effectiveAcceptanceMaxTurns())
	}
}

func TestLoopActivityLabelSeparatesPhases(t *testing.T) {
	a := New(nil, nil, nil, nil)
	a.maxTurns = 20
	if got := a.loopActivityLabel(7, 20); got != "Working · calling model (turn 8/20)…" {
		t.Fatalf("implement: %q", got)
	}
	a.loopPhase = LoopPhaseAcceptance
	a.acceptanceCycle = 3
	a.maxTurns = 10
	a.maxAcceptanceCycles = 10
	if got := a.loopActivityLabel(2, 10); got != "Acceptance recovery 3/10 · turn 3/10…" {
		t.Fatalf("acceptance: %q", got)
	}
}

func TestLoopExhaustedReason(t *testing.T) {
	a := New(nil, nil, nil, nil)
	a.maxTurns = 20
	if !containsStr(a.loopExhaustedReason(), "task iteration") {
		t.Fatalf("implement reason: %q", a.loopExhaustedReason())
	}
	a.loopPhase = LoopPhaseAcceptance
	a.maxTurns = 10
	if !containsStr(a.loopExhaustedReason(), "acceptance recovery") {
		t.Fatalf("acceptance reason: %q", a.loopExhaustedReason())
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
