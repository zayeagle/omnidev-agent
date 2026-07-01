package agent

import "fmt"

// turnsUnlimited reports whether the current agent loop has no turn cap.
func (a *Agent) turnsUnlimited() bool {
	return a.maxTurns <= 0
}

// formatTurnCounter renders "3/50" or "3/∞" for activity labels.
func formatTurnCounter(oneBased, limit int) string {
	if limit <= 0 {
		return fmt.Sprintf("%d/∞", oneBased)
	}
	return fmt.Sprintf("%d/%d", oneBased, limit)
}

func formatTurnLimitWords(limit int) string {
	if limit <= 0 {
		return "unlimited"
	}
	return fmt.Sprintf("%d", limit)
}
