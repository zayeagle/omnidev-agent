package agent

import "strings"

// MergeFollowUpInstruction combines the prior task text with a new user message.
func MergeFollowUpInstruction(base, followUp string) string {
	base = strings.TrimSpace(base)
	followUp = strings.TrimSpace(followUp)
	if base == "" {
		return followUp
	}
	if followUp == "" {
		return base
	}
	return base + "\n\n[Follow-up]\n" + followUp
}

// PrepareRunInstruction merges a follow-up into an interrupted checkpoint and marks auto-resume.
func (a *Agent) PrepareRunInstruction(newInstruction string) string {
	newInstruction = strings.TrimSpace(newInstruction)
	if newInstruction == "" || a.cpStore == nil {
		return newInstruction
	}
	cp, err := a.cpStore.Load()
	if err != nil || cp == nil || !cp.Interrupted || cp.Phase == CheckpointDone {
		return newInstruction
	}
	merged := MergeFollowUpInstruction(cp.Instruction, newInstruction)
	cp.Instruction = merged
	cp.Interrupted = true
	_ = a.cpStore.Save(cp)

	mode := ClassifyFollowUpIntent(newInstruction)
	a.setFollowUpMode(mode)
	if mode == FollowUpContinue {
		a.mu.Lock()
		a.autoResumeNext = true
		a.mu.Unlock()
	}
	return merged
}

func (a *Agent) setFollowUpMode(m FollowUpMode) {
	a.mu.Lock()
	a.followUpMode = m
	a.mu.Unlock()
}

// consumeFollowUpMode returns and clears the mode set by PrepareRunInstruction.
func (a *Agent) consumeFollowUpMode() FollowUpMode {
	a.mu.Lock()
	defer a.mu.Unlock()
	m := a.followUpMode
	a.followUpMode = FollowUpUnknown
	return m
}

// ShouldAutoResume reports whether the next dispatch should skip the resume prompt.
func (a *Agent) ShouldAutoResume(cp *Checkpoint) bool {
	if cp == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return cp.Interrupted || a.autoResumeNext
}

func (a *Agent) ConsumeAutoResume() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.autoResumeNext {
		return false
	}
	a.autoResumeNext = false
	return true
}

func (a *Agent) consumeAutoResume() { a.ConsumeAutoResume() }

// SaveInterruptCheckpoint persists in-progress work so a follow-up can continue.
func (a *Agent) SaveInterruptCheckpoint(turn int) {
	if a.cpStore == nil || a.subAgent {
		return
	}
	cp, err := a.cpStore.Load()
	if err != nil || cp == nil {
		cp = &Checkpoint{
			Instruction: latestUserInstruction(a.session),
			Phase:       CheckpointExecuting,
		}
	}
	a.mergeCheckpointTasks(cp)
	if cp.Instruction == "" {
		cp.Instruction = latestUserInstruction(a.session)
	}
	if cp.Phase == "" || cp.Phase == CheckpointDone {
		cp.Phase = CheckpointExecuting
	}
	if turn > 0 {
		cp.Turn = turn
	}
	cp.Interrupted = true
	if a.acceptancePlan != nil && len(a.acceptancePlan.Criteria) > 0 {
		cp.AcceptancePlan = *a.acceptancePlan
	}
	_ = a.cpStore.Save(cp)
	a.rememberCheckpoint(cp)
}
