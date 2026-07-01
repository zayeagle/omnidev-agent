package agent

import tea "github.com/charmbracelet/bubbletea"

// rememberCheckpoint keeps the latest task list in memory so interrupt saves
// cannot drop sub-tasks when acceptance-only data overwrote the file.
func (a *Agent) rememberCheckpoint(cp *Checkpoint) {
	if snap := cloneCheckpointSnapshot(cp); snap != nil {
		a.mu.Lock()
		a.checkpointSnap = snap
		a.mu.Unlock()
	}
}

func (a *Agent) clearCheckpointSnap() {
	a.mu.Lock()
	a.checkpointSnap = nil
	a.mu.Unlock()
}

// mergeCheckpointTasks restores Tasks/Results from the in-memory snapshot when
// the on-disk checkpoint lost them (e.g. acceptance-only persist).
func (a *Agent) mergeCheckpointTasks(cp *Checkpoint) {
	if cp == nil {
		return
	}
	a.mu.Lock()
	snap := a.checkpointSnap
	a.mu.Unlock()
	if snap == nil || len(snap.Tasks) == 0 {
		return
	}
	if len(cp.Tasks) > 0 {
		return
	}
	cp.Tasks = append([]Task(nil), snap.Tasks...)
	cp.Results = append([]TaskResult(nil), snap.Results...)
	if cp.Phase == "" || cp.Phase == CheckpointDone {
		cp.Phase = snap.Phase
	}
	if cp.Instruction == "" {
		cp.Instruction = snap.Instruction
	}
}

func cloneCheckpointSnapshot(cp *Checkpoint) *Checkpoint {
	if cp == nil || len(cp.Tasks) == 0 {
		return nil
	}
	return &Checkpoint{
		Phase:       cp.Phase,
		Instruction: cp.Instruction,
		Tasks:       append([]Task(nil), cp.Tasks...),
		Results:     append([]TaskResult(nil), cp.Results...),
	}
}

// emitCheckpointTaskStates updates the TUI with completed/failed sub-task rows
// after TaskPlanMsg seeds the full list as pending.
func emitCheckpointTaskStates(cp *Checkpoint, msgCh chan<- tea.Msg) {
	if cp == nil || len(cp.Tasks) == 0 || msgCh == nil {
		return
	}
	descByID := make(map[string]string, len(cp.Tasks))
	for _, t := range cp.Tasks {
		descByID[t.ID] = t.Description
	}
	for _, r := range cp.Results {
		label := descByID[r.TaskID]
		if r.Success {
			msgCh <- SubtaskMsg{TaskID: r.TaskID, Status: "done", Label: label}
			continue
		}
		errLabel := r.Error
		if errLabel == "" {
			errLabel = label
		}
		msgCh <- SubtaskMsg{TaskID: r.TaskID, Status: "error", Label: errLabel}
	}
}
