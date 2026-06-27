package tools

import (
	"path/filepath"
	"strings"
)

var sensitiveBaseNames = map[string]bool{
	".omnidev-agent.json": true,
	".env":                true,
	".env.local":          true,
	".env.production":     true,
	"credentials.json":    true,
	"secrets.json":        true,
	"id_rsa":              true,
	"id_ed25519":          true,
	"id_ecdsa":            true,
}

// IsSensitivePath reports whether a path must not be read or listed by the agent.
func IsSensitivePath(path string) bool {
	if path == "" {
		return false
	}
	clean := filepath.Clean(path)
	base := strings.ToLower(filepath.Base(clean))
	if sensitiveBaseNames[base] {
		return true
	}
	// Block explicit home agent config paths regardless of cwd.
	lower := strings.ToLower(filepath.ToSlash(clean))
	if strings.Contains(lower, "/.omnidev-agent/") {
		return true
	}
	if strings.HasSuffix(lower, "/.ssh/id_rsa") || strings.HasSuffix(lower, "/.ssh/id_ed25519") {
		return true
	}
	return false
}
