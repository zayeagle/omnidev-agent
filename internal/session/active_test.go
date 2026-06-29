package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveActiveLoadActiveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	sess := New()
	sess.AddWithState("user", "build snake game", "Thinking", 0)
	sess.AddWithState("assistant", "Sure, I'll build it.", "Done", 0)
	sess.UI = &PersistedUI{TurnCount: 1, OutputDir: "deliverables/snake-game"}

	if err := store.SaveActive(sess); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ActiveFilename)); err != nil {
		t.Fatalf("active file: %v", err)
	}

	loaded, err := store.LoadActive()
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("expected active session")
	}
	if loaded.ID != sess.ID {
		t.Fatalf("id = %q, want %q", loaded.ID, sess.ID)
	}
	if loaded.Count() != 2 {
		t.Fatalf("entries = %d, want 2", loaded.Count())
	}
	if loaded.UI == nil || loaded.UI.OutputDir != "deliverables/snake-game" {
		t.Fatalf("ui snapshot missing: %+v", loaded.UI)
	}
}

func TestArchiveClearsActive(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	sess := New()
	sess.AddWithState("user", "hello", "Thinking", 0)

	if err := store.SaveActive(sess); err != nil {
		t.Fatal(err)
	}
	if err := store.Archive(sess); err != nil {
		t.Fatal(err)
	}
	if store.HasActive() {
		t.Fatal("_active.json should be removed after archive")
	}
	archived := filepath.Join(dir, sess.ID+".json")
	if _, err := os.Stat(archived); err != nil {
		t.Fatalf("archived session: %v", err)
	}
	md := filepath.Join(dir, sess.ID+".md")
	if _, err := os.Stat(md); err != nil {
		t.Fatalf("archived markdown: %v", err)
	}
}

func TestLoadActiveMissingReturnsNil(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	loaded, err := store.LoadActive()
	if err != nil {
		t.Fatal(err)
	}
	if loaded != nil {
		t.Fatalf("expected nil, got %+v", loaded)
	}
}
