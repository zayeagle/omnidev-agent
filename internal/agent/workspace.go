package agent

import (
	"path/filepath"
	"runtime"
	"strings"
)

var pathBoundTools = map[string]bool{
	"write_file":  true,
	"edit_file":   true,
	"delete_file": true,
}

var legacyInternalPackages = map[string]bool{
	"agent": true, "config": true, "llm": true, "permissions": true,
	"session": true, "stream": true, "tools": true, "tui": true,
}

// validateWorkspacePath blocks file writes outside the greenfield output directory.
func (a *Agent) validateWorkspacePath(toolName string, args map[string]interface{}) (string, bool) {
	if a.outputDir == "" || !pathBoundTools[toolName] {
		return "", true
	}
	path, _ := args["path"].(string)
	if path == "" {
		return "", true
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", true
	}
	root, err := filepath.Abs(a.outputDir)
	if err != nil {
		return "", true
	}
	if pathWithinRoot(abs, root) {
		return "", true
	}
	return "BLOCKED: " + toolName + " path must stay under workspace " + root + " (got " + path + ")", false
}

// validateLegacyWrite blocks standalone-app pollution when no deliverables workspace is set.
func (a *Agent) validateLegacyWrite(toolName string, args map[string]interface{}) (string, bool) {
	if a.outputDir != "" || !pathBoundTools[toolName] {
		return "", true
	}
	if a.guard == nil || a.guard.ProjectType() != ProjectLegacy {
		return "", true
	}
	path, _ := args["path"].(string)
	if path == "" {
		return "", true
	}
	rel := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(path)), "./")

	// Root-level source / script files (e.g. hello_server.go).
	if !strings.Contains(rel, "/") {
		switch filepath.Ext(rel) {
		case ".go", ".mod", ".sh", ".py", ".rs", ".js", ".ts":
			return legacyWriteBlockMsg(rel), false
		}
	}

	parts := strings.Split(rel, "/")
	if len(parts) < 2 {
		return "", true
	}

	switch parts[0] {
	case "cmd":
		if parts[1] != "omnidev-agent" {
			return legacyWriteBlockMsg(rel), false
		}
	case "internal":
		if !legacyInternalPackages[parts[1]] {
			return legacyWriteBlockMsg(rel), false
		}
	case "tests":
		if strings.HasSuffix(rel, ".sh") {
			return legacyWriteBlockMsg(rel), false
		}
	case "go-user-api":
		return legacyWriteBlockMsg(rel), false
	}
	return "", true
}

func pathWithinRoot(abs, root string) bool {
	abs = filepath.Clean(abs)
	root = filepath.Clean(root)
	sep := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		lowerAbs := strings.ToLower(abs)
		lowerRoot := strings.ToLower(root)
		return lowerAbs == lowerRoot || strings.HasPrefix(lowerAbs, lowerRoot+sep)
	}
	return abs == root || strings.HasPrefix(abs, root+sep)
}

func legacyWriteBlockMsg(path string) string {
	return "BLOCKED: cannot write " + path + " in legacy repo. Use deliverables/<project>/ for new apps, or edit existing omnidev-agent source files."
}

// ValidateWorkspacePathForTest exposes workspace validation for unit tests.
func (a *Agent) ValidateWorkspacePathForTest(toolName string, args map[string]interface{}) (string, bool) {
	return a.validateWorkspacePath(toolName, args)
}

// ValidateLegacyWriteForTest exposes legacy write validation for unit tests.
func (a *Agent) ValidateLegacyWriteForTest(toolName string, args map[string]interface{}) (string, bool) {
	return a.validateLegacyWrite(toolName, args)
}
