package agent

import (
	"fmt"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

const testEvidenceReason = "go test ./... passed — behavioral coverage via tests"

// applyTestEvidencePass marks functional criteria satisfied when mechanical tests are green.
func applyTestEvidencePass(statuses []CriterionStatus, plan AcceptancePlan, mech mechanicalVerifyResult, sess *session.Session) []CriterionStatus {
	if !mech.TestOK || !mech.TestsRan || !mech.BuildOK {
		return statuses
	}
	writes, reads, searches, lists := countSessionTools(sess)
	if writes < 1 {
		return statuses
	}
	explore := explorationOps(reads, searches, lists)
	out := append([]CriterionStatus(nil), statuses...)
	for i := range out {
		lower := strings.ToLower(out[i].Text)
		if isBuildTestCriterion(lower) {
			continue
		}
		if isFunctionalCriterion(lower) || shouldInferFunctionalFromTests(lower, plan) {
			if out[i].Met && strings.Contains(out[i].Evidence, "go test") {
				continue
			}
			out[i].Met = true
			out[i].Evidence = fmt.Sprintf("%s (%d writes, %d exploration ops)", testEvidenceReason, writes, explore)
		}
	}
	return out
}

func shouldInferFunctionalFromTests(lower string, plan AcceptancePlan) bool {
	if isFunctionalCriterion(lower) {
		return true
	}
	// Heuristic plan's first criterion ("Address the user request") is functional even if wording differs.
	if len(plan.Criteria) > 0 && strings.EqualFold(strings.TrimSpace(plan.Criteria[0]), strings.TrimSpace(lower)) {
		return true
	}
	if strings.Contains(lower, "address the user") || strings.Contains(lower, "user request") ||
		strings.Contains(lower, "需求") || strings.Contains(lower, "功能") {
		return true
	}
	return false
}

// mechanicalTestsSatisfyAcceptance reports whether green tests + file changes are enough to finish.
func mechanicalTestsSatisfyAcceptance(statuses []CriterionStatus, plan AcceptancePlan, mech mechanicalVerifyResult, sess *session.Session) bool {
	if !mech.allOK() || !mech.TestOK || !mech.TestsRan || !mech.BuildOK {
		return false
	}
	upgraded := applyTestEvidencePass(statuses, plan, mech, sess)
	upgraded = applyMechanicalCrossCheck(upgraded, plan, mech, sess)
	return allCriteriaMet(upgraded)
}
