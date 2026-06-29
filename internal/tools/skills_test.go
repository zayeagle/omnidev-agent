package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/skills"
)

func TestLoadSkillTool(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# Demo\n\nStep 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	SetSkillCatalog(skills.LoadCatalog([]string{dir}))

	tool := &loadSkillTool{}
	res := tool.Execute(context.Background(), map[string]interface{}{"name": "demo"})
	if !res.Success {
		t.Fatal(res.Error)
	}
	if !strings.Contains(res.Data, "[SKILL: demo]") {
		t.Fatalf("unexpected: %q", res.Data)
	}
}
