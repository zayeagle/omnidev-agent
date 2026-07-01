package config

import "testing"

func TestNormalizeTurnLimit(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{50, 50},
		{80, 80},
		{0, 0},
		{-1, 0},
	}
	for _, tc := range tests {
		if got := NormalizeTurnLimit(tc.in); got != tc.want {
			t.Fatalf("NormalizeTurnLimit(%d)=%d want %d", tc.in, got, tc.want)
		}
	}
}

func TestFormatTurnLimit(t *testing.T) {
	if FormatTurnLimit(50) != "50" {
		t.Fatalf("got %q", FormatTurnLimit(50))
	}
	if FormatTurnLimit(0) != "unlimited" {
		t.Fatalf("got %q", FormatTurnLimit(0))
	}
	if FormatTurnLimit(-1) != "unlimited" {
		t.Fatalf("got %q", FormatTurnLimit(-1))
	}
}
