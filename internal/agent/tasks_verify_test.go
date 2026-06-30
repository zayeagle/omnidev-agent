package agent

import (
	"strings"
	"testing"
)

func TestEnsureVerificationTaskBuildOnlyByDefault(t *testing.T) {
	tasks := []Task{{ID: "1", Description: "implement snake game"}}
	out := ensureVerificationTask(tasks, "implement snake game in Go")
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	desc := out[1].Description
	if !IsVerificationTask(out[1]) {
		t.Fatalf("task 2 should be verify: %+v", out[1])
	}
	if strings.Contains(desc, "go test") {
		t.Fatalf("default verify should not require go test: %q", desc)
	}
	if !strings.Contains(desc, "go build") {
		t.Fatalf("verify should require go build: %q", desc)
	}
}

func TestEnsureVerificationTaskIncludesTestsWhenRequested(t *testing.T) {
	tasks := []Task{{ID: "1", Description: "implement feature"}}
	out := ensureVerificationTask(tasks, "implement feature with unit tests")
	desc := out[1].Description
	if !strings.Contains(desc, "go test") {
		t.Fatalf("expected go test when user asked for tests: %q", desc)
	}
}

func TestEnsureVerificationTaskAppendsOnce(t *testing.T) {
	tasks := []Task{
		{ID: "1", Description: "implement feature"},
	}
	out := ensureVerificationTask(tasks, "implement feature")
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if !IsVerificationTask(out[1]) {
		t.Fatalf("task 2 should be verify: %+v", out[1])
	}
	if len(out[1].DependsOn) != 1 || out[1].DependsOn[0] != "1" {
		t.Fatalf("depends_on=%v", out[1].DependsOn)
	}
	out2 := ensureVerificationTask(out, "implement feature")
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
