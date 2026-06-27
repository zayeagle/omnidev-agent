package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// flatLogEntry is one line in YYYYMMDD-session.jsonl (matches existing Codex/Cursor logs).
type flatLogEntry struct {
	Timestamp time.Time      `json:"timestamp"`
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []flatToolCall `json:"tool_calls,omitempty"`
}

type flatToolCall struct {
	Name    string `json:"name"`
	Summary string `json:"summary,omitempty"`
	Allowed bool   `json:"allowed,omitempty"`
}

// AppendTurnLog appends entries to YYYYMMDD-session.jsonl and YYYYMMDD-session.md.
// Intended for external dev assistants (Cursor, Codex) collaborating on omnidev-agent;
// the omnidev-agent binary itself does not call this.
func (s *Store) AppendTurnLog(_ string, entries []Entry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(s.baseDir, 0o755); err != nil {
		return err
	}

	date := time.Now().Format("20060102")
	jsonlPath := filepath.Join(s.baseDir, date+"-session.jsonl")
	mdPath := filepath.Join(s.baseDir, date+"-session.md")

	for _, e := range entries {
		line, err := json.Marshal(entryToFlat(e))
		if err != nil {
			return err
		}
		if err := appendFileLine(jsonlPath, string(line)); err != nil {
			return err
		}
	}

	mdNew, err := isNewOrEmpty(mdPath)
	if err != nil {
		return err
	}
	var md strings.Builder
	if mdNew {
		md.WriteString("# Session " + date + "\n")
	}
	for _, e := range entries {
		md.WriteString(entryToMarkdown(e))
	}
	if err := appendFileBytes(mdPath, []byte(md.String())); err != nil {
		return err
	}
	return nil
}

func entryToFlat(e Entry) flatLogEntry {
	out := flatLogEntry{
		Timestamp: e.Timestamp,
		Role:      e.Role,
		Content:   e.Content,
	}
	if e.Timestamp.IsZero() {
		out.Timestamp = time.Now()
	}
	for _, tc := range e.ToolCalls {
		out.ToolCalls = append(out.ToolCalls, flatToolCall{
			Name:    tc.Name,
			Summary: toolCallSummary(tc),
			Allowed: tc.Allowed,
		})
	}
	return out
}

func toolCallSummary(tc ToolCallEntry) string {
	if tc.Error != "" {
		return tc.Error
	}
	if tc.Result != "" {
		s := strings.ReplaceAll(tc.Result, "\n", " ")
		if len(s) > 120 {
			return s[:117] + "..."
		}
		return s
	}
	if tc.Arguments != nil {
		if m, ok := tc.Arguments.(map[string]interface{}); ok {
			if path, ok := m["path"].(string); ok {
				return path
			}
			if cmd, ok := m["cmd"].(string); ok {
				if len(cmd) > 80 {
					return cmd[:77] + "..."
				}
				return cmd
			}
		}
	}
	return tc.Name
}

func entryToMarkdown(e Entry) string {
	var b strings.Builder
	b.WriteString("\n---\n\n")
	b.WriteString("**" + strings.ToUpper(e.Role[:1]) + e.Role[1:] + "**")
	if e.State != "" {
		b.WriteString(" [" + e.State + "]")
	}
	ts := e.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	b.WriteString(" — " + ts.Format(time.RFC3339) + "\n\n")
	b.WriteString(e.Content + "\n\n")
	for _, tc := range e.ToolCalls {
		b.WriteString("- ⚙ `" + tc.Name + "`")
		if summary := toolCallSummary(tc); summary != "" && summary != tc.Name {
			b.WriteString(" — " + summary)
		}
		if !tc.Allowed {
			b.WriteString(" denied")
		}
		b.WriteString("\n")
	}
	if len(e.ToolCalls) > 0 {
		b.WriteString("\n")
	}
	return b.String()
}

func appendFileLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func appendFileBytes(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func isNewOrEmpty(path string) (bool, error) {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return info.Size() == 0, nil
}
