package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

const exitGateNudgePrefix = `[ACCEPTANCE INCOMPLETE] You stopped before the request was fully satisfied.

Review the gaps below, use tools to finish the work, then reply without tool calls only when every criterion is met.`

// AcceptancePlan holds structured completion criteria for a user request.
type AcceptancePlan struct {
	Criteria       []string `json:"acceptance_criteria"`
	VerifyCommands []string `json:"verify_commands,omitempty"`
}

// CriterionStatus tracks one criterion through verification.
type CriterionStatus struct {
	Index    int    `json:"index"`
	Text     string `json:"text"`
	Met      bool   `json:"met"`
	Skipped  bool   `json:"skipped,omitempty"`
	Evidence string `json:"evidence,omitempty"`
}

func (a *Agent) SetAcceptanceStrict(strict bool) { a.acceptanceStrict = strict }

func (a *Agent) ensureAcceptancePlan(ctx context.Context, instruction string) AcceptancePlan {
	if a.acceptancePlan != nil && len(a.acceptancePlan.Criteria) > 0 {
		return mergeVerifyCommands(*a.acceptancePlan, instruction)
	}
	plan := extractAcceptanceCriteria(ctx, a, instruction)
	a.acceptancePlan = &plan
	return plan
}

func extractAcceptanceCriteria(ctx context.Context, a *Agent, instruction string) AcceptancePlan {
	var plan AcceptancePlan
	if a.provider != nil && a.acceptanceStrict && a.pipelineOpts.UseLLMAcceptance {
		if extracted, err := extractAcceptanceCriteriaLLM(ctx, a, instruction); err == nil && len(extracted.Criteria) > 0 {
			plan = extracted
			return mergeVerifyCommands(plan, instruction)
		}
	}
	plan = heuristicAcceptancePlan(instruction)
	return mergeVerifyCommands(plan, instruction)
}

func extractAcceptanceCriteriaLLM(ctx context.Context, a *Agent, instruction string) (AcceptancePlan, error) {
	prompt := fmt.Sprintf(`Extract acceptance criteria for this coding request.

Output ONLY valid JSON:
{"acceptance_criteria":["..."],"verify_commands":["go build ./..."]}

Rules:
- 3 to 7 specific, testable criteria in the same language as the user request
- Include functional requirements AND quality gates (build/tests when applicable)
- verify_commands: shell commands for mechanical checks (may be empty)

Request:
%s`, instruction)

	messages := []llm.Message{
		{Role: "system", Content: "Output only JSON. No markdown."},
		{Role: "user", Content: prompt},
	}
	resp, err := stream.RetryChat(ctx, a.provider, &llm.Request{Messages: messages}, a.retryConfig)
	if err != nil {
		return AcceptancePlan{}, err
	}
	var plan AcceptancePlan
	if err := json.Unmarshal([]byte(cleanJSON(resp.Content)), &plan); err != nil {
		return AcceptancePlan{}, err
	}
	return plan, nil
}

func heuristicAcceptancePlan(instruction string) AcceptancePlan {
	text := strings.TrimSpace(instruction)
	if text == "" {
		text = "Complete the user request"
	}
	if looksLikeReviewInstruction(instruction) {
		return AcceptancePlan{
			Criteria: []string{
				"Answer the user's review/analysis question with a structured conclusion",
				"Exploration covered main packages (list_dir + read key source files)",
				"Conclusion states completeness, gaps, bugs, and build readiness where relevant",
			},
		}
	}
	return AcceptancePlan{
		Criteria: []string{
			"Address the user request: " + truncateForDigest(text, 240),
			"Make necessary code or file changes (unless the request is analysis-only)",
			"Project builds (go build ./...) when go.mod is present",
			"Tests pass (go test ./...) when _test.go files exist",
		},
	}
}

func (a *Agent) resolveVerifyDir() string {
	if d := strings.TrimSpace(a.outputDir); d != "" {
		if abs, err := filepath.Abs(d); err == nil {
			return abs
		}
		return d
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func countSessionTools(sess *session.Session) (writes, reads, searches, lists int) {
	if sess == nil {
		return 0, 0, 0, 0
	}
	for _, e := range sess.EntriesCopy() {
		for _, tc := range e.AssistantToolCalls {
			switch tc.Name {
			case "write_file", "edit_file", "delete_file":
				writes++
			case "read_file":
				reads++
			case "search_code", "search_file":
				searches++
			case "list_dir":
				lists++
			}
		}
	}
	return writes, reads, searches, lists
}

func explorationOps(reads, searches, lists int) int {
	return reads + searches + lists
}

func looksLikeReadOnlyTask(description string) bool {
	lower := strings.ToLower(strings.TrimSpace(description))
	hints := []string{"analyze", "review", "read", "understand", "explore", "audit", "inspect", "分析", "审查", "阅读", "理解"}
	for _, h := range hints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

func defaultTaskContract(task Task) TaskContract {
	if IsVerificationTask(task) {
		return TaskContract{}
	}
	if looksLikeReadOnlyTask(task.Description) {
		return TaskContract{MinWriteOps: 0, MinReadOps: 2}
	}
	return TaskContract{MinWriteOps: 1, MinReadOps: 2}
}

func taskContract(task Task) TaskContract {
	if task.Contract != nil {
		return *task.Contract
	}
	return defaultTaskContract(task)
}

// validateSubTaskResult applies Level-3 success rules for a sub-agent run.
func validateSubTaskResult(task Task, sess *session.Session, _ string, strict bool) (bool, string) {
	if !strict {
		if sess == nil {
			return false, "empty session"
		}
		return true, ""
	}
	if IsVerificationTask(task) {
		return true, ""
	}

	contract := taskContract(task)
	writes, reads, searches, lists := countSessionTools(sess)
	explore := explorationOps(reads, searches, lists)
	total := writes + explore

	if total == 0 {
		return false, "no tool activity recorded"
	}
	if contract.MinWriteOps > 0 && writes < contract.MinWriteOps {
		return false, fmt.Sprintf("expected at least %d write/edit ops, got %d", contract.MinWriteOps, writes)
	}
	if contract.MinReadOps > 0 && explore < contract.MinReadOps {
		return false, fmt.Sprintf("expected at least %d exploration ops (read/search/list), got %d", contract.MinReadOps, explore)
	}

	return true, ""
}

func (a *Agent) runExitGate(ctx context.Context, msgCh chan<- tea.Msg, instruction string, results []TaskResult, nudges *int) (passed bool, continueLoop bool) {
	if !a.acceptanceStrict || !a.shouldRunExitGate() {
		return true, false
	}

	a.setState(StateVerifying)
	msgCh <- AgentStateMsg{State: StateVerifying}
	emitActivity(msgCh, "Working · checking acceptance criteria…")

	statuses, mech, allMet := a.runAcceptanceWithProgress(ctx, msgCh, instruction, results, verifyDirOrEmpty(a))
	plan := a.ensureAcceptancePlan(ctx, instruction)

	if allMet && mech.allOK() {
		return true, false
	}

	if nudges != nil {
		*nudges++
	}
	a.persistAcceptanceCheckpoint(plan, statuses, nudgeCount(nudges))
	gap := formatAcceptanceGaps(statuses, mech.allOK(), mech.Summary)
	report := formatAcceptanceReport(statuses, mech)
	a.session.AddWithState("system", "[VERIFICATION REPORT]\n"+report, StateVerifying.String(), 0)
	a.session.AddWithState("system", exitGateNudgePrefix+"\n\n"+gap, StateVerifying.String(), 0)
	msgCh <- StreamChunkMsg{Content: "Acceptance incomplete — see verification report in transcript; continuing work…", Done: true}
	return false, true
}

func nudgeCount(nudges *int) int {
	if nudges == nil {
		return 0
	}
	return *nudges
}

func (a *Agent) persistAcceptanceCheckpoint(plan AcceptancePlan, statuses []CriterionStatus, nudges int) {
	if a.cpStore == nil {
		return
	}
	cp, err := a.cpStore.Load()
	if err != nil || cp == nil {
		cp = &Checkpoint{Instruction: latestUserInstruction(a.session)}
	}
	if len(plan.Criteria) > 0 {
		cp.AcceptancePlan = plan
	}
	cp.CriteriaStatus = append([]CriterionStatus(nil), statuses...)
	cp.ExitGateNudges = nudges
	cp.AcceptanceIncomplete = len(statuses) > 0 && !allCriteriaMet(statuses)
	_ = a.cpStore.Save(cp)
}

func (a *Agent) restoreAcceptanceFromCheckpoint() int {
	if a.cpStore == nil {
		return 0
	}
	cp, err := a.cpStore.Load()
	if err != nil || cp == nil {
		return 0
	}
	if len(cp.AcceptancePlan.Criteria) > 0 {
		plan := cp.AcceptancePlan
		a.acceptancePlan = &plan
	}
	if cp.AcceptanceIncomplete && len(cp.CriteriaStatus) > 0 {
		gap := formatAcceptanceGaps(cp.CriteriaStatus, true, "")
		a.session.AddWithState("system", "[ACCEPTANCE RESUME]\n"+exitGateNudgePrefix+"\n\n"+gap, StateVerifying.String(), 0)
	}
	return cp.ExitGateNudges
}

func (a *Agent) shouldRunExitGate() bool {
	return true // caller gates on includeTools / intent
}

func verifyDirOrEmpty(a *Agent) string {
	return a.resolveVerifyDir()
}

// mechanicalVerifyResult splits workspace auto-checks from user-defined verify commands.
type mechanicalVerifyResult struct {
	WorkspaceOK bool
	BuildOK     bool
	TestOK      bool
	TestsRan    bool
	CustomOK    bool
	Summary     string
	VerifyDir   string
	Diagnosis   VerifyDiagnosis
}

func (m mechanicalVerifyResult) allOK() bool {
	return m.WorkspaceOK && m.CustomOK
}

func (a *Agent) runMechanicalVerify(ctx context.Context, dir string, plan AcceptancePlan) mechanicalVerifyResult {
	dir = strings.TrimSpace(dir)
	if dir == "" && len(plan.VerifyCommands) == 0 {
		return mechanicalVerifyResult{WorkspaceOK: true, CustomOK: true}
	}
	var parts []string
	res := mechanicalVerifyResult{WorkspaceOK: true, BuildOK: true, TestOK: true, CustomOK: true}
	if dir != "" {
		res.VerifyDir = dir
		wr := VerifyProjectWorkspaceDetailed(ctx, dir)
		if wr.Summary != "" {
			parts = append(parts, wr.Summary)
		}
		res.BuildOK = wr.BuildOK
		res.TestOK = wr.TestOK
		res.TestsRan = wr.TestsRan
		res.WorkspaceOK = wr.OK
	}
	if len(plan.VerifyCommands) > 0 && dir != "" {
		customSummary, customOK := RunVerifyCommands(ctx, dir, plan.VerifyCommands)
		if customSummary != "" {
			parts = append(parts, customSummary)
		}
		res.CustomOK = customOK
	} else if len(plan.VerifyCommands) > 0 {
		res.CustomOK = false
		parts = append(parts, "Custom verify skipped: no workspace directory")
	}
	res.Summary = strings.Join(parts, "\n")
	if !res.allOK() {
		res.Diagnosis = diagnoseMechanicalVerify(dir, res.BuildOK, res.TestOK, res.TestsRan, res.Summary)
	}
	if res.Summary != "" {
		a.session.AddWithState("system", "[VERIFICATION]\n"+res.Summary, StateVerifying.String(), 0)
	}
	return res
}

func (a *Agent) judgeAcceptance(ctx context.Context, instruction string, plan AcceptancePlan, results []TaskResult, mech mechanicalVerifyResult) ([]CriterionStatus, bool) {
	if len(plan.Criteria) == 0 {
		return nil, true
	}

	statuses, _ := judgeAcceptanceHeuristic(instruction, plan, a.session, results, mech)
	statuses = applyTestEvidencePass(statuses, plan, mech, a.session)
	statuses = applyMechanicalCrossCheck(statuses, plan, mech, a.session)
	source := "heuristic+test_evidence"

	if allCriteriaMet(statuses) {
		a.logRun("acceptance_decision", "source=%s all_met=true buildOK=%v testOK=%v", source, mech.BuildOK, mech.TestOK)
		return statuses, true
	}

	// Green mechanical tests: trust test evidence; skip LLM judge (saves tokens and avoids false negatives).
	if mech.TestOK && mech.TestsRan && mech.BuildOK && mechanicalTestsSatisfyAcceptance(statuses, plan, mech, a.session) {
		statuses = applyTestEvidencePass(statuses, plan, mech, a.session)
		statuses = applyMechanicalCrossCheck(statuses, plan, mech, a.session)
		a.logRun("acceptance_decision", "source=test_evidence_skip_llm all_met=true")
		return statuses, true
	}

	if a.provider != nil && a.acceptanceStrict && a.pipelineOpts.UseLLMAcceptance {
		llmStatuses, _, err := judgeAcceptanceLLM(ctx, a, instruction, plan, results)
		if err == nil {
			llmStatuses = applyTestEvidencePass(llmStatuses, plan, mech, a.session)
			llmStatuses = applyMechanicalCrossCheck(llmStatuses, plan, mech, a.session)
			source = "llm+test_evidence"
			allMet := allCriteriaMet(llmStatuses)
			a.logRun("acceptance_decision", "source=%s all_met=%v buildOK=%v testOK=%v", source, allMet, mech.BuildOK, mech.TestOK)
			return llmStatuses, allMet
		}
		a.logRun("acceptance_decision", "source=llm_error err=%v fallback=heuristic", err)
		// Strict + LLM judge failed: fall back to heuristic+test evidence instead of hard fail.
		allMet := allCriteriaMet(statuses)
		a.logRun("acceptance_decision", "source=%s all_met=%v (llm unavailable)", source, allMet)
		return statuses, allMet
	}

	allMet := allCriteriaMet(statuses)
	a.logRun("acceptance_decision", "source=%s all_met=%v buildOK=%v testOK=%v", source, allMet, mech.BuildOK, mech.TestOK)
	return statuses, allMet
}

func applyMechanicalCrossCheck(statuses []CriterionStatus, plan AcceptancePlan, mech mechanicalVerifyResult, sess *session.Session) []CriterionStatus {
	writes, reads, searches, lists := countSessionTools(sess)
	explore := explorationOps(reads, searches, lists)
	instruction := latestUserInstruction(sess)
	readOnly := looksLikeReviewInstruction(instruction) || looksLikeReadOnlyTask(instruction)
	for i := range statuses {
		if !statuses[i].Met {
			continue
		}
		lower := strings.ToLower(statuses[i].Text)
		if isBuildTestCriterion(lower) {
			if strings.Contains(lower, "test") || strings.Contains(lower, "测试") {
				if mech.TestsRan && !mech.TestOK {
					statuses[i].Met = false
					ev := "cross-check: go test ./... did not pass"
					if mech.Diagnosis.Kind != VerifyFailureNone {
						ev += " — likely " + mech.Diagnosis.Kind.Label()
					}
					statuses[i].Evidence = ev
				}
				continue
			}
			if !mech.BuildOK {
				statuses[i].Met = false
				statuses[i].Evidence = "cross-check: go build ./... did not pass"
			}
			continue
		}
		if isFunctionalCriterion(lower) {
			if readOnly {
				if explore < 2 {
					statuses[i].Met = false
					statuses[i].Evidence = fmt.Sprintf("cross-check: need exploration>=2 (got %d)", explore)
				}
				continue
			}
			if mech.TestOK && mech.TestsRan && mech.BuildOK && writes >= 1 {
				continue // tests passed — behavioral evidence sufficient
			}
			if len(plan.VerifyCommands) > 0 {
				if !mech.CustomOK {
					statuses[i].Met = false
					statuses[i].Evidence = "cross-check: custom verify command(s) did not pass"
				}
			} else if writes < 1 || explore < 2 {
				statuses[i].Met = false
				statuses[i].Evidence = fmt.Sprintf("cross-check: need writes>=1 and exploration>=2 (got %d writes, %d explore)", writes, explore)
			}
		}
	}
	return statuses
}

func isBuildTestCriterion(lower string) bool {
	return strings.Contains(lower, "build") || strings.Contains(lower, "test") ||
		strings.Contains(lower, "编译") || strings.Contains(lower, "测试")
}

func isFunctionalCriterion(lower string) bool {
	if isBuildTestCriterion(lower) {
		return false
	}
	hints := []string{
		"address the user", "user request", "function", "feature", "implement",
		"file changes", "需求", "功能", "实现",
	}
	for _, h := range hints {
		if strings.Contains(lower, h) {
			return true
		}
	}
	return false
}

func unmetCriteriaStatuses(plan AcceptancePlan, evidence string) []CriterionStatus {
	statuses := make([]CriterionStatus, len(plan.Criteria))
	for i, c := range plan.Criteria {
		statuses[i] = CriterionStatus{Index: i, Text: c, Met: false, Evidence: evidence}
	}
	return statuses
}

func judgeAcceptanceLLM(ctx context.Context, a *Agent, instruction string, plan AcceptancePlan, results []TaskResult) ([]CriterionStatus, bool, error) {
	criteriaJSON, _ := json.Marshal(plan.Criteria)
	digest := buildSessionDigest(a.session, results)
	prompt := fmt.Sprintf(`Judge whether each acceptance criterion is satisfied.

User request:
%s

Criteria (JSON array):
%s

Session activity:
%s

Output ONLY JSON array:
[{"index":0,"met":true,"evidence":"..."}]`, instruction, string(criteriaJSON), digest)

	messages := []llm.Message{
		{Role: "system", Content: "You are an independent acceptance judge. Be strict — met=true only with concrete evidence. Output only JSON."},
		{Role: "user", Content: prompt},
	}
	resp, err := stream.RetryChat(ctx, a.provider, &llm.Request{Messages: messages}, a.retryConfig)
	if err != nil {
		return nil, false, err
	}
	var raw []struct {
		Index    int    `json:"index"`
		Met      bool   `json:"met"`
		Evidence string `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(cleanJSON(resp.Content)), &raw); err != nil {
		return nil, false, err
	}
	statuses := make([]CriterionStatus, len(plan.Criteria))
	allMet := len(plan.Criteria) > 0
	for i, c := range plan.Criteria {
		statuses[i] = CriterionStatus{Index: i, Text: c, Met: false}
	}
	for _, r := range raw {
		if r.Index < 0 || r.Index >= len(statuses) {
			continue
		}
		statuses[r.Index].Met = r.Met
		statuses[r.Index].Evidence = strings.TrimSpace(r.Evidence)
	}
	for _, s := range statuses {
		if !s.Met {
			allMet = false
		}
	}
	return statuses, allMet, nil
}

func judgeAcceptanceHeuristic(instruction string, plan AcceptancePlan, sess *session.Session, results []TaskResult, mech mechanicalVerifyResult) ([]CriterionStatus, bool) {
	writes, reads, searches, lists := countSessionTools(sess)
	explore := explorationOps(reads, searches, lists)
	digest := strings.ToLower(buildSessionDigest(sess, results) + " " + instruction)

	statuses := make([]CriterionStatus, len(plan.Criteria))
	allMet := true
	for i, c := range plan.Criteria {
		met, ev := heuristicCriterionMet(c, digest, writes, explore, mech)
		skipped := criterionEvidenceSkipped(ev)
		statuses[i] = CriterionStatus{Index: i, Text: c, Met: met || skipped, Skipped: skipped, Evidence: ev}
		if !met {
			allMet = false
		}
	}
	return statuses, allMet
}

func heuristicCriterionMet(criterion, digest string, writes, explore int, mech mechanicalVerifyResult) (bool, string) {
	lower := strings.ToLower(criterion)
	switch {
	case strings.Contains(lower, "address the user request"):
		if writes < 1 {
			return false, "need at least one write/edit"
		}
		if explore < 2 {
			return false, fmt.Sprintf("need exploration (>=2 read/search/list, got %d)", explore)
		}
		return true, fmt.Sprintf("%d writes, %d exploration ops", writes, explore)
	case strings.Contains(lower, "make necessary code") || strings.Contains(lower, "file changes"):
		if writes < 1 {
			return false, "no file changes recorded"
		}
		return true, fmt.Sprintf("%d write/edit ops", writes)
	case strings.Contains(lower, "build") || strings.Contains(lower, "编译"):
		if mech.BuildOK {
			return true, "go build ./... passed"
		}
		return false, "go build ./... did not pass — see mechanical verify output"
	case strings.Contains(lower, "test") || strings.Contains(lower, "测试"):
		if !mech.TestsRan {
			return true, "skipped — no _test.go files (N/A)"
		}
		if mech.TestOK {
			return true, "go test ./... passed"
		}
		ev := "go test ./... failed"
		if mech.BuildOK {
			ev += " — build passed; inspect whether failure is tests vs application logic"
		}
		if mech.Diagnosis.Kind == VerifyFailureTestHarness {
			ev += " — likely test expectations, not runtime behavior"
		} else if mech.Diagnosis.Kind == VerifyFailurePath {
			ev += " — check workspace path/module before editing code"
		} else if mech.Diagnosis.Kind == VerifyFailureEnvironment {
			ev += " — check toolchain/deps before editing code"
		}
		return false, ev
	case strings.Contains(lower, "analysis-only") || strings.Contains(lower, "分析"):
		if explore >= 2 {
			return true, fmt.Sprintf("%d exploration ops", explore)
		}
		return explore > 0, "read/search/list activity"
	case strings.Contains(lower, "conclusion") || strings.Contains(lower, "结论"):
		if hasSubstantialConclusionText(latestAssistantFromDigest(digest)) {
			return true, "structured conclusion present"
		}
		return false, "missing structured analysis conclusion"
	default:
		if writes < 1 {
			return false, "insufficient file changes"
		}
		if explore < 1 {
			return false, "insufficient exploration"
		}
		return true, fmt.Sprintf("%d writes, %d exploration ops", writes, explore)
	}
}

func latestAssistantFromDigest(digest string) string {
	// digest may embed assistant lines from buildSessionDigest
	if idx := strings.LastIndex(digest, "assistant:"); idx >= 0 {
		return digest[idx+len("assistant:"):]
	}
	return digest
}

func allCriteriaMet(statuses []CriterionStatus) bool {
	if len(statuses) == 0 {
		return true
	}
	for _, s := range statuses {
		if !s.Met {
			return false
		}
	}
	return true
}

func appendMechanicalStatus(statuses []CriterionStatus, mechOK bool, summary string) []CriterionStatus {
	ev := "build/test checks passed"
	if !mechOK {
		ev = truncateForDigest(summary, 200)
	}
	return append(statuses, CriterionStatus{
		Index:    len(statuses),
		Text:     "Mechanical verify (go build/test)",
		Met:      mechOK,
		Evidence: ev,
	})
}

func formatAcceptanceReport(statuses []CriterionStatus, mech mechanicalVerifyResult) string {
	var b strings.Builder
	b.WriteString("── Acceptance verification ──\n")
	if len(statuses) == 0 {
		b.WriteString("(no criteria recorded)\n")
	} else {
		passed := 0
		for _, s := range statuses {
			mark := "FAIL"
			if s.Met {
				mark = "PASS"
				passed++
			}
			b.WriteString(fmt.Sprintf("[%s] %s\n", mark, s.Text))
			if ev := strings.TrimSpace(s.Evidence); ev != "" {
				b.WriteString(fmt.Sprintf("      reason: %s\n", ev))
			}
		}
		b.WriteString(fmt.Sprintf("Summary: %d/%d criteria met\n", passed, len(statuses)))
	}
	if mech.Summary != "" {
		b.WriteString("\n── Mechanical checks ──\n")
		b.WriteString(mech.Summary)
		if !mech.allOK() {
			b.WriteString("\n      → build/test or custom verify did not pass")
		}
		b.WriteString("\n")
	} else if !mech.allOK() {
		b.WriteString("\n── Mechanical checks ──\n")
		b.WriteString("FAIL — workspace or custom verify commands did not pass\n")
	}
	if !mech.allOK() {
		if block := formatDiagnosisBlock(mech.Diagnosis, mech.VerifyDir); block != "" {
			b.WriteString("\n")
			b.WriteString(block)
			b.WriteString("\n")
		} else if mech.Summary != "" {
			d := diagnoseMechanicalVerify(mech.VerifyDir, mech.BuildOK, mech.TestOK, mech.TestsRan, mech.Summary)
			if block := formatDiagnosisBlock(d, mech.VerifyDir); block != "" {
				b.WriteString("\n")
				b.WriteString(block)
				b.WriteString("\n")
			}
		}
	}
	if (len(statuses) > 0 && !allCriteriaMet(statuses)) || !mech.allOK() {
		b.WriteString("\n── Action required ──\n")
		b.WriteString(formatAcceptanceGaps(statuses, mech.allOK(), mech.Summary))
	}
	return strings.TrimSpace(b.String())
}

func criterionEvidenceSkipped(evidence string) bool {
	lower := strings.ToLower(strings.TrimSpace(evidence))
	return strings.Contains(lower, "skipped") ||
		strings.Contains(lower, "n/a") ||
		strings.Contains(evidence, "不适用")
}

func formatAcceptanceGaps(statuses []CriterionStatus, mechOK bool, mechSummary string) string {
	var b strings.Builder
	for _, s := range statuses {
		if s.Met {
			continue
		}
		b.WriteString(fmt.Sprintf("- [ ] %s", s.Text))
		if s.Evidence != "" {
			b.WriteString(fmt.Sprintf(" (%s)", s.Evidence))
		}
		b.WriteString("\n")
	}
	if !mechOK && mechSummary != "" {
		b.WriteString("- [ ] Mechanical verify failed:\n")
		b.WriteString(truncateForDigest(mechSummary, 600))
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func (a *Agent) runFinalAcceptanceGate(ctx context.Context, msgCh chan<- tea.Msg, instruction string, results []TaskResult) ([]CriterionStatus, bool) {
	if !a.acceptanceStrict {
		return nil, true
	}
	a.setState(StateVerifying)
	msgCh <- AgentStateMsg{State: StateVerifying}
	emitActivity(msgCh, "Working · final acceptance verification…")

	statuses, mech, allMet := a.runAcceptanceWithProgress(ctx, msgCh, instruction, results, a.resolveVerifyDir())
	return statuses, allMet && mech.allOK()
}

func (a *Agent) runAcceptanceWithProgress(ctx context.Context, msgCh chan<- tea.Msg, instruction string, results []TaskResult, verifyDir string) ([]CriterionStatus, mechanicalVerifyResult, bool) {
	plan := a.ensureAcceptancePlan(ctx, instruction)
	a.emitChecklistInit(msgCh, plan)

	mech := a.runMechanicalVerify(ctx, verifyDir, plan)
	statuses, _ := a.judgeAcceptance(ctx, instruction, plan, results, mech)
	if !mech.allOK() {
		statuses = appendMechanicalStatus(statuses, mech.allOK(), mech.Summary)
		if len(statuses) > len(plan.Criteria) {
			msgCh <- VerificationProgressMsg{AppendText: statuses[len(statuses)-1].Text}
		}
	}

	for i := range statuses {
		a.emitCriterionChecked(msgCh, statuses, i, mech)
		select {
		case <-ctx.Done():
			return statuses, mech, false
		case <-time.After(200 * time.Millisecond):
		}
	}

	allMet := allCriteriaMet(statuses) && mech.allOK()
	a.pendingMech = mech
	writes, reads, searches, lists := countSessionTools(a.session)
	a.logRun("verify", "mechanical dir=%s buildOK=%v testOK=%v testsRan=%v customOK=%v allOK=%v tools writes=%d reads=%d search=%d list=%d criteria=%d/%d",
		mech.VerifyDir, mech.BuildOK, mech.TestOK, mech.TestsRan, mech.CustomOK, mech.allOK(),
		writes, reads, searches, lists, countCriteriaMet(statuses), len(statuses))
	if mech.Diagnosis.Kind != VerifyFailureNone {
		a.logRun("verify", "diagnosis kind=%s summary=%s", mech.Diagnosis.Kind.Label(), truncateForDigest(mech.Diagnosis.Summary, 120))
	}
	a.emitVerificationFinal(msgCh, statuses, mech, allMet)
	return statuses, mech, allMet
}

func (a *Agent) injectAcceptanceRecovery(statuses []CriterionStatus, instruction string, mech mechanicalVerifyResult) {
	gap := formatAcceptanceGaps(statuses, mech.allOK(), mech.Summary)
	if gap == "" {
		gap = "- [ ] Complete the user request: " + truncateForDigest(instruction, 200)
	}
	body := acceptanceRecoveryPrefix
	if block := formatRecoveryDiagnosis(mech); block != "" {
		body += "\n\n" + block
	}
	body += "\n\n" + gap
	a.session.AddWithState("system", body, StateVerifying.String(), 0)
}

func countCriteriaMet(statuses []CriterionStatus) int {
	n := 0
	for _, s := range statuses {
		if s.Met {
			n++
		}
	}
	return n
}

func (a *Agent) clearAcceptanceIncompleteCheckpoint() {
	if a.cpStore == nil {
		return
	}
	cp, err := a.cpStore.Load()
	if err != nil || cp == nil {
		return
	}
	cp.AcceptanceIncomplete = false
	cp.ExitGateNudges = 0
	_ = a.cpStore.Save(cp)
}

func (a *Agent) emitChecklistInit(msgCh chan<- tea.Msg, plan AcceptancePlan) {
	if msgCh == nil || len(plan.Criteria) == 0 {
		return
	}
	statuses := make([]CriterionStatus, len(plan.Criteria))
	for i, c := range plan.Criteria {
		statuses[i] = CriterionStatus{Index: i, Text: c, Met: false}
	}
	msgCh <- VerificationProgressMsg{
		InitChecklist: true,
		Criteria:      statuses,
		Total:         len(statuses),
	}
}

func (a *Agent) emitCriterionChecked(msgCh chan<- tea.Msg, statuses []CriterionStatus, index int, mech mechanicalVerifyResult) {
	if msgCh == nil || index < 0 || index >= len(statuses) {
		return
	}
	passed := countCriteriaMet(statuses)
	msgCh <- VerificationProgressMsg{
		CheckedIndex: index,
		Criteria:     append([]CriterionStatus(nil), statuses...),
		Passed:       passed,
		Total:        len(statuses),
		AllMet:       allCriteriaMet(statuses) && mech.allOK(),
	}
}

func (a *Agent) emitVerificationFinal(msgCh chan<- tea.Msg, statuses []CriterionStatus, mech mechanicalVerifyResult, allMet bool) {
	if msgCh == nil {
		return
	}
	passed := countCriteriaMet(statuses)
	msgCh <- VerificationProgressMsg{
		Passed:   passed,
		Total:    len(statuses),
		Criteria: append([]CriterionStatus(nil), statuses...),
		AllMet:   allMet,
		Finalize: true,
	}
}

func (a *Agent) emitVerificationProgress(msgCh chan<- tea.Msg, statuses []CriterionStatus, mech mechanicalVerifyResult) {
	passed := 0
	for _, s := range statuses {
		if s.Met {
			passed++
		}
	}
	allMet := allCriteriaMet(statuses) && mech.allOK()
	detail := formatAcceptanceReport(statuses, mech)
	msgCh <- VerificationProgressMsg{
		Passed:   passed,
		Total:    len(statuses),
		Criteria: append([]CriterionStatus(nil), statuses...),
		Detail:   detail,
		AllMet:   allMet,
	}
}

func auditSubTaskResults(results []TaskResult) bool {
	for _, r := range results {
		if !r.Success {
			return false
		}
	}
	return len(results) > 0
}

func attachTaskContracts(tasks []Task) []Task {
	out := make([]Task, len(tasks))
	copy(out, tasks)
	for i := range out {
		if IsVerificationTask(out[i]) {
			continue
		}
		c := defaultTaskContract(out[i])
		out[i].Contract = &c
	}
	return out
}
