package agent

import (
	"strings"
	"testing"
)

func TestMergeFollowUpInstruction(t *testing.T) {
	got := MergeFollowUpInstruction("implement snake", "add scoreboard")
	if got == "" || !strings.Contains(got, "implement snake") || !strings.Contains(got, "add scoreboard") {
		t.Fatalf("merge: %q", got)
	}
	if MergeFollowUpInstruction("", "only") != "only" {
		t.Fatal("empty base")
	}
}

func TestPrepareRunInstruction_AutoResumeOnContinue(t *testing.T) {
	dir := t.TempDir()
	store := NewCheckpointStore(dir)
	a := New(nil, nil, nil, nil)
	a.SetCheckpointStore(store)
	_ = store.Save(&Checkpoint{
		Phase:       CheckpointExecuting,
		Instruction: "build api",
		Interrupted: true,
	})

	merged := a.PrepareRunInstruction("continue")
	if !strings.Contains(merged, "build api") {
		t.Fatalf("merged: %q", merged)
	}
	if !a.ConsumeAutoResume() {
		t.Fatal("expected auto-resume flag for continue")
	}
	if mode := a.consumeFollowUpMode(); mode != FollowUpContinue {
		t.Fatalf("mode=%v want continue", mode)
	}
}

func TestPrepareRunInstruction_ReplanOnSupplement(t *testing.T) {
	dir := t.TempDir()
	store := NewCheckpointStore(dir)
	a := New(nil, nil, nil, nil)
	a.SetCheckpointStore(store)
	_ = store.Save(&Checkpoint{
		Phase:       CheckpointExecuting,
		Instruction: "build api",
		Interrupted: true,
	})

	merged := a.PrepareRunInstruction("also add tests")
	if !strings.Contains(merged, "add tests") {
		t.Fatalf("merged: %q", merged)
	}
	if a.ConsumeAutoResume() {
		t.Fatal("supplement should not set auto-resume")
	}
	if mode := a.consumeFollowUpMode(); mode != FollowUpReplanLight {
		t.Fatalf("mode=%v want replan_light", mode)
	}
}
