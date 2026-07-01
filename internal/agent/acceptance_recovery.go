package agent

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const acceptanceStallThreshold = 3

const acceptanceRecoveryPrefix = `[ACCEPTANCE RECOVERY] Verification did not pass yet.

First classify the failure (path/workspace vs environment vs unit tests vs application code).
Read the diagnosis and verify output — do not assume every test failure is an application bug.
Fix the classified root cause with tools, then re-verify.`

// driveUntilAccepted runs final acceptance and, on failure, injects gap analysis and
// re-enters the agent loop until criteria pass or a non-recoverable error occurs.
func (a *Agent) driveUntilAccepted(ctx context.Context, msgCh chan<- tea.Msg, instruction string, includeTools bool, results []TaskResult) ([]CriterionStatus, bool) {
	if !includeTools || !a.acceptanceStrict {
		return nil, true
	}

	maxCycles := a.effectiveMaxAcceptanceCycles()
	acceptTurns := a.effectiveAcceptanceMaxTurns()

	var lastStatuses []CriterionStatus
	var lastMech mechanicalVerifyResult
	var lastFailKey string
	stallCount := 0

	for cycle := 0; cycle < maxCycles; cycle++ {
		if a.state == StateError {
			return lastStatuses, false
		}
		select {
		case <-ctx.Done():
			return lastStatuses, false
		default:
		}

		statuses, accepted := a.runFinalAcceptanceGate(ctx, msgCh, instruction, results)
		lastStatuses = statuses
		if accepted && allCriteriaMet(statuses) {
			a.clearAcceptanceIncompleteCheckpoint()
			a.logRun("acceptance", "passed after %d recovery cycle(s)", cycle)
			return statuses, true
		}

		mech := a.lastMechanicalVerify()
		lastMech = mech
		plan := a.ensureAcceptancePlan(ctx, instruction)
		if mechanicalTestsSatisfyAcceptance(statuses, plan, mech, a.session) {
			upgraded := applyTestEvidencePass(statuses, plan, mech, a.session)
			upgraded = applyMechanicalCrossCheck(upgraded, plan, mech, a.session)
			a.clearAcceptanceIncompleteCheckpoint()
			a.logRun("recovery", "skipped cycle=%d reason=mechanical_green+test_evidence criteria=%d/%d",
				cycle+1, countCriteriaMet(upgraded), len(upgraded))
			msgCh <- StreamChunkMsg{Content: formatAcceptanceReport(upgraded, mech), Done: true}
			return upgraded, true
		}
		report := formatAcceptanceReport(statuses, mech)
		a.logRun("acceptance", "failed cycle=%d/%d report:\n%s", cycle+1, maxCycles, report)
		msgCh <- StreamChunkMsg{Content: report, Done: true}

		failKey := acceptanceFailureKey(statuses, mech)
		if failKey != "" && failKey == lastFailKey {
			stallCount++
		} else {
			stallCount = 0
			lastFailKey = failKey
		}
		if stallCount >= acceptanceStallThreshold {
			stallMsg := fmt.Sprintf("[ACCEPTANCE STALLED] Same failure repeated %d times — stopping auto-recovery. See Acceptance criteria and report above.", stallCount+1)
			if hint := diagnosisStallHint(lastMech.Diagnosis); hint != "" {
				stallMsg += "\n" + hint
			}
			msgCh <- StreamChunkMsg{Content: stallMsg, Done: true}
			emitActivity(msgCh, "Acceptance stalled — same failures repeated")
			a.logRun("acceptance", "stalled: %s", failKey)
			return lastStatuses, false
		}

		a.injectAcceptanceRecovery(statuses, instruction, mech)
		emitActivity(msgCh, "Working · recovering acceptance…")

		savedPhase := a.loopPhase
		savedCycle := a.acceptanceCycle
		savedMax := a.maxTurns
		a.loopPhase = LoopPhaseAcceptance
		a.acceptanceCycle = cycle + 1
		a.maxTurns = acceptTurns
		err := a.agentLoop(ctx, msgCh, includeTools)
		a.maxTurns = savedMax
		a.loopPhase = savedPhase
		a.acceptanceCycle = savedCycle
		if err != nil {
			return lastStatuses, false
		}
	}

	report := formatAcceptanceReport(lastStatuses, lastMech)
	limitMsg := fmt.Sprintf("[ACCEPTANCE] Max recovery cycles reached (%d/%d) — still not passing.", maxCycles, maxCycles)
	msgCh <- StreamChunkMsg{Content: report + "\n\n" + limitMsg, Done: true}
	summary := a.BuildSessionSummary(ctx, SessionOutcomeFailed, limitMsg, results, lastStatuses)
	if summary != "" {
		msgCh <- StreamChunkMsg{Content: summary, Done: true}
	}
	a.logRun("acceptance", "max recovery cycles exhausted (%d)", maxCycles)
	statuses, accepted := a.runFinalAcceptanceGate(ctx, msgCh, instruction, results)
	return statuses, accepted && allCriteriaMet(statuses)
}

func acceptanceFailureKey(statuses []CriterionStatus, mech mechanicalVerifyResult) string {
	var b strings.Builder
	for _, s := range statuses {
		if !s.Met {
			b.WriteString(s.Text)
			b.WriteString("|")
			b.WriteString(s.Evidence)
			b.WriteString(";")
		}
	}
	if !mech.allOK() {
		b.WriteString(truncateForDigest(mech.Summary, 160))
	}
	return b.String()
}

func (a *Agent) lastMechanicalVerify() mechanicalVerifyResult {
	return a.pendingMech
}
