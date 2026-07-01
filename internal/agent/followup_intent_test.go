package agent

import "testing"

func TestClassifyFollowUpIntent_Continue(t *testing.T) {
	for _, in := range []string{"continue", "resume", "继续", "接着做", "go on"} {
		if got := ClassifyFollowUpIntent(in); got != FollowUpContinue {
			t.Fatalf("%q: got %v want continue", in, got)
		}
	}
}

func TestClassifyFollowUpIntent_ForceReplan(t *testing.T) {
	for _, in := range []string{"重新规划", "replan tasks", "re-plan from scratch"} {
		if got := ClassifyFollowUpIntent(in); got != FollowUpReplanFull {
			t.Fatalf("%q: got %v want replan_full", in, got)
		}
	}
}

func TestClassifyFollowUpIntent_NewDirection(t *testing.T) {
	for _, in := range []string{"改成做 CLI 而不是 API", "switch to a CLI instead of REST API"} {
		if got := ClassifyFollowUpIntent(in); got != FollowUpReplanFull {
			t.Fatalf("%q: got %v want replan_full", in, got)
		}
	}
}

func TestClassifyFollowUpIntent_Supplement(t *testing.T) {
	for _, in := range []string{"also add unit tests", "补充一下错误处理", "fix the build error in main.go"} {
		if got := ClassifyFollowUpIntent(in); got != FollowUpReplanLight {
			t.Fatalf("%q: got %v want replan_light", in, got)
		}
	}
}

func TestFilterResultsForTasks(t *testing.T) {
	results := []TaskResult{
		{TaskID: "1", Success: true},
		{TaskID: "2", Success: false},
		{TaskID: "3", Success: true},
	}
	tasks := []Task{{ID: "1"}, {ID: "3"}, {ID: "4"}}
	got := filterResultsForTasks(results, tasks)
	if len(got) != 2 || got[0].TaskID != "1" || got[1].TaskID != "3" {
		t.Fatalf("filter: %+v", got)
	}
}
