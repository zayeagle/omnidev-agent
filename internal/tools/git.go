package tools

import (
	"context"
	"os/exec"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
)

type gitStatusTool struct{}

func (t *gitStatusTool) Name() string        { return "git_status" }
func (t *gitStatusTool) Description() string { return "Show git working tree status (porcelain)." }
func (t *gitStatusTool) Level() permissions.Level {
	return permissions.LevelSafe
}
func (t *gitStatusTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Repository root (default: .).",
		},
	}
}
func (t *gitStatusTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	dir := getStringArg(args, "path", ".")
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain").CombinedOutput()
	if err != nil {
		return ErrResult(strings.TrimSpace(string(out)))
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return okLimited("git_status", "clean working tree")
	}
	return okLimited("git_status", s)
}

type gitDiffStatTool struct{}

func (t *gitDiffStatTool) Name() string        { return "git_diff_stat" }
func (t *gitDiffStatTool) Description() string { return "Show git diff --stat for unstaged and staged changes." }
func (t *gitDiffStatTool) Level() permissions.Level {
	return permissions.LevelSafe
}
func (t *gitDiffStatTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Repository root (default: .).",
		},
	}
}
func (t *gitDiffStatTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	dir := getStringArg(args, "path", ".")
	out, err := exec.CommandContext(ctx, "git", "-C", dir, "diff", "--stat", "HEAD").CombinedOutput()
	if err != nil {
		return ErrResult(strings.TrimSpace(string(out)))
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return okLimited("git_diff_stat", "no diff vs HEAD")
	}
	return okLimited("git_diff_stat", s)
}
