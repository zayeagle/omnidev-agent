package agent

import "testing"

func TestBuildConfirmPreviewWriteFile(t *testing.T) {
	p := buildConfirmPreview("write_file", map[string]interface{}{
		"path":    "main.go",
		"content": "package main\n\nfunc main() {}\n",
	})
	if p == "" || !contains(p, "main.go") || !contains(p, "+ package main") {
		t.Fatalf("unexpected preview: %q", p)
	}
}

func TestBuildConfirmPreviewEditFile(t *testing.T) {
	p := buildConfirmPreview("edit_file", map[string]interface{}{
		"path":        "a.go",
		"old_snippet": "foo",
		"new_snippet": "bar",
	})
	if !contains(p, "- foo") || !contains(p, "+ bar") {
		t.Fatalf("unexpected preview: %q", p)
	}
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
