package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

// TestGuardLegacyProjectBlocksWrites verifies that on a legacy project,
// destructive tool calls are blocked until the awareness scan completes.
func TestGuardLegacyProjectBlocksWrites(t *testing.T) {
	// Create a temporary legacy project
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "lib.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "util.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test Project"), 0644)

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	sess := session.New()

	guard := agent.NewProjectAwarenessGuard(toolbox, sess, tmpDir)

	// Without scan, destructive writes should NOT be allowed
	t.Run("blocks write without scan", func(t *testing.T) {
		if guard.Allow("write_file") {
			t.Error("expected Allow(write_file) to return false before scan on legacy project")
		}
		if guard.Allow("edit_file") {
			t.Error("expected Allow(edit_file) to return false before scan on legacy project")
		}
		if guard.Allow("delete_file") {
			t.Error("expected Allow(delete_file) to return false before scan on legacy project")
		}
	})

	// Safe reads should always pass
	t.Run("allows read operations", func(t *testing.T) {
		if !guard.Allow("read_file") {
			t.Error("expected Allow(read_file) to return true even without scan")
		}
		if !guard.Allow("list_dir") {
			t.Error("expected Allow(list_dir) to return true even without scan")
		}
	})

	// After scan, writes should be allowed
	t.Run("allows write after scan", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		guard.RunScan(ctx)

		if !guard.IsAwarenessComplete() {
			t.Error("expected awareness complete after scan")
		}
		if !guard.Allow("write_file") {
			t.Error("expected Allow(write_file) to return true after scan")
		}
	})

	// Verify project analysis was injected into session
	t.Run("analysis injected into session", func(t *testing.T) {
		foundAnalysis := false
		for _, e := range sess.Entries {
			if e.State == "analyzed" {
				foundAnalysis = true
				break
			}
		}
		if !foundAnalysis {
			t.Error("expected [PROJECT ANALYSIS] entry in session after scan")
		}
	})
}

// TestGuardGreenfieldSkipsScan verifies that greenfield projects skip the scan.
func TestGuardGreenfieldSkipsScan(t *testing.T) {
	tmpDir := t.TempDir()

	toolbox := tools.NewRegistry()
	tools.RegisterAll(toolbox)
	sess := session.New()

	guard := agent.NewProjectAwarenessGuard(toolbox, sess, tmpDir)

	if !guard.IsAwarenessComplete() {
		t.Error("expected greenfield project to immediately complete awareness")
	}
	if !guard.Allow("write_file") {
		t.Error("expected write allowed on greenfield project")
	}
}

// TestIsDestructive verifies the destructive tool classification.
func TestIsDestructive(t *testing.T) {
	tests := []struct {
		name     string
		level    permissions.Level
		expected bool
	}{
		{"write_file", permissions.LevelDangerous, true},
		{"edit_file", permissions.LevelDangerous, true},
		{"delete_file", permissions.LevelDangerous, true},
		{"read_file", permissions.LevelSafe, false},
		{"list_dir", permissions.LevelSafe, false},
		{"shell_exec", permissions.LevelDangerous, false}, // shell is dangerous but not destructive write
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := agent.IsDestructive(tt.name, tt.level)
			if result != tt.expected {
				t.Errorf("IsDestructive(%s, %v) = %v, want %v", tt.name, tt.level, result, tt.expected)
			}
		})
	}
}
