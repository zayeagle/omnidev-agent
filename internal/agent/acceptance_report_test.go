package agent

import (
	"strings"
	"testing"
)

func TestFormatAcceptanceReportListsFailures(t *testing.T) {
	statuses := []CriterionStatus{
		{Index: 0, Text: "Implement snake game", Met: false, Evidence: "need at least one write/edit"},
		{Index: 1, Text: "Tests pass", Met: true, Evidence: "mechanical checks passed"},
	}
	report := formatAcceptanceReport(statuses, mechanicalVerifyResult{WorkspaceOK: false, Summary: "build failed"})
	if !strings.Contains(report, "[FAIL] Implement snake game") {
		t.Fatalf("expected FAIL line: %s", report)
	}
	if !strings.Contains(report, "reason: need at least one write/edit") {
		t.Fatalf("expected evidence: %s", report)
	}
	if !strings.Contains(report, "[PASS] Tests pass") {
		t.Fatalf("expected PASS line: %s", report)
	}
	if !strings.Contains(report, "build failed") {
		t.Fatalf("expected mechanical section: %s", report)
	}
}
