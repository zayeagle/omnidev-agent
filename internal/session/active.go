package session

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const ActiveFilename = "_active.json"

// Load reads a session snapshot from path.
func Load(path string) (*Session, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var sess Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// LoadActive returns the in-progress session, or nil if none exists.
func (s *Store) LoadActive() (*Session, error) {
	path := filepath.Join(s.baseDir, ActiveFilename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return Load(path)
}

// SaveActive persists the current working session (not archived).
func (s *Store) SaveActive(session *Session) error {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.baseDir, ActiveFilename), data, 0644)
}

// Archive writes the session to {id}.json, exports Markdown, and clears _active.json.
func (s *Store) Archive(session *Session) error {
	if err := s.Save(session); err != nil {
		return err
	}
	if err := s.Export(session); err != nil {
		return err
	}
	activePath := filepath.Join(s.baseDir, ActiveFilename)
	if err := os.Remove(activePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HasActive reports whether an unarchived session file exists.
func (s *Store) HasActive() bool {
	path := filepath.Join(s.baseDir, ActiveFilename)
	_, err := os.Stat(path)
	return err == nil
}

// ClearActive removes the in-progress session file without archiving.
func (s *Store) ClearActive() error {
	path := filepath.Join(s.baseDir, ActiveFilename)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
