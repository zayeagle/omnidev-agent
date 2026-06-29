package components

import "testing"

func TestRenderTodoListBlockedAndDepends(t *testing.T) {
	tasks := []*TaskEntry{
		{ID: "1", Description: "Setup", Status: StatusSuccess},
		{ID: "2", Description: "Build UI", Status: StatusBlocked, DependsOn: []string{"1"}},
	}
	lines := RenderTodoList(tasks, 80, false, "Executing")
	body := joinLines(lines)
	if !containsSubstring(body, "waits on") {
		t.Fatalf("expected dependency hint, got %q", body)
	}
}

func TestRenderTodoListCollapseWhenAllDone(t *testing.T) {
	tasks := []*TaskEntry{
		{ID: "1", Description: "A", Status: StatusSuccess},
		{ID: "2", Description: "B", Status: StatusSuccess},
	}
	lines := RenderTodoList(tasks, 80, true, "Done")
	if len(lines) == 0 || !containsSubstring(lines[0], "all done") {
		t.Fatalf("expected collapsed header, got %v", lines)
	}
}

func TestRecomputeTaskBlocked(t *testing.T) {
	tn := NewTurn(1, "x")
	tn.SetTasks([]*TaskEntry{
		{ID: "1", Description: "first", Status: StatusPending},
		{ID: "2", Description: "second", Status: StatusPending, DependsOn: []string{"1"}},
	})
	if tn.Tasks[1].Status != StatusBlocked {
		t.Fatalf("task 2 should be blocked, got %v", tn.Tasks[1].Status)
	}
	tn.UpdateTaskStatus("1", StatusSuccess, "")
	if tn.Tasks[1].Status != StatusPending {
		t.Fatalf("task 2 should be pending after dep done, got %v", tn.Tasks[1].Status)
	}
}

func joinLines(lines []string) string {
	var s string
	for _, l := range lines {
		s += l + "\n"
	}
	return s
}
