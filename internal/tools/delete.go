package tools

import (
	"context"
	"os"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
)

// deleteFileTool removes a file or directory.
type deleteFileTool struct{}

func (t *deleteFileTool) Name() string        { return "delete_file" }
func (t *deleteFileTool) Description() string { return "Delete a file or directory." }
func (t *deleteFileTool) Level() permissions.Level { return permissions.LevelDangerous }
func (t *deleteFileTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"path": map[string]interface{}{
			"type":        "string",
			"description": "Path to the file or directory to delete.",
			"required":    true,
		},
	}
}
func (t *deleteFileTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	path := getStringArg(args, "path", "")
	if path == "" {
		return ErrResult("path is required")
	}
	var removed int
	if data, err := os.ReadFile(path); err == nil {
		removed = LineCount(string(data))
	}
	if err := os.RemoveAll(path); err != nil {
		return ErrResult(err.Error())
	}
	return OkResult(FormatChange("deleted", path, 0, removed))
}
