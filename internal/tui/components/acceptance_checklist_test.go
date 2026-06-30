package components

import "testing"

func TestAcceptanceChecklistInitAndUpdate(t *testing.T) {
	tn := NewTurn(1, "test")
	tn.InitAcceptanceChecklist([]string{"build passes", "tests pass"})
	if len(tn.acceptanceChecks) != 2 {
		t.Fatalf("checks = %d", len(tn.acceptanceChecks))
	}
	if tn.acceptanceChecks[0].Status != StatusPending {
		t.Fatal("expected pending")
	}
	tn.UpdateAcceptanceCheck(0, true, "go build ok")
	if tn.acceptanceChecks[0].Status != StatusSuccess {
		t.Fatal("expected success")
	}
	tn.UpdateAcceptanceCheck(1, true, "skipped — no _test.go files (N/A)")
	if tn.acceptanceChecks[1].Status != StatusSkipped {
		t.Fatalf("expected skipped, got %v", tn.acceptanceChecks[1].Status)
	}
}
