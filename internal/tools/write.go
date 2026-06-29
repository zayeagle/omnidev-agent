package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
)

// ── write_file ──

type writeFileTool struct{}

func (t *writeFileTool) Name() string        { return "write_file" }
func (t *writeFileTool) Description() string { return "Create a new file or overwrite an existing file with the given content." }
func (t *writeFileTool) Level() permissions.Level { return permissions.LevelDangerous }
func (t *writeFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to write.",
			"required":    true,
		},
		"content": map[string]interface{}{
			"type":        "string",
			"description": "Content to write to the file.",
			"required":    true,
		},
	}
}
func (t *writeFileTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	path := getStringArg(args, "path", "")
	if path == "" {
		return ErrResult("path is required")
	}
	content := getStringArg(args, "content", "")

	var oldContent string
	if data, err := os.ReadFile(path); err == nil {
		oldContent = string(data)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return ErrResult(err.Error())
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return ErrResult(err.Error())
	}
	added, removed := FileLineChange(oldContent, content)
	verb := "wrote"
	if oldContent != "" {
		verb = "updated"
	}
	return OkResult(FormatChange(verb, path, added, removed))
}

// ── edit_file ──

type editFileTool struct{}

func (t *editFileTool) Name() string        { return "edit_file" }
func (t *editFileTool) Description() string {
	return "Replace a snippet of text in a file. Finds old_snippet and replaces with new_snippet."
}
func (t *editFileTool) Level() permissions.Level { return permissions.LevelDangerous }
func (t *editFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file to edit.",
			"required":    true,
		},
		"old_snippet": map[string]interface{}{
			"type":        "string",
			"description": "The exact text to find and replace.",
			"required":    true,
		},
		"new_snippet": map[string]interface{}{
			"type":        "string",
			"description": "The replacement text.",
			"required":    true,
		},
	}
}
func (t *editFileTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	path := getStringArg(args, "path", "")
	oldSnippet := getStringArg(args, "old_snippet", "")
	newSnippet := getStringArg(args, "new_snippet", "")
	if path == "" {
		return ErrResult("path is required")
	}
	if oldSnippet == "" {
		return ErrResult("old_snippet is required")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ErrResult(err.Error())
	}
	content := string(data)
	if !strings.Contains(content, oldSnippet) {
		return ErrResult("old_snippet not found in file")
	}
	newContent := strings.Replace(content, oldSnippet, newSnippet, 1)
	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return ErrResult(err.Error())
	}
	added, removed := SnippetLineChange(oldSnippet, newSnippet)
	return OkResult(FormatChange("edited", path, added, removed))
}
