package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
)

// ── list_dir ──

type listDirTool struct{}

func (t *listDirTool) Name() string        { return "list_dir" }
func (t *listDirTool) Description() string {
	return "List directory contents. Large directories return PARTIAL with continuation hint."
}
func (t *listDirTool) Level() permissions.Level { return permissions.LevelSafe }
func (t *listDirTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Directory path to list. Defaults to current directory.",
		},
	}
}
func (t *listDirTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	path := getStringArg(args, "path", ".")
	if IsSensitivePath(path) {
		return ErrResult("BLOCKED: listing sensitive path is not allowed")
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return ErrResult(err.Error())
	}
	max := effectiveLimits().ListDirMaxEntries
	var sb strings.Builder
	truncated := false
	for i, e := range entries {
		if max > 0 && i >= max {
			truncated = true
			break
		}
		prefix := "f "
		if e.IsDir() {
			prefix = "d "
		}
		info, _ := e.Info()
		size := ""
		if info != nil {
			size = " " + formatSize(info.Size())
		}
		sb.WriteString(prefix + e.Name() + size + "\n")
	}
	out := sb.String()
	if truncated {
		out += fmt.Sprintf("\n[PARTIAL list_dir: %d+ entries | list_dir path=%q with narrower path or use search_file]", len(entries), path)
	}
	return okLimited("list_dir", out)
}

// ── read_file ──

type readFileTool struct{}

func (t *readFileTool) Name() string { return "read_file" }
func (t *readFileTool) Description() string {
	return "Read a file. Use offset (1-based line) and limit (max lines) to paginate. Default limit is 300 lines when omitted."
}
func (t *readFileTool) Level() permissions.Level { return permissions.LevelSafe }
func (t *readFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to read.",
			"required":    true,
		},
		"offset": map[string]interface{}{
			"type":        "integer",
			"description": "1-based line number to start reading. Default 1.",
		},
		"limit": map[string]interface{}{
			"type":        "integer",
			"description": "Maximum lines to read from offset. Default: all remaining lines.",
		},
	}
}
func (t *readFileTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	path := getStringArg(args, "path", "")
	if path == "" {
		return ErrResult("path is required")
	}
	if IsSensitivePath(path) {
		return ErrResult("BLOCKED: reading sensitive path is not allowed")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ErrResult(err.Error())
	}
	offset := getIntArg(args, "offset", 1)
	limit := getIntArg(args, "limit", 0)
	if limit <= 0 {
		limit = effectiveLimits().ReadFileDefaultLimit
	}
	slice, total := readFileSlice(data, offset, limit)
	if slice == "" && total > 0 && offset > total {
		return ErrResult(fmt.Sprintf("offset %d beyond file (%d lines)", offset, total))
	}
	abs, _ := filepath.Abs(path)
	return okLimitedFile("read_file", slice, abs, offset, limit, total)
}

// ── search_file ──

type searchFileTool struct{}

func (t *searchFileTool) Name() string        { return "search_file" }
func (t *searchFileTool) Description() string { return "Search for files by name pattern (glob-style or substring)." }
func (t *searchFileTool) Level() permissions.Level { return permissions.LevelSafe }
func (t *searchFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"pattern": map[string]interface{}{
			"type":        "string",
			"description": "Pattern to match against file names. Supports * wildcard.",
		},
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Directory to search in. Defaults to current directory.",
		},
	}
}
func (t *searchFileTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	pattern := getStringArg(args, "pattern", "*")
	root := getStringArg(args, "path", ".")
	max := effectiveLimits().SearchMaxLines
	var results []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			if max > 0 && len(results) >= max {
				return filepath.SkipAll
			}
			results = append(results, path)
		}
		return nil
	})
	out := strings.Join(results, "\n")
	if max > 0 && len(results) >= max {
		out += fmt.Sprintf("\n[PARTIAL search_file: first %d matches | narrow pattern or path]", max)
	}
	return okLimited("search_file", out)
}

// ── search_code ──

type searchCodeTool struct{}

func (t *searchCodeTool) Name() string { return "search_code" }
func (t *searchCodeTool) Description() string {
	return "Search code for keywords or regex. Match count is capped; full results spooled when large."
}
func (t *searchCodeTool) Level() permissions.Level { return permissions.LevelSafe }
func (t *searchCodeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"query": map[string]interface{}{
			"type":        "string",
			"description": "Keyword or regex pattern to search for.",
			"required":    true,
		},
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Directory or file to search in. Defaults to current directory.",
		},
	}
}
func (t *searchCodeTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	query := getStringArg(args, "query", "")
	if query == "" {
		return ErrResult("query is required")
	}
	root := getStringArg(args, "path", ".")

	re, err := regexp.Compile(query)
	isRegex := (err == nil)

	maxLines := effectiveLimits().SearchMaxLines
	var out string
	if s, ok := searchCodeWithRipgrep(ctx, query, root, isRegex, maxLines); ok {
		out = s
	} else {
		out = searchCodeWalk(query, root, isRegex, re, maxLines)
	}
	if maxLines > 0 && strings.Count(out, "\n") >= maxLines {
		out += fmt.Sprintf("\n[PARTIAL search_code: capped at %d lines | narrow query or path]", maxLines)
	}
	return okLimited("search_code", out)
}

func searchCodeWithRipgrep(ctx context.Context, query, root string, isRegex bool, maxLines int) (string, bool) {
	if _, err := exec.LookPath("rg"); err != nil {
		return "", false
	}
	args := []string{
		"--no-heading", "--line-number", "--color=never",
		"--glob", "!.git/*", "--glob", "!node_modules/*", "--glob", "!vendor/*",
	}
	if maxLines > 0 {
		args = append(args, "--max-count", strconv.Itoa(maxLines))
	}
	if isRegex {
		args = append(args, "-e", query)
	} else {
		args = append(args, "-F", query)
	}
	args = append(args, root)

	cmd := exec.CommandContext(ctx, "rg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
			return "", true
		}
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

func searchCodeWalk(query, root string, isRegex bool, re *regexp.Regexp, maxLines int) string {
	var results []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if maxLines > 0 && len(results) >= maxLines {
			return filepath.SkipAll
		}
		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == ".idea" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if maxLines > 0 && len(results) >= maxLines {
				return filepath.SkipAll
			}
			matched := false
			if isRegex {
				matched = re.MatchString(line)
			} else {
				matched = strings.Contains(line, query)
			}
			if matched {
				results = append(results, path+":"+itoa(i+1)+": "+line)
			}
		}
		return nil
	})
	return strings.Join(results, "\n")
}

// ── helpers ──

func getIntArg(args map[string]interface{}, key string, defaultVal int) int {
	if v, ok := args[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		case string:
			if i, err := strconv.Atoi(n); err == nil {
				return i
			}
		}
	}
	return defaultVal
}

func getStringArg(args map[string]interface{}, key, defaultVal string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func formatSize(size int64) string {
	if size < 1024 {
		return itoa(int(size)) + "B"
	}
	if size < 1024*1024 {
		return itoa(int(size/1024)) + "KB"
	}
	return itoa(int(size/(1024*1024))) + "MB"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
