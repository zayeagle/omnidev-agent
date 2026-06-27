package version

import "strings"

// Display returns vX.Y.Z for UI and CLI output.
func Display(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "v")
	if raw == "" {
		return "v0.0.0"
	}
	return "v" + raw
}

// Core returns X.Y.Z without the v prefix.
func Core(raw string) string {
	raw = strings.TrimSpace(raw)
	return strings.TrimPrefix(raw, "v")
}
