package agent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

const verificationTaskPrefix = "verify:"

const defaultVerifyFixPrompt = `[VERIFICATION FAILED] Build or tests did not pass.

Analyze the errors above, fix the code, and resolve missing dependencies.
- Use shell_exec for go mod tidy, go get, npm install, etc. when needed (these require user approval unless yolo mode).
- Re-check with go build ./... and go test ./... only — never go run or dev servers.
- Stop calling tools only when verification would pass.`

// IsVerificationTask reports whether a planned task is the auto-appended verify step.
func IsVerificationTask(t Task) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(t.Description)), verificationTaskPrefix)
}

// ensureVerificationTask appends a final verify step depending on all leaf tasks.
func ensureVerificationTask(tasks []Task) []Task {
	if len(tasks) == 0 {
		return tasks
	}
	for _, t := range tasks {
		if IsVerificationTask(t) {
			return tasks
		}
	}
	leaves := leafTaskIDs(tasks)
	if len(leaves) == 0 {
		leaves = []string{tasks[len(tasks)-1].ID}
	}
	return append(tasks, Task{
		ID:          nextNumericTaskID(tasks),
		Description: verificationTaskPrefix + " run go build ./... and go test ./...; fix compile, test, and dependency issues until all pass",
		DependsOn:   leaves,
	})
}

func findVerificationTask(tasks []Task) *Task {
	for i := range tasks {
		if IsVerificationTask(tasks[i]) {
			return &tasks[i]
		}
	}
	return nil
}

func implementationTasks(tasks []Task) []Task {
	out := make([]Task, 0, len(tasks))
	for _, t := range tasks {
		if !IsVerificationTask(t) {
			out = append(out, t)
		}
	}
	return out
}

func leafTaskIDs(tasks []Task) []string {
	referenced := make(map[string]bool)
	for _, t := range tasks {
		if IsVerificationTask(t) {
			continue
		}
		for _, dep := range t.DependsOn {
			referenced[dep] = true
		}
	}
	var leaves []string
	for _, t := range tasks {
		if IsVerificationTask(t) {
			continue
		}
		if !referenced[t.ID] {
			leaves = append(leaves, t.ID)
		}
	}
	sort.Strings(leaves)
	return leaves
}

func nextNumericTaskID(tasks []Task) string {
	max := 0
	for _, t := range tasks {
		var n int
		if _, err := fmt.Sscanf(t.ID, "%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("%d", max+1)
}

func (d *TaskDispatcher) runVerificationTask(ctx context.Context, task Task, msgCh chan<- tea.Msg) TaskResult {
	msgCh <- SubtaskMsg{TaskID: task.ID, Status: "running", Label: task.Description}

	projectDir := d.agent.OutputDir()
	if projectDir == "" {
		return TaskResult{TaskID: task.ID, Success: false, Error: "no project workspace for verification"}
	}

	ok := d.agent.runVerifyFixUntilPass(ctx, msgCh, projectDir)
	if ok {
		msgCh <- SubtaskMsg{TaskID: task.ID, Status: "done", Label: task.Description}
		return TaskResult{TaskID: task.ID, Success: true, Content: "Build and tests passed"}
	}
	msgCh <- SubtaskMsg{TaskID: task.ID, Status: "error", Label: "verification failed after retries"}
	return TaskResult{TaskID: task.ID, Success: false, Error: "verification failed after retries"}
}
