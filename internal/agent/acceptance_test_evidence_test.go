package agent

import (
	"strings"
	"testing"
)

func TestApplyTestEvidencePassMarksFunctionalWhenTestsGreen(t *testing.T) {
	plan := AcceptancePlan{Criteria: []string{
		"Address the user request: snake game",
		"Tests pass (go test ./...) when _test.go files exist",
	}}
	mech := mechanicalVerifyResult{WorkspaceOK: true, BuildOK: true, TestOK: true, TestsRan: true, CustomOK: true}
	sess := sessionWithTools(2, 2, 0, 0)
	statuses := []CriterionStatus{
		{Index: 0, Text: plan.Criteria[0], Met: false, Evidence: "LLM: no gameplay evidence"},
		{Index: 1, Text: plan.Criteria[1], Met: true, Evidence: "go test ./... passed"},
	}
	out := applyTestEvidencePass(statuses, plan, mech, sess)
	if !out[0].Met {
		t.Fatalf("expected functional criterion met via tests, got %+v", out[0])
	}
	if out[0].Evidence == "" || !strings.Contains(out[0].Evidence, "go test") {
		t.Fatalf("expected test evidence, got %q", out[0].Evidence)
	}
}

func TestMechanicalTestsSatisfyAcceptance(t *testing.T) {
	plan := AcceptancePlan{Criteria: []string{"Address the user request: game"}}
	mech := mechanicalVerifyResult{WorkspaceOK: true, BuildOK: true, TestOK: true, TestsRan: true, CustomOK: true}
	sess := sessionWithTools(1, 2, 0, 0)
	statuses := []CriterionStatus{{Index: 0, Text: plan.Criteria[0], Met: false}}
	if !mechanicalTestsSatisfyAcceptance(statuses, plan, mech, sess) {
		t.Fatal("expected acceptance satisfied when tests green with writes")
	}
}

func TestUserExplicitlyWantsTests(t *testing.T) {
	if userExplicitlyWantsTests("build a CLI") {
		t.Fatal("simple request should not imply tests")
	}
	if !userExplicitlyWantsTests("add unit tests for the parser") {
		t.Fatal("expected explicit test request")
	}
}
