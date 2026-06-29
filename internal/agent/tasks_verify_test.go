package agent

import "testing"

func TestEnsureVerificationTaskAppendsOnce(t *testing.T) {
	tasks := []Task{
		{ID: "1", Description: "implement feature"},
	}
	out := ensureVerificationTask(tasks)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if !IsVerificationTask(out[1]) {
		t.Fatalf("task 2 should be verify: %+v", out[1])
	}
	if len(out[1].DependsOn) != 1 || out[1].DependsOn[0] != "1" {
		t.Fatalf("depends_on=%v", out[1].DependsOn)
	}
	out2 := ensureVerificationTask(out)
	if len(out2) != 2 {
		t.Fatalf("should not duplicate verify task, len=%d", len(out2))
	}
}

func TestLeafTaskIDs(t *testing.T) {
	tasks := []Task{
		{ID: "1", Description: "a"},
		{ID: "2", Description: "b", DependsOn: []string{"1"}},
	}
	leaves := leafTaskIDs(tasks)
	if len(leaves) != 1 || leaves[0] != "2" {
		t.Fatalf("leaves=%v", leaves)
	}
}
