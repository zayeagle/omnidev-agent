package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxWholeTestFileWriteBytes = 4096

// validateTestFileWrite blocks large whole-file rewrites of existing *_test.go files.
func (a *Agent) validateTestFileWrite(toolName string, args map[string]interface{}) (string, bool) {
	if toolName != "write_file" {
		return "", true
	}
	path := strings.TrimSpace(fmt.Sprint(args["path"]))
	if path == "" || !strings.HasSuffix(strings.ToLower(path), "_test.go") {
		return "", true
	}
	content, _ := args["content"].(string)
	if len(content) <= maxWholeTestFileWriteBytes {
		return "", true
	}
	abs := path
	if a.outputDir != "" {
		if joined := filepath.Join(a.outputDir, path); !filepath.IsAbs(path) {
			abs = joined
		}
	}
	if _, err := os.Stat(abs); err != nil {
		if _, err2 := os.Stat(path); err2 != nil {
			return "", true // new test file
		}
		abs = path
	}
	return fmt.Sprintf(
		"BLOCKED: existing _test.go (%d bytes) — use edit_file for targeted changes instead of rewriting the whole file (reduces context bloat)",
		len(content),
	), false
}

func editFileFailureHint(args map[string]interface{}) string {
	path := strings.TrimSpace(fmt.Sprint(args["path"]))
	if path == "" {
		return "Hint: read_file the full file once (no offset/limit), then edit_file with an exact old_snippet from that read."
	}
	return fmt.Sprintf(
		"Hint: read_file %q once without offset/limit, then edit_file with an exact old_snippet — avoid sweeping offset variants.",
		path,
	)
}
