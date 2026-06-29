package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalog(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "demo")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Demo Skill\n\nDo the demo workflow."
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cat := LoadCatalog([]string{dir})
	if cat.Count() != 1 {
		t.Fatalf("count=%d want 1", cat.Count())
	}
	sk, ok := cat.Get("demo")
	if !ok || sk.Description == "" {
		t.Fatalf("skill: %+v ok=%v", sk, ok)
	}
}

func TestGetCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "MySkill")
	_ = os.MkdirAll(skillDir, 0o755)
	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# X"), 0o644)
	cat := LoadCatalog([]string{dir})
	if _, ok := cat.Get("myskill"); !ok {
		t.Fatal("expected case-insensitive lookup")
	}
}
