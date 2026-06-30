package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type projectKind int

const (
	projectUnknown projectKind = iota
	projectGo
	projectNode
	projectPython
)

func detectProjectKind(dir string) projectKind {
	if fileExists(filepath.Join(dir, "go.mod")) {
		return projectGo
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		return projectNode
	}
	if fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "requirements.txt")) ||
		hasPythonSources(dir) {
		return projectPython
	}
	return projectUnknown
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func hasPythonSources(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "vendor", "node_modules", "__pycache__", ".venv", "venv":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".py") {
			found = true
		}
		return nil
	})
	return found
}

// WorkspaceVerifyResult holds split build/test outcomes for acceptance criteria.
type WorkspaceVerifyResult struct {
	Summary  string
	BuildOK  bool
	TestOK   bool
	TestsRan bool
	OK       bool
}

// VerifyProjectWorkspace runs project-appropriate non-interactive checks.
func VerifyProjectWorkspace(ctx context.Context, dir string) (summary string, ok bool) {
	r := VerifyProjectWorkspaceDetailed(ctx, dir)
	return r.Summary, r.OK
}

// VerifyProjectWorkspaceDetailed runs checks and reports build vs test separately.
func VerifyProjectWorkspaceDetailed(ctx context.Context, dir string) WorkspaceVerifyResult {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return WorkspaceVerifyResult{OK: true}
	}

	switch detectProjectKind(dir) {
	case projectGo:
		return verifyGoWorkspaceDetailed(ctx, dir)
	case projectNode:
		s, ok := verifyNodeWorkspace(ctx, dir)
		return WorkspaceVerifyResult{Summary: s, BuildOK: ok, TestOK: ok, OK: ok}
	case projectPython:
		s, ok := verifyPythonWorkspace(ctx, dir)
		return WorkspaceVerifyResult{Summary: s, BuildOK: ok, TestOK: ok, OK: ok}
	default:
		s, ok := verifyUnknownWorkspace(dir)
		return WorkspaceVerifyResult{Summary: s, BuildOK: ok, TestOK: ok, OK: ok}
	}
}

func verifyGoWorkspaceDetailed(ctx context.Context, dir string) WorkspaceVerifyResult {
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	var lines []string
	buildOK := true
	testOK := true
	testsRan := false

	if out, err := runVerifyCommand(ctx, dir, goBuildArgs()); err != nil {
		buildOK = false
		lines = append(lines, "Build check failed:")
		lines = append(lines, indentOutput(out, err))
	} else {
		lines = append(lines, "Build check passed (go build ./...).")
	}

	if hasGoTests(dir) {
		testsRan = true
		if out, err := runVerifyCommand(ctx, dir, []string{"test", "./..."}); err != nil {
			testOK = false
			lines = append(lines, "Test check failed:")
			lines = append(lines, indentOutput(out, err))
		} else {
			lines = append(lines, "Tests passed (go test ./...).")
		}
	}
	return WorkspaceVerifyResult{
		Summary:  strings.Join(lines, "\n"),
		BuildOK:  buildOK,
		TestOK:     testOK,
		TestsRan: testsRan,
		OK:       buildOK && (!testsRan || testOK),
	}
}

func verifyNodeWorkspace(ctx context.Context, dir string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	script := npmScript(dir, "build")
	if script == "" {
		script = npmScript(dir, "test")
	}
	if script == "" {
		return "Node project: no build/test script in package.json (skipped).", true
	}

	cmd := exec.CommandContext(ctx, "npm", "run", script)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	summary := strings.TrimSpace(string(out))
	if err != nil {
		if summary == "" {
			summary = err.Error()
		}
		return "npm run " + script + " failed:\n" + summary, false
	}
	return "npm run " + script + " passed.", true
}

func npmScript(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	if pkg.Scripts == nil {
		return ""
	}
	if _, ok := pkg.Scripts[name]; ok {
		return name
	}
	return ""
}

func verifyPythonWorkspace(ctx context.Context, dir string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python", "-m", "compileall", "-q", ".")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		summary := strings.TrimSpace(string(out))
		if summary == "" {
			summary = err.Error()
		}
		return "Python compileall failed:\n" + summary, false
	}
	return "Python compileall passed.", true
}

func verifyUnknownWorkspace(dir string) (string, bool) {
	if hasAnySourceFile(dir) {
		return "No recognized project manifest (go.mod / package.json / Python). Manual review recommended.", true
	}
	return "", true
}

func hasAnySourceFile(dir string) bool {
	exts := []string{".go", ".py", ".js", ".ts", ".tsx", ".jsx", ".rs", ".java"}
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "__pycache__":
				return filepath.SkipDir
			}
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(d.Name(), ext) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}
