package agent

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const maxAcceptanceRecoveryCycles = 15
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

	extraTurns := a.maxTurns
	if extraTurns < 15 {
		extraTurns = 15
	}

	var lastStatuses []CriterionStatus
	var lastMech mechanicalVerifyResult
	var lastFailKey string
	stallCount := 0

	for cycle := 0; cycle < maxAcceptanceRecoveryCycles; cycle++ {
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
		a.logRun("acceptance", "failed cycle=%d report:\n%s", cycle+1, report)
		msgCh <- StreamChunkMsg{Content: report, Done: true}

		failKey := acceptanceFailureKey(statuses, mech)
		if failKey != "" && failKey == lastFailKey {
			stallCount++
		} else {
			stallCount = 0
			lastFailKey = failKey
		}
		if stallCount >= acceptanceStallThreshold {
			stallMsg := fmt.Sprintf("[验收停滞] 相同失败已连续 %d 次，停止自动恢复。未通过原因见上方 Acceptance criteria 与报告。", stallCount+1)
			if hint := diagnosisStallHint(lastMech.Diagnosis); hint != "" {
				stallMsg += "\n" + hint
			}
			msgCh <- StreamChunkMsg{Content: stallMsg, Done: true}
			emitActivity(msgCh, "Acceptance stalled — same failures repeated")
			a.logRun("acceptance", "stalled: %s", failKey)
			return lastStatuses, false
		}

		a.injectAcceptanceRecovery(statuses, instruction, mech)
		passed, met := countCriteriaMet(statuses), len(statuses)
		emitActivity(msgCh, fmt.Sprintf("Acceptance %d/%d not met — recovery round %d…", passed, met, cycle+1))

		savedMax := a.maxTurns
		a.maxTurns = savedMax + extraTurns
		err := a.agentLoop(ctx, msgCh, includeTools)
		a.maxTurns = savedMax
		if err != nil {
			return lastStatuses, false
		}
	}

	report := formatAcceptanceReport(lastStatuses, lastMech)
	msgCh <- StreamChunkMsg{Content: report + "\n\n[验收] 已达最大恢复轮次 (" + fmt.Sprint(maxAcceptanceRecoveryCycles) + ")，仍未通过。", Done: true}
	a.logRun("acceptance", "max recovery cycles exhausted")
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
	// Re-run lightweight read from session last [VERIFICATION] block is expensive; caller already ran verify in gate.
	// Use checkpoint/session hint: runFinalAcceptanceGate already computed mech — store on agent briefly.
	return a.pendingMech
}
