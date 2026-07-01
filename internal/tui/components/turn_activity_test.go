package components

import "testing"

func TestSanitizeActivityDetail_HidesTurnLabels(t *testing.T) {
	for _, in := range []string{
		"Working · calling model (turn 8/20)…",
		"Calling model (turn 8/20)…",
		"Thinking · waiting for model (turn 3/20)…",
		"Acceptance recovery 3/10 · turn 3/10…",
	} {
		if got := sanitizeActivityDetail(in); got != "" {
			t.Fatalf("%q: got %q want empty", in, got)
		}
	}
}

func TestSanitizeActivityDetail_KeepsUsefulLabels(t *testing.T) {
	for _, in := range []string{
		"Working · edit_file…",
		"Working · checking acceptance criteria…",
		"Working · recovering acceptance…",
	} {
		if got := sanitizeActivityDetail(in); got != in {
			t.Fatalf("%q: got %q", in, got)
		}
	}
}

func TestNormalizeActivityLabel(t *testing.T) {
	if got := normalizeActivityLabel("Working · calling model (turn 8/20)…"); got != "Working" {
		t.Fatalf("got %q want Working", got)
	}
	if normalizeActivityLabel("Working · shell_exec…") != "Working · shell_exec…" {
		t.Fatal("tool label should remain")
	}
}
