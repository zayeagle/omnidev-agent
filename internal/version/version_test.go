package version

import "testing"

func TestDisplay(t *testing.T) {
	if got := Display("0.0.1"); got != "v0.0.1" {
		t.Fatalf("Display = %q", got)
	}
	if got := Display("v0.0.1"); got != "v0.0.1" {
		t.Fatalf("Display = %q", got)
	}
}
