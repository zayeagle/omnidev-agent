package components

import "testing"

func TestNormalizeActivityLabel(t *testing.T) {
	got := normalizeActivityLabel("Calling model (turn 8/20)…")
	want := "Working · Calling model (turn 8/20)…"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if normalizeActivityLabel("Working · calling model (turn 1/2)…") != "Working · calling model (turn 1/2)…" {
		t.Fatal("should not double-prefix Working")
	}
}
