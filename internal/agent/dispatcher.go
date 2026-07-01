package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

// TaskContract defines programmatic success checks for a sub-task.
type TaskContract struct {
	MinWriteOps int `json:"min_write_ops,omitempty"`
	MinReadOps  int `json:"min_read_ops,omitempty"`
}

// Task represents a unit of work that may depend on other tasks.
type Task struct {
	ID          string        `json:"id"`
	Description string        `json:"description"`
	DependsOn   []string      `json:"depends_on,omitempty"`
	Contract    *TaskContract `json:"contract,omitempty"`
}

// TaskResult captures the outcome of a single sub-task execution.
type TaskResult struct {
	TaskID  string `json:"task_id"`
	Success bool   `json:"success"`
	Content string `json:"content"`
	Error   string `json:"error,omitempty"`
}

// SubtaskMsg carries a single subtask status update to the TUI.
type SubtaskMsg struct {
	TaskID string
	Status string // "running", "done", "error"
	Label  string
}

// ResolveConflictMsg asks the user what to do when a checkpoint is found.
type ResolveConflictMsg struct {
	HasInProgress bool
	LastPhase     string
}

// TaskDispatcher decomposes complex instructions into parallel sub-tasks
// and orchestrates their execution via SubAgents with checkpoint support.
type TaskDispatcher struct {
	agent              *Agent
	maxParallel        int
	subAgentTimeout    time.Duration
	subAgentMaxTurns   int
	subAgentMaxRetries int
	checkpointStore    *CheckpointStore
	skipPlanConfirm    bool
}

// DispatcherOptions configures parallel sub-agent execution.
type DispatcherOptions struct {
	MaxParallel        int
	SubAgentTimeout    time.Duration
	SubAgentMaxTurns   int
	SubAgentMaxRetries int
	SkipPlanConfirm    bool // tests/headless auto-confirm via message handler
}

// DefaultDispatcherOptions returns built-in defaults when no config is supplied.
func DefaultDispatcherOptions() DispatcherOptions {
	return DispatcherOptions{
		MaxParallel:        4,
		SubAgentTimeout:    180 * time.Second,
		SubAgentMaxTurns:   50,
		SubAgentMaxRetries: 0,
	}
}

// NewTaskDispatcher creates a dispatcher bound to the parent agent.
func NewTaskDispatcher(agent *Agent, opts DispatcherOptions) *TaskDispatcher {
	if opts.MaxParallel < 1 {
		opts.MaxParallel = DefaultDispatcherOptions().MaxParallel
	}
	if opts.SubAgentTimeout <= 0 {
		opts.SubAgentTimeout = DefaultDispatcherOptions().SubAgentTimeout
	}
	if opts.SubAgentMaxTurns < 0 {
		opts.SubAgentMaxTurns = 0 // unlimited
	}
	return &TaskDispatcher{
		agent:              agent,
		maxParallel:        opts.MaxParallel,
		subAgentTimeout:    opts.SubAgentTimeout,
		subAgentMaxTurns:   opts.SubAgentMaxTurns,
		subAgentMaxRetries: opts.SubAgentMaxRetries,
		skipPlanConfirm:    opts.SkipPlanConfirm,
	}
}

// SetCheckpointStore attaches a checkpoint store for resume/rollback.
func (d *TaskDispatcher) SetCheckpointStore(cs *CheckpointStore) {
	d.checkpointStore = cs
}

// Dispatch decomposes the instruction and executes tasks with checkpoint support.
// Always decomposes (simple tasks yield a single-task plan).
func (d *TaskDispatcher) Dispatch(ctx context.Context, instruction string, msgCh chan<- tea.Msg) (DispatchOutcome, error) {
	// Check for existing checkpoint (resume scenario)
	cp, _ := d.checkpointLoad()
	if cp != nil {
		d.agent.mergeCheckpointTasks(cp)
		if len(cp.Tasks) > 0 {
			d.checkpointSave(cp)
		}
	}
	if cp != nil && cp.Phase != CheckpointDone && cp.Interrupted {
		mode := d.agent.consumeFollowUpMode()
		if mode == FollowUpUnknown {
			mode = FollowUpContinue
		}
		cp.Interrupted = false
		d.checkpointSave(cp)
		if len(cp.Tasks) > 0 {
			instr := cp.Instruction
			if strings.TrimSpace(instr) == "" {
				instr = instruction
			}
			switch mode {
			case FollowUpContinue:
				msgCh <- StreamChunkMsg{
					Content: fmt.Sprintf("Resuming interrupted session — %d/%d tasks completed.",
						len(cp.Results), len(cp.Tasks)),
					Done: true,
				}
				return d.dispatchWithCheckpoint(ctx, instr, cp, msgCh)
			case FollowUpReplanLight, FollowUpReplanFull:
				return d.dispatchWithReplan(ctx, instr, cp, mode, msgCh)
			}
		}
		d.checkpointClear()
		cp = nil
	}
	if cp != nil && cp.Phase != CheckpointDone {
		reply := make(chan CheckpointResponse, 1)
		msgCh <- CheckpointPromptMsg{
			Phase:                string(cp.Phase),
			Completed:            len(cp.Results),
			Total:                len(cp.Tasks),
			AcceptanceIncomplete: cp.AcceptanceIncomplete,
			Reply:                reply,
		}
		var resume bool
		select {
		case resp := <-reply:
			resume = resp.Resume
		case <-ctx.Done():
			return OutcomeNotHandled, ctx.Err()
		}
		if resume {
			if len(cp.Tasks) > 0 {
				return d.dispatchWithCheckpoint(ctx, instruction, cp, msgCh)
			}
			msgCh <- StreamChunkMsg{
				Content: "Checkpoint has no tasks — starting fresh decomposition.",
				Done:    true,
			}
		}
		d.checkpointClear()
	}

	acceptancePlan := d.agent.ensureAcceptancePlan(ctx, instruction)

	// Fresh start: decompose then execute
	tasks, err := d.Plan(ctx, instruction)
	if err != nil {
		msgCh <- StreamChunkMsg{
			Content: fmt.Sprintf("Decomposition failed (%v), falling back to sequential execution.", err),
			Done:    true,
		}
		return OutcomeNotHandled, err
	}
	tasks = attachTaskContracts(tasks)

	cp = &Checkpoint{
		Phase:          CheckpointDecomposed,
		Tasks:          tasks,
		Instruction:    instruction,
		AcceptancePlan: acceptancePlan,
	}
	d.checkpointSave(cp)

	msgCh <- TaskPlanMsg{Tasks: tasksToPlanItems(tasks)}

	if len(tasks) > 1 {
		confirmed, err := d.waitForPlanConfirm(ctx, msgCh, len(tasks))
		if err != nil {
			return OutcomeNotHandled, err
		}
		if !confirmed {
			d.checkpointClear()
			msgCh <- StreamChunkMsg{Content: "Task plan cancelled.", Done: true}
			return OutcomeCancelled, nil
		}
	}

	implTasks := implementationTasks(tasks)
	verifyTask := findVerificationTask(tasks)

	cp.Phase = CheckpointExecuting
	d.checkpointSave(cp)

	// One implementation task + auto verify: main agent implements, then verify-fix loop.
	if len(implTasks) == 1 && verifyTask != nil {
		msgCh <- StreamChunkMsg{Content: "Executing task, then build/test verification.", Done: true}
		msgCh <- SubtaskMsg{TaskID: implTasks[0].ID, Status: "running", Label: implTasks[0].Description}
		if err := d.agent.agentLoop(ctx, msgCh, true); err != nil {
			return OutcomeNotHandled, err
		}
		if d.agent.state == StateError {
			d.signalDispatchFailed(ctx, msgCh, instruction, nil, cp.Results, "implementation failed")
			d.checkpointClearUnlessResumable(nil)
			return OutcomeFailed, nil
		}
		if ok, why := validateSubTaskResult(implTasks[0], d.agent.session, d.agent.resolveVerifyDir(), d.agent.acceptanceStrict); !ok {
			d.agent.signalTaskFailed(ctx, msgCh, fmt.Sprintf("task %s: %s", implTasks[0].ID, why), nil, nil)
			d.checkpointClearUnlessResumable(nil)
			return OutcomeFailed, nil
		}
		msgCh <- SubtaskMsg{TaskID: implTasks[0].ID, Status: "done", Label: implTasks[0].Description}
		result := d.runVerificationTask(ctx, *verifyTask, msgCh)
		projectDir := d.agent.resolveVerifyDir()
		if result.Success {
			statuses, accepted := d.agent.driveUntilAccepted(ctx, msgCh, instruction, true, []TaskResult{result})
			cp.CriteriaStatus = statuses
			d.checkpointSave(cp)
			if accepted {
				d.agent.signalTaskComplete(ctx, msgCh, true, 1, projectDir, []TaskResult{result}, statuses)
				d.checkpointClear()
				return OutcomeSuccess, nil
			}
			d.agent.signalTaskFailed(ctx, msgCh, "could not complete task after autonomous recovery", statuses, []TaskResult{result})
			d.checkpointClearUnlessResumable(statuses)
			return OutcomeFailed, nil
		}
		d.agent.signalTaskFailed(ctx, msgCh, result.Error, nil, []TaskResult{result})
		d.checkpointClearUnlessResumable(nil)
		return OutcomeFailed, nil
	}

	err = d.executeTasksWithCheckpoint(ctx, cp, msgCh)
	if err != nil {
		return OutcomeNotHandled, err
	}
	outcome := d.mergeResults(ctx, instruction, cp, msgCh)
	if outcome == OutcomeSuccess {
		d.checkpointClear()
	}
	return outcome, nil
}

// Plan decomposes an instruction into sub-tasks using the LLM (default).
// Mode 0/1: LLM decides whether to return one task or many. Mode 2: skip LLM, single task.
func (d *TaskDispatcher) Plan(ctx context.Context, instruction string) ([]Task, error) {
	return d.planTasks(ctx, instruction, "")
}

func (d *TaskDispatcher) planAfterInterrupt(ctx context.Context, instruction string, cp *Checkpoint, mode FollowUpMode) ([]Task, error) {
	progress := formatCheckpointProgress(cp)
	var extra string
	switch mode {
	case FollowUpReplanFull:
		extra = fmt.Sprintf(`

The user interrupted and may have changed direction or asked for a new plan.
Prior progress (context only — may be obsolete):
%s

Produce a fresh task breakdown for the FULL merged request below.`, progress)
	default:
		extra = fmt.Sprintf(`

The session was interrupted. Progress so far:
%s

Plan remaining or adjusted work only. Do NOT recreate finished sub-tasks. Use new task IDs for newly added work.`, progress)
	}
	return d.planTasks(ctx, instruction, extra)
}

func (d *TaskDispatcher) planTasks(ctx context.Context, instruction, extraContext string) ([]Task, error) {
	if d.agent.pipelineOpts.PlanMode == 2 {
		return ensureVerificationTask([]Task{{ID: "1", Description: instruction}}, instruction), nil
	}

	layoutHint := ""
	switch d.agent.ProjectLayout() {
	case LayoutDDD:
		layoutHint = "\n- Project uses DDD layered architecture; split tasks by layer when sensible"
	case LayoutMinimal:
		layoutHint = "\n- Project is minimal scope; prefer exactly ONE task for small programs (snake game, calculator, CLI). Only split when clearly separable."
	}

	prompt := fmt.Sprintf(`You are a task planner. Analyze the request and decide how many sub-tasks are needed.

Rules:
- YOU decide: return ONE task when the work is small/cohesive; split into multiple tasks only when clearly separable or parallelizable
- Simple requests (single bug fix, one file, small feature) → exactly ONE task; put the full user request in description
- Complex requests (multiple layers, frontend+backend, independent modules) → multiple tasks with depends_on where order matters
- Write every task description in English
- Each sub-task must be self-contained (no shared mutable state)
- Tasks that can run in parallel must NOT list each other as dependencies
- Output ONLY valid JSON — an array of objects with id, description, and optional depends_on
- Keep each description concise (one sentence)
- Do NOT add a final verification task — one is appended automatically after planning%s%s

Request: %s

Output format (one task example):
[{"id": "1", "description": "implement X end-to-end"}]

Multi-task example:
[{"id": "1", "description": "do X"}, {"id": "2", "description": "do Y", "depends_on": ["1"]}]`, layoutHint, extraContext, instruction)

	messages := []llm.Message{
		{Role: "system", Content: "You are a task planner. Output only valid JSON arrays. Prefer a single task unless splitting clearly helps. No markdown, no explanation."},
		{Role: "user", Content: prompt},
	}

	resp, err := stream.RetryChat(ctx, d.agent.provider, &llm.Request{Messages: messages}, d.agent.retryConfig)
	if err != nil {
		return nil, err
	}

	jsonStr := cleanJSON(resp.Content)
	var tasks []Task
	if err := json.Unmarshal([]byte(jsonStr), &tasks); err != nil {
		return nil, fmt.Errorf("task plan parse error: %w", err)
	}

	if len(tasks) == 0 {
		tasks = []Task{{ID: "1", Description: instruction}}
	}
	return ensureVerificationTask(tasks, instruction), nil
}

// dispatchWithReplan runs the planner after interrupt follow-up, preserving completed work when appropriate.
func (d *TaskDispatcher) dispatchWithReplan(ctx context.Context, instruction string, cp *Checkpoint, mode FollowUpMode, msgCh chan<- tea.Msg) (DispatchOutcome, error) {
	if len(cp.AcceptancePlan.Criteria) > 0 {
		d.agent.acceptancePlan = &cp.AcceptancePlan
	}

	msgCh <- StreamChunkMsg{Content: followUpModeLabel(mode), Done: true}

	tasks, err := d.planAfterInterrupt(ctx, instruction, cp, mode)
	if err != nil {
		msgCh <- StreamChunkMsg{
			Content: fmt.Sprintf("Re-plan failed (%v), resuming previous task list.", err),
			Done:    true,
		}
		return d.dispatchWithCheckpoint(ctx, instruction, cp, msgCh)
	}
	tasks = attachTaskContracts(tasks)

	switch mode {
	case FollowUpReplanFull:
		cp.Results = nil
	default:
		cp.Results = filterResultsForTasks(cp.Results, tasks)
	}
	cp.Tasks = tasks
	cp.Instruction = instruction
	cp.Phase = CheckpointDecomposed
	d.checkpointSave(cp)

	msgCh <- TaskPlanMsg{Tasks: tasksToPlanItems(tasks)}
	emitCheckpointTaskStates(cp, msgCh)

	if len(tasks) > 1 {
		confirmed, err := d.waitForPlanConfirm(ctx, msgCh, len(tasks))
		if err != nil {
			return OutcomeNotHandled, err
		}
		if !confirmed {
			d.checkpointClear()
			msgCh <- StreamChunkMsg{Content: "Task plan cancelled.", Done: true}
			return OutcomeCancelled, nil
		}
	}

	cp.Phase = CheckpointExecuting
	d.checkpointSave(cp)

	if err := d.executeTasksWithCheckpoint(ctx, cp, msgCh); err != nil {
		return OutcomeNotHandled, err
	}
	outcome := d.mergeResults(ctx, instruction, cp, msgCh)
	if outcome == OutcomeSuccess {
		d.checkpointClear()
	}
	return outcome, nil
}

// dispatchWithCheckpoint handles resume after a checkpoint is found.
func (d *TaskDispatcher) dispatchWithCheckpoint(ctx context.Context, instruction string, cp *Checkpoint, msgCh chan<- tea.Msg) (DispatchOutcome, error) {
	if cp == nil || len(cp.Tasks) == 0 {
		d.checkpointClear()
		msgCh <- StreamChunkMsg{Content: "Checkpoint has no tasks — starting fresh decomposition.", Done: true}
		return OutcomeNotHandled, nil
	}
	if cp.Phase == CheckpointDone {
		d.checkpointClear()
		msgCh <- StreamChunkMsg{Content: "Previous session completed. Starting fresh.", Done: true}
		return OutcomeNotHandled, nil
	}

	if len(cp.AcceptancePlan.Criteria) > 0 {
		d.agent.acceptancePlan = &cp.AcceptancePlan
	}

	if cp.AcceptanceIncomplete && len(cp.CriteriaStatus) > 0 {
		gap := formatAcceptanceGaps(cp.CriteriaStatus, true, "")
		d.agent.session.AddWithState("system", "[ACCEPTANCE RESUME]\n"+exitGateNudgePrefix+"\n\n"+gap, StateVerifying.String(), 0)
	}

	msgCh <- TaskPlanMsg{Tasks: tasksToPlanItems(cp.Tasks)}
	emitCheckpointTaskStates(cp, msgCh)

	msgCh <- StreamChunkMsg{
		Content: fmt.Sprintf("Resuming from checkpoint — %d/%d tasks completed. Remaining: %d",
			len(cp.Results), len(cp.Tasks), len(cp.Tasks)-len(cp.Results)),
		Done: true,
	}

	err := d.executeTasksWithCheckpoint(ctx, cp, msgCh)
	if err != nil {
		return OutcomeNotHandled, err
	}
	outcome := d.mergeResults(ctx, instruction, cp, msgCh)
	if outcome == OutcomeSuccess {
		d.checkpointClear()
	}
	return outcome, nil
}

// executeTasksWithCheckpoint filters already-completed tasks and runs pending ones.
func (d *TaskDispatcher) executeTasksWithCheckpoint(ctx context.Context, cp *Checkpoint, msgCh chan<- tea.Msg) error {
	completed := cp.CompletedTaskIDs()

	if len(completed) >= len(cp.Tasks) {
		cp.Phase = CheckpointDone
		d.checkpointSave(cp)
		return nil
	}

	cp.Phase = CheckpointExecuting
	d.checkpointSave(cp)

	newResults := d.executeTasks(ctx, cp.Tasks, completed, msgCh)
	cp.Results = append(cp.Results, newResults...)

	allDone := cp.CompletedTaskIDs()
	if len(allDone) >= len(cp.Tasks) {
		cp.Phase = CheckpointDone
	} else {
		cp.Phase = CheckpointExecuting
	}
	d.checkpointSave(cp)
	return nil
}

// executeTasks runs tasks in numeric ID order, respecting the dependency DAG.
func (d *TaskDispatcher) executeTasks(ctx context.Context, allTasks []Task, completedIDs map[string]bool, msgCh chan<- tea.Msg) []TaskResult {
	allTasks = sortTasksByID(allTasks)
	results := make([]TaskResult, 0, len(allTasks))
	var mu sync.Mutex
	completed := make(map[string]bool)
	for id := range completedIDs {
		completed[id] = true
	}
	running := make(map[string]bool)

	var wg sync.WaitGroup
	sem := make(chan struct{}, d.maxParallel)

	var launchReady func()
	launchReady = func() {
		mu.Lock()
		defer mu.Unlock()
		for _, t := range allTasks {
			if completed[t.ID] || running[t.ID] {
				continue
			}
			allDepsDone := true
			for _, depID := range t.DependsOn {
				if !completed[depID] {
					allDepsDone = false
					break
				}
			}
			if allDepsDone {
				running[t.ID] = true
				task := t
				wg.Add(1)
				go func(task Task) {
					sem <- struct{}{}
					defer func() { <-sem }()
					defer wg.Done()

					result := d.runSubAgent(ctx, task, msgCh)

					mu.Lock()
					completed[task.ID] = true
					running[task.ID] = false
					results = append(results, result)
					d.appendCheckpointResult(task.ID, result)
					mu.Unlock()

					launchReady()
				}(task)
			}
		}
	}

	launchReady()
	wg.Wait()
	return results
}

// scaleSubAgentLimits bumps turns/timeout for heavier sub-tasks.
// baseTurns <= 0 means unlimited (no cap).
func scaleSubAgentLimits(task Task, baseTurns int, baseTimeout time.Duration) (int, time.Duration) {
	if baseTurns <= 0 {
		return 0, scaleSubAgentTimeout(task, baseTimeout)
	}
	turns := baseTurns
	timeout := baseTimeout
	if len(task.DependsOn) > 0 || len(task.Description) > 120 {
		turns += 5
		timeout += 60 * time.Second
	}
	if len(task.Description) > 240 {
		turns += 5
		timeout += 60 * time.Second
	}
	return turns, scaleSubAgentTimeout(task, timeout)
}

func scaleSubAgentTimeout(task Task, timeout time.Duration) time.Duration {
	maxTimeout := 5 * time.Minute
	if timeout > maxTimeout {
		return maxTimeout
	}
	_ = task
	return timeout
}

// runSubAgent creates a lightweight sub-agent that executes a single task.
func (d *TaskDispatcher) runSubAgent(ctx context.Context, task Task, msgCh chan<- tea.Msg) TaskResult {
	if IsVerificationTask(task) {
		return d.runVerificationTask(ctx, task, msgCh)
	}

	msgCh <- SubtaskMsg{TaskID: task.ID, Status: "running", Label: task.Description}

	maxAttempts := 1 + d.subAgentMaxRetries
	var lastResult TaskResult

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			label := fmt.Sprintf("retry %d/%d: %s", attempt, d.subAgentMaxRetries, task.Description)
			msgCh <- SubtaskMsg{TaskID: task.ID, Status: "running", Label: label}
		}

		subSess := session.New()
		if hint := ParentContextForSubAgent(d.agent.session.Entries); hint != "" {
			subSess.AddWithState("system", hint, "parent-context", 0)
		}

		subGuard := NewProjectAwarenessGuard(d.agent.toolbox, subSess, "")
		subGuard.state = GuardDone

		scaledTurns, scaledTimeout := scaleSubAgentLimits(task, d.subAgentMaxTurns, d.subAgentTimeout)

		subAgent := &Agent{
			state:                 StateIdle,
			provider:              d.agent.provider,
			permChecker:           d.agent.permChecker,
			toolbox:               d.agent.toolbox,
			session:               subSess,
			maxTurns:              scaledTurns,
			subAgent:              true,
			guard:                 subGuard,
			ctxMgr:                d.agent.ctxMgr,
			retryConfig:           d.agent.retryConfig,
			maxConsecutiveDenials: d.agent.maxConsecutiveDenials,
			acceptanceStrict:      d.agent.acceptanceStrict,
			pipelineOpts:          d.agent.pipelineOpts,
		}
		if dir := d.agent.OutputDir(); dir != "" {
			subAgent.SetOutputDir(dir)
		}
		subAgent.SetProjectLayout(d.agent.ProjectLayout())
		subAgent.SetActiveSubtaskID(task.ID)

		timeoutCtx, cancel := context.WithTimeout(ctx, scaledTimeout)
		err := subAgent.RunLoop(timeoutCtx, task.Description, msgCh)
		cancel()

		result := TaskResult{TaskID: task.ID}
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			lastResult = result
			if attempt+1 < maxAttempts {
				msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: fmt.Sprintf("attempt %d failed: %s", attempt+1, err.Error())}
				continue
			}
			msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: err.Error()}
			return result
		}

		if subAgent.state == StateError {
			result.Success = false
			result.Error = "sub-agent stopped before satisfying acceptance gate"
			msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: result.Error}
			return result
		}

		verifyDir := subAgent.resolveVerifyDir()
		if verifyDir != "" && !IsVerificationTask(task) {
			if !subAgent.runVerifyFixUntilPass(ctx, msgCh, verifyDir) {
				result.Success = false
				result.Error = "verification failed after retries"
				lastResult = result
				if attempt+1 < maxAttempts {
					msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: result.Error}
					continue
				}
				msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: result.Error}
				return result
			}
		}

		result.Content = subAgent.session.LastAssistantContent()
		if ok, why := validateSubTaskResult(task, subAgent.session, subAgent.resolveVerifyDir(), d.agent.acceptanceStrict); !ok {
			result.Success = false
			result.Error = why
			lastResult = result
			if attempt+1 < maxAttempts {
				msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: why}
				continue
			}
			msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: why}
			return result
		}

		result.Success = true
		msgCh <- SubtaskMsg{TaskID: task.ID, Status: "done", Label: task.Description}
		return result
	}

	return lastResult
}

// mergeResults injects sub-task results, audits evidence, runs final acceptance, and signals outcome.
func (d *TaskDispatcher) mergeResults(ctx context.Context, instruction string, cp *Checkpoint, msgCh chan<- tea.Msg) DispatchOutcome {
	results := cp.Results
	d.logSubTaskResults(results)
	var sb strings.Builder
	sb.WriteString("[SUB-TASK RESULTS]\n")
	if len(results) == 0 {
		sb.WriteString("- (no parallel sub-tasks recorded)\n")
	}
	for _, r := range results {
		status := "OK"
		if !r.Success {
			status = "FAILED"
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", status, r.TaskID, r.Content))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("  error: %s\n", r.Error))
		}
	}
	d.agent.session.AddWithState("system", sb.String(), StateDone.String(), 0)

	projectDir := d.agent.resolveVerifyDir()
	tasksOK := auditSubTaskResults(results)
	statuses, accepted := d.agent.driveUntilAccepted(ctx, msgCh, instruction, true, results)
	cp.CriteriaStatus = statuses
	d.checkpointSave(cp)

	if tasksOK && accepted && allCriteriaMet(statuses) {
		d.agent.signalTaskComplete(ctx, msgCh, true, len(results), projectDir, results, statuses)
		return OutcomeSuccess
	}

	reason := "could not complete task after autonomous recovery"
	if !tasksOK {
		reason = "one or more sub-tasks failed"
	} else if !accepted || !allCriteriaMet(statuses) {
		reason = "acceptance criteria not fully met"
	}
	d.agent.signalTaskFailed(ctx, msgCh, reason, statuses, results)
	return OutcomeFailed
}

func (d *TaskDispatcher) logSubTaskResults(results []TaskResult) {
	if len(results) == 0 {
		d.agent.logRun("dispatcher", "sub-task audit: no results recorded")
		return
	}
	for _, r := range results {
		if r.Success {
			d.agent.logRun("dispatcher", "sub-task %s OK", r.TaskID)
		} else {
			errMsg := r.Error
			if errMsg == "" {
				errMsg = "unknown"
			}
			d.agent.logRun("dispatcher", "sub-task %s FAILED err=%s", r.TaskID, errMsg)
		}
	}
}

func (d *TaskDispatcher) signalDispatchFailed(ctx context.Context, msgCh chan<- tea.Msg, instruction string, statuses []CriterionStatus, results []TaskResult, reason string) {
	d.agent.signalTaskFailed(ctx, msgCh, reason, statuses, results)
}

func tasksToPlanItems(tasks []Task) []TaskPlanItem {
	items := make([]TaskPlanItem, len(tasks))
	for i, t := range tasks {
		items[i] = TaskPlanItem{
			ID:          t.ID,
			Description: t.Description,
			DependsOn:   append([]string(nil), t.DependsOn...),
		}
	}
	return items
}

// Rollback rolls back a specific task and re-executes from there.
func (d *TaskDispatcher) Rollback(ctx context.Context, taskID string, msgCh chan<- tea.Msg) (bool, error) {
	cp, _ := d.checkpointLoad()
	if cp == nil {
		msgCh <- StreamChunkMsg{Content: "No checkpoint to rollback.", Done: true}
		return false, nil
	}
	cp.RollbackTo(taskID)
	d.checkpointSave(cp)

	msgCh <- StreamChunkMsg{
		Content: fmt.Sprintf("Rolled back task %s and its dependents. Resuming execution...", taskID),
		Done:    true,
	}
	err := d.executeTasksWithCheckpoint(ctx, cp, msgCh)
	if err != nil {
		return false, err
	}
	outcome := d.mergeResults(ctx, cp.Instruction, cp, msgCh)
	if outcome == OutcomeSuccess {
		d.checkpointClear()
	}
	return outcome == OutcomeSuccess, nil
}

// showTaskPlan sends the task plan to the TUI.
func (d *TaskDispatcher) showTaskPlan(tasks []Task, msgCh chan<- tea.Msg) {
	type kv struct{ id string; depth int }
	var items []kv
	var calcDepth func(t Task, visited map[string]bool, depthSoFar int)
	var depthMap = make(map[string]int)
	calcDepth = func(t Task, visited map[string]bool, depthSoFar int) {
		if _, ok := depthMap[t.ID]; ok {
			if depthSoFar > depthMap[t.ID] {
				depthMap[t.ID] = depthSoFar
			}
			return
		}
		if visited[t.ID] {
			return
		}
		visited[t.ID] = true
		depthMap[t.ID] = depthSoFar
		for _, depID := range t.DependsOn {
			for _, dt := range tasks {
				if dt.ID == depID {
					calcDepth(dt, visited, depthSoFar+1)
					break
				}
			}
		}
	}
	for _, t := range tasks {
		calcDepth(t, make(map[string]bool), 0)
	}
	for _, t := range tasks {
		items = append(items, kv{t.ID, depthMap[t.ID]})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].depth != items[j].depth {
			return items[i].depth < items[j].depth
		}
		return items[i].id < items[j].id
	})

	taskLookup := make(map[string]Task, len(tasks))
	for _, t := range tasks {
		taskLookup[t.ID] = t
	}

	var sb strings.Builder
	sb.WriteString("Task Plan:\n")
	for _, it := range items {
		t := taskLookup[it.id]
		sb.WriteString(fmt.Sprintf("  [%s] %s", t.ID, t.Description))
		if len(t.DependsOn) > 0 {
			sb.WriteString(fmt.Sprintf("  (depends: %s)", strings.Join(t.DependsOn, ", ")))
		}
		sb.WriteString("\n")
	}
	msgCh <- StreamChunkMsg{Content: sb.String(), Done: true}
}

// appendCheckpointResult adds a single task result to the on-disk checkpoint.
func (d *TaskDispatcher) appendCheckpointResult(taskID string, result TaskResult) {
	if d.checkpointStore == nil {
		return
	}
	cp, err := d.checkpointStore.Load()
	if err != nil || cp == nil {
		return
	}
	cp.Results = append(cp.Results, result)
	d.checkpointStore.Save(cp)
}

func (d *TaskDispatcher) checkpointSave(cp *Checkpoint) {
	if d.checkpointStore != nil {
		d.checkpointStore.Save(cp)
	}
	if d.agent != nil {
		d.agent.rememberCheckpoint(cp)
	}
}

func (d *TaskDispatcher) checkpointLoad() (*Checkpoint, error) {
	if d.checkpointStore == nil {
		return nil, nil
	}
	return d.checkpointStore.Load()
}

func (d *TaskDispatcher) checkpointClearUnlessResumable(criteria []CriterionStatus) {
	if d.agent.acceptanceStrict && len(criteria) > 0 && !allCriteriaMet(criteria) {
		return
	}
	d.checkpointClear()
}

func (d *TaskDispatcher) checkpointClear() {
	if d.checkpointStore != nil {
		d.checkpointStore.Clear()
	}
	if d.agent != nil {
		d.agent.clearCheckpointSnap()
	}
}

func (d *TaskDispatcher) waitForPlanConfirm(ctx context.Context, msgCh chan<- tea.Msg, taskCount int) (bool, error) {
	if d.skipPlanConfirm || taskCount <= 1 {
		return true, nil
	}
	reply := make(chan TaskPlanConfirmResponse, 1)
	msgCh <- TaskPlanConfirmMsg{TaskCount: taskCount, Reply: reply}
	select {
	case resp := <-reply:
		return resp.Confirmed, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

// cleanJSON extracts JSON from LLM responses that may contain markdown.
func cleanJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func sortTasksByID(tasks []Task) []Task {
	out := make([]Task, len(tasks))
	copy(out, tasks)
	sort.Slice(out, func(i, j int) bool {
		ai, aErr := parseTaskNum(out[i].ID)
		bj, bErr := parseTaskNum(out[j].ID)
		if aErr == nil && bErr == nil {
			return ai < bj
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func parseTaskNum(id string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(id), "%d", &n)
	return n, err
}
