package agent

import (
	"strings"
	"testing"
)

func TestDiagnoseMechanicalVerify_PathMissingGoMod(t *testing.T) {
	dir := t.TempDir()
	d := diagnoseMechanicalVerify(dir, false, false, false, "Build check failed:\n  go.mod file not found")
	if d.Kind != VerifyFailurePath {
		t.Fatalf("kind=%q want path", d.Kind)
	}
}

func TestDiagnoseMechanicalVerify_TestHarnessWhenBuildOK(t *testing.T) {
	dir := t.TempDir()
	summary := "Build check passed (go build ./...).\nTest check failed:\n  --- FAIL: TestScore (0.00s)\n  game_test.go:12: expected 5 got 3"
	d := diagnoseMechanicalVerify(dir, true, false, true, summary)
	if d.Kind != VerifyFailureTestHarness && d.Kind != VerifyFailureProgram {
		t.Fatalf("kind=%q expected test or program", d.Kind)
	}
}

func TestDiagnoseMechanicalVerify_Environment(t *testing.T) {
	d := diagnoseMechanicalVerify("/tmp/ws", false, false, false, "Build check failed:\n  go: command not found")
	if d.Kind != VerifyFailureEnvironment {
		t.Fatalf("kind=%q want environment", d.Kind)
	}
}

func TestDiagnoseMechanicalVerify_EmptyWorkspaceDir(t *testing.T) {
	d := diagnoseMechanicalVerify("", true, false, true, "Test check failed:\n  --- FAIL: TestFoo")
	if d.Kind != VerifyFailurePath {
		t.Fatalf("kind=%q want path for empty verify dir", d.Kind)
	}
	if !stringsContainsAny(d.Evidence, "verify directory unset") {
		t.Fatalf("expected unset dir evidence, got %v", d.Evidence)
	}
}

func TestFormatAcceptanceReport_IncludesDiagnosis(t *testing.T) {
	summary := "Build check passed.\nTest check failed:\n  game_test.go:1: expected true"
	mech := mechanicalVerifyResult{
		BuildOK:   true,
		TestOK:    false,
		TestsRan:  true,
		VerifyDir: "/tmp/snake",
		Summary:   summary,
		Diagnosis: diagnoseMechanicalVerify("/tmp/snake", true, false, true, summary),
	}
	report := formatAcceptanceReport(nil, mech)
	if !strings.Contains(report, "Failure diagnosis") {
		t.Fatalf("report missing diagnosis: %s", report)
	}
}

func stringsContainsAny(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
