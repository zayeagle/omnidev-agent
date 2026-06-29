package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const verifyTimeout = 60 * time.Second

const maxVerifyFixAttempts = 5

// VerifyProjectWorkspace runs non-interactive build checks in the output directory.
// Returns a human-readable summary and whether all checks passed.
func VerifyProjectWorkspace(ctx context.Context, dir string) (summary string, ok bool) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", true
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		return "", true
	}

	ctx, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	var lines []string
	allOK := true

	if out, err := runVerifyCommand(ctx, dir, goBuildArgs()); err != nil {
		allOK = false
		lines = append(lines, "Build check failed:")
		lines = append(lines, indentOutput(out, err))
	} else {
		lines = append(lines, "Build check passed (go build ./...).")
	}

	if hasGoTests(dir) {
		if out, err := runVerifyCommand(ctx, dir, []string{"test", "./..."}); err != nil {
			allOK = false
			lines = append(lines, "Test check failed:")
			lines = append(lines, indentOutput(out, err))
		} else {
			lines = append(lines, "Tests passed (go test ./...).")
		}
	}

	if len(lines) == 0 {
		return "", true
	}
	return strings.Join(lines, "\n"), allOK
}

func goBuildArgs() []string {
	return []string{"build", "./..."}
}

func runVerifyCommand(ctx context.Context, dir string, goArgs []string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", goArgs...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := strings.TrimSpace(stdout.String() + stderr.String())
	if out == "" && err == nil {
		out = "(no output)"
	}
	return out, err
}

func hasGoTests(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == "vendor" || base == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), "_test.go") {
			found = true
		}
		return nil
	})
	return found
}

func indentOutput(out string, err error) string {
	if err != nil && ctxDone(err) {
		return "  (verification timed out)"
	}
	if strings.TrimSpace(out) == "" && err != nil {
		return "  " + err.Error()
	}
	var b strings.Builder
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func ctxDone(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

// runVerifyFixUntilPass runs build/test checks and lets the agent fix failures until pass or max attempts.
func (a *Agent) runVerifyFixUntilPass(ctx context.Context, msgCh chan<- tea.Msg, projectDir string) bool {
	projectDir = strings.TrimSpace(projectDir)
	if projectDir == "" {
		return true
	}

	for attempt := 1; attempt <= maxVerifyFixAttempts; attempt++ {
		summary, passed := VerifyProjectWorkspace(ctx, projectDir)
		if summary != "" {
			msgCh <- StreamChunkMsg{
				Content: fmt.Sprintf("Verification (attempt %d/%d):\n%s", attempt, maxVerifyFixAttempts, summary),
				Done:    true,
			}
			a.session.AddWithState("system", "[VERIFICATION attempt "+fmt.Sprint(attempt)+"]\n"+summary, StateExecuting.String(), 0)
		}
		if passed {
			return true
		}
		if attempt >= maxVerifyFixAttempts {
			break
		}

		a.session.AddWithState("system", defaultVerifyFixPrompt+"\n\n"+summary, StateThinking.String(), 0)
		msgCh <- StreamChunkMsg{Content: "Verification failed — analyzing and fixing issues…", Done: true}

		if err := a.agentLoop(ctx, msgCh, true); err != nil {
			return false
		}
		if a.state == StateError {
			return false
		}
	}
	return false
}
