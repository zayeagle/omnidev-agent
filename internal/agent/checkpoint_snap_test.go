package agent

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPersistAcceptanceCheckpoint_PreservesTasks(t *testing.T) {
	dir := t.TempDir()
	store := NewCheckpointStore(dir)
	a := New(nil, nil, nil, nil)
	a.SetCheckpointStore(store)

	tasks := []Task{
		{ID: "1", Description: "implement core"},
		{ID: "2", Description: "add tests", DependsOn: []string{"1"}},
	}
	_ = store.Save(&Checkpoint{
		Phase:       CheckpointExecuting,
		Tasks:       tasks,
		Results:     []TaskResult{{TaskID: "1", Success: true, Content: "done"}},
		Instruction: "build feature",
	})
	a.rememberCheckpoint(&Checkpoint{Tasks: tasks, Results: []TaskResult{{TaskID: "1", Success: true}}})

	plan := AcceptancePlan{Criteria: []string{"tests pass"}}
	statuses := []CriterionStatus{{Index: 0, Text: "tests pass", Met: false}}
	a.persistAcceptanceCheckpoint(plan, statuses, 1)

	cp, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cp.Tasks) != 2 {
		t.Fatalf("tasks len=%d want 2", len(cp.Tasks))
	}
	if len(cp.Results) != 1 || cp.Results[0].TaskID != "1" {
		t.Fatalf("results=%v", cp.Results)
	}
	if !cp.AcceptanceIncomplete {
		t.Fatal("expected acceptance incomplete flag")
	}
}

func TestPersistAcceptanceCheckpoint_NoCheckpointNoCreate(t *testing.T) {
	dir := t.TempDir()
	store := NewCheckpointStore(dir)
	a := New(nil, nil, nil, nil)
	a.SetCheckpointStore(store)

	plan := AcceptancePlan{Criteria: []string{"tests pass"}}
	a.persistAcceptanceCheckpoint(plan, nil, 0)

	if store.HasInProgress() {
		t.Fatal("should not create acceptance-only checkpoint without tasks")
	}
}

func TestSaveInterruptCheckpoint_RestoresTasksFromSnap(t *testing.T) {
	dir := t.TempDir()
	store := NewCheckpointStore(dir)
	a := New(nil, nil, nil, nil)
	a.SetCheckpointStore(store)

	tasks := []Task{{ID: "1", Description: "work"}, {ID: "2", Description: "verify"}}
	a.rememberCheckpoint(&Checkpoint{
		Phase:       CheckpointExecuting,
		Tasks:       tasks,
		Results:     []TaskResult{{TaskID: "1", Success: true}},
		Instruction: "original",
	})
	_ = store.Save(&Checkpoint{
		Phase:       CheckpointExecuting,
		Instruction: "original",
		Interrupted: false,
	})

	a.SaveInterruptCheckpoint(3)

	cp, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cp.Tasks) != 2 {
		t.Fatalf("tasks len=%d want 2", len(cp.Tasks))
	}
	if len(cp.Results) != 1 {
		t.Fatalf("results=%v", cp.Results)
	}
	if !cp.Interrupted {
		t.Fatal("expected interrupted flag")
	}
}

func TestEmitCheckpointTaskStates(t *testing.T) {
	cp := &Checkpoint{
		Tasks: []Task{
			{ID: "1", Description: "first"},
			{ID: "2", Description: "second"},
		},
		Results: []TaskResult{
			{TaskID: "1", Success: true},
			{TaskID: "2", Success: false, Error: "build failed"},
		},
	}
	ch := make(chan tea.Msg, 4)
	emitCheckpointTaskStates(cp, ch)
	close(ch)

	var done, failed int
	for msg := range ch {
		st, ok := msg.(SubtaskMsg)
		if !ok {
			t.Fatalf("unexpected %T", msg)
		}
		switch st.Status {
		case "done":
			done++
		case "error":
			failed++
		}
	}
	if done != 1 || failed != 1 {
		t.Fatalf("done=%d failed=%d", done, failed)
	}
}
