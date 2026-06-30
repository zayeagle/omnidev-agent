package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CheckpointPhase marks which stage of the pipeline the checkpoint was saved at.
type CheckpointPhase string

const (
	CheckpointUnderstanding CheckpointPhase = "understanding"
	CheckpointDecomposed    CheckpointPhase = "decomposed"
	CheckpointExecuting     CheckpointPhase = "executing"
	CheckpointDone          CheckpointPhase = "done"
)

// Checkpoint captures the full state of an in-progress agent session
// so execution can be resumed or rolled back after an interrupt.
type Checkpoint struct {
	Phase           CheckpointPhase   `json:"phase"`
	Tasks           []Task            `json:"tasks"`
	Results         []TaskResult      `json:"results"` // accumulated so far
	Instruction     string            `json:"instruction"`
	AcceptancePlan  AcceptancePlan    `json:"acceptance_plan,omitempty"`
	CriteriaStatus  []CriterionStatus `json:"criteria_status,omitempty"`
	ExitGateNudges  int               `json:"exit_gate_nudges,omitempty"`
	AcceptanceIncomplete bool           `json:"acceptance_incomplete,omitempty"`
	Turn            int               `json:"turn"` // for standard loop fallback
	Timestamp       time.Time         `json:"timestamp"`
}

// CheckpointStore persists and loads checkpoint files.
type CheckpointStore struct {
	dir string
}

// NewCheckpointStore creates a store rooted at the given directory.
func NewCheckpointStore(dir string) *CheckpointStore {
	return &CheckpointStore{dir: dir}
}

// Save writes the checkpoint to disk as JSON.
func (cs *CheckpointStore) Save(cp *Checkpoint) error {
	if err := os.MkdirAll(cs.dir, 0755); err != nil {
		return err
	}
	cp.Timestamp = time.Now()
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cs.dir, "checkpoint.json"), data, 0644)
}

// Load reads the checkpoint from disk. Returns nil if no checkpoint exists.
func (cs *CheckpointStore) Load() (*Checkpoint, error) {
	data, err := os.ReadFile(filepath.Join(cs.dir, "checkpoint.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, fmt.Errorf("checkpoint parse: %w", err)
	}
	return &cp, nil
}

// Clear removes the checkpoint file (session completed or explicitly discarded).
func (cs *CheckpointStore) Clear() error {
	p := filepath.Join(cs.dir, "checkpoint.json")
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil
	}
	return os.Remove(p)
}

// HasInProgress returns true if a checkpoint exists with a non-done phase.
func (cs *CheckpointStore) HasInProgress() bool {
	cp, err := cs.Load()
	if err != nil || cp == nil {
		return false
	}
	return cp.Phase != CheckpointDone
}

// RollbackTo removes results for a given task and all tasks that depend on it,
// rewinding the checkpoint so those tasks will re-execute.
func (cp *Checkpoint) RollbackTo(taskID string) {
	// Build dependency closure: find all tasks that transitively depend on taskID
	affected := make(map[string]bool)
	var collectDependents func(id string)
	collectDependents = func(id string) {
		for _, t := range cp.Tasks {
			for _, dep := range t.DependsOn {
				if dep == id {
					if !affected[t.ID] {
						affected[t.ID] = true
						collectDependents(t.ID)
					}
				}
			}
		}
	}
	affected[taskID] = true
	collectDependents(taskID)

	// Remove affected results
	filtered := make([]TaskResult, 0, len(cp.Results))
	for _, r := range cp.Results {
		if !affected[r.TaskID] {
			filtered = append(filtered, r)
		}
	}
	cp.Results = filtered
	cp.Phase = CheckpointExecuting
}

// CompletedTaskIDs returns the set of task IDs that have a result.
func (cp *Checkpoint) CompletedTaskIDs() map[string]bool {
	set := make(map[string]bool, len(cp.Results))
	for _, r := range cp.Results {
		set[r.TaskID] = true
	}
	return set
}
