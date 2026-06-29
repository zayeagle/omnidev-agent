package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

// Task represents a unit of work that may depend on other tasks.
type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	DependsOn   []string `json:"depends_on,omitempty"`
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
		SubAgentTimeout:    120 * time.Second,
		SubAgentMaxTurns:   10,
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
	if opts.SubAgentMaxTurns < 1 {
		opts.SubAgentMaxTurns = DefaultDispatcherOptions().SubAgentMaxTurns
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
func (d *TaskDispatcher) Dispatch(ctx context.Context, instruction string, msgCh chan<- tea.Msg) (bool, error) {
	// Check for existing checkpoint (resume scenario)
	cp, _ := d.checkpointLoad()
	if cp != nil && cp.Phase != CheckpointDone {
		reply := make(chan CheckpointResponse, 1)
		msgCh <- CheckpointPromptMsg{
			Phase:     string(cp.Phase),
			Completed: len(cp.Results),
			Total:     len(cp.Tasks),
			Reply:     reply,
		}
		var resume bool
		select {
		case resp := <-reply:
			resume = resp.Resume
		case <-ctx.Done():
			return false, ctx.Err()
		}
		if resume {
			msgCh <- TaskPlanMsg{Tasks: tasksToPlanItems(cp.Tasks)}
			return d.dispatchWithCheckpoint(ctx, instruction, cp, msgCh)
		}
		d.checkpointClear()
	}

	// Fresh start: decompose then execute
	tasks, err := d.Plan(ctx, instruction)
	if err != nil {
		msgCh <- StreamChunkMsg{
			Content: fmt.Sprintf("Decomposition failed (%v), falling back to sequential execution.", err),
			Done:    true,
		}
		return false, err
	}

	cp = &Checkpoint{
		Phase:       CheckpointDecomposed,
		Tasks:       tasks,
		Instruction: instruction,
	}
	d.checkpointSave(cp)

	msgCh <- TaskPlanMsg{Tasks: tasksToPlanItems(tasks)}

	if len(tasks) > 1 {
		confirmed, err := d.waitForPlanConfirm(ctx, msgCh, len(tasks))
		if err != nil {
			return false, err
		}
		if !confirmed {
			d.checkpointClear()
			msgCh <- StreamChunkMsg{Content: "Task plan cancelled.", Done: true}
			return true, nil
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
			return false, err
		}
		if d.agent.state != StateError {
			msgCh <- SubtaskMsg{TaskID: implTasks[0].ID, Status: "done", Label: implTasks[0].Description}
			result := d.runVerificationTask(ctx, *verifyTask, msgCh)
			projectDir := d.agent.OutputDir()
			if result.Success {
				d.agent.signalTaskComplete(ctx, msgCh, true, 1, projectDir, []TaskResult{result})
			} else {
				d.agent.setState(StateError)
				msgCh <- StreamChunkMsg{Content: "Verification failed after retries.", Done: true}
				msgCh <- ErrorMsg{Error: result.Error}
			}
		}
		d.checkpointClear()
		return true, nil
	}

	err = d.executeTasksWithCheckpoint(ctx, cp, msgCh)
	if err != nil {
		return false, err
	}
	d.mergeResults(ctx, cp.Results, msgCh)
	d.checkpointClear()
	return true, nil
}

// Plan decomposes an instruction into sub-tasks using the LLM (default).
// Mode 0/1: LLM decides whether to return one task or many. Mode 2: skip LLM, single task.
func (d *TaskDispatcher) Plan(ctx context.Context, instruction string) ([]Task, error) {
	if d.agent.pipelineOpts.PlanMode == 2 {
		return ensureVerificationTask([]Task{{ID: "1", Description: instruction}}), nil
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
- Do NOT add a final verification task — one is appended automatically after planning%s

Request: %s

Output format (one task example):
[{"id": "1", "description": "implement X end-to-end"}]

Multi-task example:
[{"id": "1", "description": "do X"}, {"id": "2", "description": "do Y", "depends_on": ["1"]}]`, layoutHint, instruction)

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
	return ensureVerificationTask(tasks), nil
}

// dispatchWithCheckpoint handles resume after a checkpoint is found.
func (d *TaskDispatcher) dispatchWithCheckpoint(ctx context.Context, instruction string, cp *Checkpoint, msgCh chan<- tea.Msg) (bool, error) {
	if cp.Phase == CheckpointDone {
		d.checkpointClear()
		msgCh <- StreamChunkMsg{Content: "Previous session completed. Starting fresh.", Done: true}
		return false, nil
	}

	msgCh <- StreamChunkMsg{
		Content: fmt.Sprintf("Resuming from checkpoint — %d/%d tasks completed. Remaining: %d",
			len(cp.Results), len(cp.Tasks), len(cp.Tasks)-len(cp.Results)),
		Done: true,
	}

	err := d.executeTasksWithCheckpoint(ctx, cp, msgCh)
	if err != nil {
		return false, err
	}
	d.mergeResults(ctx, cp.Results, msgCh)
	d.checkpointClear()
	return true, nil
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

		subAgent := &Agent{
			state:                 StateIdle,
			provider:              d.agent.provider,
			permChecker:           d.agent.permChecker,
			toolbox:               d.agent.toolbox,
			session:               subSess,
			maxTurns:              d.subAgentMaxTurns,
			subAgent:              true,
			guard:                 subGuard,
			ctxMgr:                d.agent.ctxMgr,
			retryConfig:           d.agent.retryConfig,
			maxConsecutiveDenials: d.agent.maxConsecutiveDenials,
		}
		if dir := d.agent.OutputDir(); dir != "" {
			subAgent.SetOutputDir(dir)
		}
		subAgent.SetProjectLayout(d.agent.ProjectLayout())
		subAgent.SetActiveSubtaskID(task.ID)

		timeoutCtx, cancel := context.WithTimeout(ctx, d.subAgentTimeout)
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

		result.Content = subAgent.session.LastAssistantContent()
		result.Success = true
		msgCh <- SubtaskMsg{TaskID: task.ID, Status: "done", Label: task.Description}
		return result
	}

	return lastResult
}

// mergeResults injects sub-task results into the parent session and emits summary.
func (d *TaskDispatcher) mergeResults(ctx context.Context, results []TaskResult, msgCh chan<- tea.Msg) {
	var sb strings.Builder
	sb.WriteString("[SUB-TASK RESULTS]\n")
	allOK := true
	for _, r := range results {
		status := "OK"
		if !r.Success {
			status = "FAILED"
			allOK = false
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", status, r.TaskID, r.Content))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("  error: %s\n", r.Error))
		}
	}

	d.agent.session.AddWithState("system", sb.String(), StateDone.String(), 0)

	projectDir := d.agent.OutputDir()
	if projectDir != "" {
		if abs, err := filepath.Abs(projectDir); err == nil {
			projectDir = abs
		}
	}
	if allOK {
		conclusion := d.agent.BuildFinalConclusion(ctx, results)
		msgCh <- NewAllComplete(conclusion, projectDir)
	} else {
		msgCh <- StreamChunkMsg{Content: "Some sub-tasks failed. See details above.", Done: true}
	}
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
	d.mergeResults(ctx, cp.Results, msgCh)
	if cp.Phase == CheckpointDone {
		d.checkpointClear()
	}
	return true, nil
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
}

func (d *TaskDispatcher) checkpointLoad() (*Checkpoint, error) {
	if d.checkpointStore == nil {
		return nil, nil
	}
	return d.checkpointStore.Load()
}

func (d *TaskDispatcher) checkpointClear() {
	if d.checkpointStore != nil {
		d.checkpointStore.Clear()
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
