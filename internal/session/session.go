package session

import (
	"sync"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Entry struct {
	Timestamp time.Time       `json:"timestamp"`
	Role      string          `json:"role"`            // user | assistant | tool | system
	Content   string          `json:"content"`
	State     string          `json:"state,omitempty"` // agent state at entry time
	Tokens    int             `json:"tokens,omitempty"` // LLM token consumption
	ToolCalls            []ToolCallEntry `json:"tool_calls,omitempty"`
	AssistantToolCalls []ToolCallData     `json:"assistant_tool_calls,omitempty"`
}

// ToolCallData mirrors llm.ToolCall for session storage.
type ToolCallData struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

type ToolCallEntry struct {
	ID        string      `json:"id,omitempty"`
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
	Result    string      `json:"result,omitempty"`
	Error     string      `json:"error,omitempty"`
	Allowed   bool        `json:"allowed"`
}

type Session struct {
	mu      sync.RWMutex
	ID      string  `json:"id"`
	Entries []Entry `json:"entries"`
}

func New() *Session {
	return &Session{
		ID: time.Now().Format("20060102-150405"),
	}
}

func (s *Session) Add(entry Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Entries = append(s.Entries, entry)
}

// AddWithState is a convenience that stamps the current agent state and timestamp.
func (s *Session) AddWithState(role, content, state string, tokens int) {
	s.Add(Entry{
		Timestamp: time.Now(),
		Role:      role,
		Content:   content,
		State:     state,
		Tokens:    tokens,
	})
}

func (s *Session) LastEntry() *Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Entries) == 0 {
		return nil
	}
	return &s.Entries[len(s.Entries)-1]
}

// Count returns the total number of entries.
func (s *Session) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Entries)
}

// EntriesFrom returns a copy of entries starting at idx (inclusive).
func (s *Session) EntriesFrom(idx int) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if idx < 0 || idx >= len(s.Entries) {
		return nil
	}
	out := make([]Entry, len(s.Entries)-idx)
	copy(out, s.Entries[idx:])
	return out
}

// LastAssistantContent returns the content of the last assistant message
// searching backwards through entries under read lock. Returns empty string if none.
func (s *Session) LastAssistantContent() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for i := len(s.Entries) - 1; i >= 0; i-- {
		if s.Entries[i].Role == "assistant" {
			return s.Entries[i].Content
		}
	}
	return ""
}

type Store struct {
	baseDir string
}

func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

func (s *Store) Save(session *Session) error {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.baseDir, session.ID+".json"), data, 0644)
}

// Export writes a human-readable Markdown version of the session to baseDir.
func (s *Store) Export(session *Session) error {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}
	var md strings.Builder
	md.WriteString("# Session " + session.ID + "\n\n")
	for _, e := range session.Entries {
		md.WriteString("---\n")
		md.WriteString("**" + strings.Title(e.Role) + "**")
		if e.State != "" {
			md.WriteString(" [" + e.State + "]")
		}
		if e.Tokens > 0 {
			md.WriteString(" (tokens: " + strconv.Itoa(e.Tokens) + ")")
		}
		md.WriteString(" — " + e.Timestamp.Format(time.RFC3339) + "\n\n")
		md.WriteString(e.Content + "\n\n")
		for _, tc := range e.ToolCalls {
			md.WriteString("- ⚙ `" + tc.Name + "`")
			if !tc.Allowed {
				md.WriteString(" ❌ denied")
			}
			if tc.Error != "" {
				md.WriteString(" — error: " + tc.Error)
			}
			md.WriteString("\n")
		}
		md.WriteString("\n")
	}
	return os.WriteFile(filepath.Join(s.baseDir, session.ID+".md"), []byte(md.String()), 0644)
}

func (s *Store) List() ([]string, error) {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
