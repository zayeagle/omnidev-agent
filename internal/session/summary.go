package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SessionSummary is metadata for one archived runtime session (Cursor-style list row).
type SessionSummary struct {
	ID          string
	Path        string
	FirstPrompt string
	EntryCount  int
	UserTurns   int
	ToolCalls   int
	StartedAt   time.Time
	ModTime     time.Time
	Compacted   bool
}

type sessionFile struct {
	ID      string  `json:"id"`
	Entries []Entry `json:"entries"`
}

// ListSessionSummaries scans sessionsDir for .json snapshots and returns newest first.
func ListSessionSummaries(sessionsDir string, limit int) ([]SessionSummary, error) {
	if limit < 1 {
		limit = 20
	}
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []SessionSummary
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if e.Name() == ActiveFilename {
			continue
		}
		path := filepath.Join(sessionsDir, e.Name())
		sum, err := SummarizeSessionFile(path)
		if err != nil {
			continue
		}
		if info, err := e.Info(); err == nil {
			sum.ModTime = info.ModTime()
		}
		out = append(out, sum)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].ModTime.Equal(out[j].ModTime) {
			return out[i].ID > out[j].ID
		}
		return out[i].ModTime.After(out[j].ModTime)
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// SummarizeSessionFile builds a SessionSummary from a saved session JSON file.
func SummarizeSessionFile(path string) (SessionSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SessionSummary{}, err
	}
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return SessionSummary{}, err
	}
	sum := SessionSummary{
		ID:   sf.ID,
		Path: path,
	}
	if sum.ID == "" {
		sum.ID = strings.TrimSuffix(filepath.Base(path), ".json")
	}

	for _, e := range sf.Entries {
		sum.EntryCount++
		if e.Role == "user" {
			sum.UserTurns++
			if sum.FirstPrompt == "" && !strings.HasPrefix(strings.TrimSpace(e.Content), "/") {
				sum.FirstPrompt = TruncateOneLine(e.Content, 100)
			}
		}
		sum.ToolCalls += len(e.ToolCalls) + len(e.AssistantToolCalls)
		if strings.Contains(e.Content, "[EARLY CONTEXT SUMMARY]") {
			sum.Compacted = true
		}
		if sum.FirstPrompt == "" && e.Role == "user" && sum.UserTurns == 1 {
			sum.FirstPrompt = TruncateOneLine(e.Content, 100)
		}
		if !e.Timestamp.IsZero() && (sum.StartedAt.IsZero() || e.Timestamp.Before(sum.StartedAt)) {
			sum.StartedAt = e.Timestamp
		}
	}

	if sum.FirstPrompt == "" {
		sum.FirstPrompt = "(no user message)"
	}
	if sum.StartedAt.IsZero() {
		if t, err := time.Parse("20060102-150405", sum.ID); err == nil {
			sum.StartedAt = t
		}
	}
	return sum, nil
}

// FormatSessionSummaryLine renders one list row for /sessions.
func FormatSessionSummaryLine(index int, s SessionSummary) string {
	when := s.ModTime
	if when.IsZero() && !s.StartedAt.IsZero() {
		when = s.StartedAt
	}
	timeStr := when.Format("2006-01-02 15:04")
	flags := ""
	if s.Compacted {
		flags = " · compacted"
	}
	return fmt.Sprintf("  %d. %s  ·  %d msgs  ·  %d tools%s\n     › %s",
		index, timeStr, s.EntryCount, s.ToolCalls, flags, s.FirstPrompt)
}

// LoadSessionDetail returns a human-readable preview of a saved session.
func LoadSessionDetail(sessionsDir, idOrPath string, maxExchanges int) (string, error) {
	path, err := ResolveSessionPath(sessionsDir, idOrPath)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var sf sessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return "", err
	}
	sum, _ := SummarizeSessionFile(path)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Session %s\n", sum.ID))
	sb.WriteString(fmt.Sprintf("  Messages: %d  ·  User turns: %d  ·  Tool calls: %d\n",
		sum.EntryCount, sum.UserTurns, sum.ToolCalls))
	if !sum.StartedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("  Started:  %s\n", sum.StartedAt.Format(time.RFC3339)))
	}
	if sum.Compacted {
		sb.WriteString("  Context:  early history summarized\n")
	}
	sb.WriteString("\n")

	if maxExchanges < 1 {
		maxExchanges = 6
	}
	shown := 0
	for _, e := range sf.Entries {
		if e.Role != "user" && e.Role != "assistant" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(e.Content), "[EARLY CONTEXT SUMMARY]") {
			sb.WriteString("--- [context summary] ---\n")
			sb.WriteString(TruncateOneLine(strings.TrimPrefix(e.Content, "[EARLY CONTEXT SUMMARY]\n"), 400))
			sb.WriteString("\n\n")
			continue
		}
		label := strings.ToUpper(e.Role[:1]) + e.Role[1:]
		sb.WriteString(fmt.Sprintf("[%s] %s\n", label, TruncateOneLine(e.Content, 500)))
		if len(e.ToolCalls) > 0 {
			sb.WriteString(fmt.Sprintf("  (%d tool results)\n", len(e.ToolCalls)))
		}
		sb.WriteString("\n")
		shown++
		if shown >= maxExchanges {
			remaining := countUserAssistant(sf.Entries) - shown
			if remaining > 0 {
				sb.WriteString(fmt.Sprintf("… and %d more exchanges (open %s for full log)\n", remaining, path))
			}
			break
		}
	}
	return sb.String(), nil
}

// ResolveSessionPath maps an id or filename to an absolute session JSON path.
func ResolveSessionPath(sessionsDir, idOrPath string) (string, error) {
	arg := strings.TrimSpace(idOrPath)
	if arg == "" {
		return "", fmt.Errorf("empty session id")
	}
	if strings.ContainsAny(arg, `/\`) {
		if strings.HasSuffix(arg, ".json") {
			return arg, nil
		}
		return arg + ".json", nil
	}
	base := arg
	if strings.HasSuffix(base, ".md") {
		base = strings.TrimSuffix(base, ".md")
	}
	if !strings.HasSuffix(base, ".json") {
		base += ".json"
	}
	path := filepath.Join(sessionsDir, base)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("session not found: %s", base)
	}
	return path, nil
}

// TruncateOneLine collapses whitespace and limits rune length.
func TruncateOneLine(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	runes := []rune(s)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes-1]) + "…"
	}
	return s
}

func countUserAssistant(entries []Entry) int {
	n := 0
	for _, e := range entries {
		if e.Role == "user" || e.Role == "assistant" {
			n++
		}
	}
	return n
}
