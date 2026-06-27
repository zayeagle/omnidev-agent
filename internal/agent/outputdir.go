package agent

import (
	"os"
	"path/filepath"
	"strings"
)

// displayOutputDir returns a cwd-relative path for LLM prompts when possible.
func displayOutputDir(dir string) string {
	if dir == "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(dir)
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(dir)
	}
	return filepath.ToSlash(rel)
}
