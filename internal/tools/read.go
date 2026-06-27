package tools

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
)

// ── list_dir ──

type listDirTool struct{}

func (t *listDirTool) Name() string        { return "list_dir" }
func (t *listDirTool) Description() string { return "List the contents of a directory." }
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
	var sb strings.Builder
	for _, e := range entries {
		prefix := ""
		if e.IsDir() {
			prefix = "d "
		} else {
			prefix = "f "
		}
		info, _ := e.Info()
		size := ""
		if info != nil {
			size = " " + formatSize(info.Size())
		}
		sb.WriteString(prefix + e.Name() + size + "\n")
	}
	return OkResult(sb.String())
}

// ── read_file ──

type readFileTool struct{}

func (t *readFileTool) Name() string        { return "read_file" }
func (t *readFileTool) Description() string { return "Read the complete contents of a file." }
func (t *readFileTool) Level() permissions.Level { return permissions.LevelSafe }
func (t *readFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to read.",
			"required":    true,
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
	return OkResult(string(data))
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
	var results []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			if info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			results = append(results, path)
		}
		return nil
	})
	return OkResult(strings.Join(results, "\n"))
}

// ── search_code ──

type searchCodeTool struct{}

func (t *searchCodeTool) Name() string        { return "search_code" }
func (t *searchCodeTool) Description() string { return "Search code for keywords or regex patterns." }
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

	var results []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
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
	return OkResult(strings.Join(results, "\n"))
}

// ── helpers ──

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
