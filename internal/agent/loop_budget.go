package agent

import "fmt"

// LoopPhase separates implement/tool loops from acceptance recovery loops.
type LoopPhase int

const (
	LoopPhaseImplement LoopPhase = iota
	LoopPhaseAcceptance
)

const (
	defaultMaxAcceptanceCycles = 10
	defaultAcceptanceMaxTurns  = 10
)

// SetAcceptanceLimits configures acceptance recovery (separate from maxTurns implement budget).
func (a *Agent) SetAcceptanceLimits(maxCycles, turnsPerCycle int) {
	if maxCycles > 0 {
		a.maxAcceptanceCycles = maxCycles
	}
	if turnsPerCycle > 0 {
		a.acceptanceMaxTurns = turnsPerCycle
	}
}

func (a *Agent) effectiveMaxAcceptanceCycles() int {
	if a.maxAcceptanceCycles > 0 {
		return a.maxAcceptanceCycles
	}
	return defaultMaxAcceptanceCycles
}

func (a *Agent) effectiveAcceptanceMaxTurns() int {
	if a.acceptanceMaxTurns > 0 {
		return a.acceptanceMaxTurns
	}
	return defaultAcceptanceMaxTurns
}

func (a *Agent) loopActivityLabel(turn, max int) string {
	if a.loopPhase == LoopPhaseAcceptance {
		return fmt.Sprintf("Acceptance recovery %s · turn %s…",
			formatTurnCounter(a.acceptanceCycle, a.effectiveMaxAcceptanceCycles()),
			formatTurnCounter(turn+1, max))
	}
	return fmt.Sprintf("Working · calling model (turn %s)…", formatTurnCounter(turn+1, max))
}

func (a *Agent) loopExhaustedReason() string {
	if a.loopPhase == LoopPhaseAcceptance {
		return fmt.Sprintf("acceptance recovery turn limit (%s per cycle)", formatTurnLimitWords(a.maxTurns))
	}
	return fmt.Sprintf("task iteration limit (%s)", formatTurnLimitWords(a.maxTurns))
}
