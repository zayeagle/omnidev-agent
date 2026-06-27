package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendTurnLogFlatFormat(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	now := time.Now().Truncate(time.Second)
	date := now.Format("20060102")

	entries := []Entry{
		{Timestamp: now, Role: "user", Content: "hello"},
		{
			Timestamp: now.Add(time.Second),
			Role:      "assistant",
			Content:   "hi there",
			State:     "Done",
			ToolCalls: []ToolCallEntry{
				{Name: "read_file", Result: "ok.txt", Allowed: true},
			},
		},
	}
	if err := store.AppendTurnLog("turn-1", entries); err != nil {
		t.Fatal(err)
	}

	jsonlPath := filepath.Join(dir, date+"-session.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 jsonl lines, got %d: %s", len(lines), string(data))
	}
	var first flatLogEntry
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.Role != "user" || first.Content != "hello" {
		t.Fatalf("unexpected first line: %+v", first)
	}
	var second flatLogEntry
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatal(err)
	}
	if len(second.ToolCalls) != 1 || second.ToolCalls[0].Name != "read_file" {
		t.Fatalf("unexpected tool_calls: %+v", second)
	}

	// Second append must extend the same files.
	entries2 := []Entry{{Timestamp: now.Add(2 * time.Second), Role: "user", Content: "follow-up"}}
	if err := store.AppendTurnLog("turn-2", entries2); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	lines = strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 jsonl lines after append, got %d", len(lines))
	}

	mdPath := filepath.Join(dir, date+"-session.md")
	md, err := os.ReadFile(mdPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(md), "# Session "+date) {
		t.Fatalf("md missing header: %q", string(md[:min(40, len(md))]))
	}
	if !strings.Contains(string(md), "**User**") || !strings.Contains(string(md), "**Assistant**") {
		t.Fatal("md missing role headers")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
