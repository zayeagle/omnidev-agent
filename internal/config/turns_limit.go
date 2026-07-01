package config

import "strconv"

// NormalizeTurnLimit maps config values to agent turn budgets.
// Positive N = max N LLM↔tool turns; 0 or negative = unlimited (stored as 0).
func NormalizeTurnLimit(n int) int {
	if n <= 0 {
		return 0
	}
	return n
}

// FormatTurnLimit renders a turn limit for /status and logs.
func FormatTurnLimit(n int) string {
	if n <= 0 {
		return "unlimited"
	}
	return strconv.Itoa(n)
}
