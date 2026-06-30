package agent

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestValidateSubTaskResultRequiresToolActivity(t *testing.T) {
	task := Task{ID: "1", Description: "implement feature X"}
	sess := session.New()
	sess.AddWithState("assistant", "done", "Thinking", 0)

	ok, why := validateSubTaskResult(task, sess, "", true)
	if ok {
		t.Fatal("expected failure without tool activity")
	}
	if why == "" {
		t.Fatal("expected reason")
	}
}

func TestValidateSubTaskResultRelaxedMode(t *testing.T) {
	task := Task{ID: "1", Description: "implement feature X"}
	sess := session.New()

	ok, _ := validateSubTaskResult(task, sess, "", false)
	if !ok {
		t.Fatal("relaxed mode should pass")
	}
}

func TestHeuristicAcceptancePlan(t *testing.T) {
	plan := heuristicAcceptancePlan("add snake game")
	if len(plan.Criteria) < 3 {
		t.Fatalf("expected criteria, got %d", len(plan.Criteria))
	}
}

func TestAuditSubTaskResults(t *testing.T) {
	if auditSubTaskResults(nil) {
		t.Fatal("empty results should fail audit")
	}
	if !auditSubTaskResults([]TaskResult{{TaskID: "1", Success: true}}) {
		t.Fatal("single success should pass")
	}
	if auditSubTaskResults([]TaskResult{{TaskID: "1", Success: false}}) {
		t.Fatal("failed sub-task should fail audit")
	}
}

func TestDispatchOutcomeHandled(t *testing.T) {
	if OutcomeNotHandled.Handled() {
		t.Fatal("not handled")
	}
	if !OutcomeSuccess.Handled() {
		t.Fatal("success should be handled")
	}
}
