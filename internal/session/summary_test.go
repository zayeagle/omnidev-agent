package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSummarizeSessionFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260627-120000.json")
	payload := `{
		"id": "20260627-120000",
		"entries": [
			{"role":"user","content":"Build a snake game in Go","timestamp":"2026-06-27T12:00:00Z"},
			{"role":"assistant","content":"I'll create the game.","assistant_tool_calls":[{"id":"1","name":"write_file"}]},
			{"role":"tool","content":"ok","tool_calls":[{"name":"write_file","result":"wrote main.go"}]}
		]
	}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	sum, err := SummarizeSessionFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sum.FirstPrompt != "Build a snake game in Go" {
		t.Fatalf("first prompt: %q", sum.FirstPrompt)
	}
	if sum.EntryCount != 3 || sum.UserTurns != 1 || sum.ToolCalls != 2 {
		t.Fatalf("counts: entries=%d users=%d tools=%d", sum.EntryCount, sum.UserTurns, sum.ToolCalls)
	}
}

func TestListSessionSummaries(t *testing.T) {
	dir := t.TempDir()
	write := func(name, user string) {
		path := filepath.Join(dir, name+".json")
		os.WriteFile(path, []byte(`{"id":"`+name+`","entries":[{"role":"user","content":"`+user+`"}]}`), 0o644)
	}
	write("20260627-100000", "task A")
	time.Sleep(10 * time.Millisecond)
	write("20260627-110000", "task B")

	list, err := ListSessionSummaries(dir, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(list))
	}
	if list[0].FirstPrompt != "task B" {
		t.Fatalf("newest first: got %q", list[0].FirstPrompt)
	}
}

func TestLoadSessionDetail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20260627-120000.json")
	os.WriteFile(path, []byte(`{"id":"20260627-120000","entries":[
		{"role":"user","content":"hello"},
		{"role":"assistant","content":"hi there"}
	]}`), 0o644)

	out, err := LoadSessionDetail(dir, "20260627-120000", 4)
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(out, "hello") || !containsStr(out, "hi there") {
		t.Fatalf("detail missing exchanges: %s", out)
	}
}

func containsStr(s, sub string) bool {
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
