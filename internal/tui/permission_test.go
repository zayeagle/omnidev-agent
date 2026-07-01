package tui

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

func TestTogglePermissionModeDuringSession(t *testing.T) {
	a := agent.New(nil, permissions.NewChecker(true), tools.NewRegistry(), session.New())
	m := New(a, &config.Config{}, nil, session.NewStore(t.TempDir()), "0.0.1", "test").(*model)

	if m.permissionModeLabel() != "confirm" {
		t.Fatalf("label=%q", m.permissionModeLabel())
	}
	m.togglePermissionMode()
	if m.permissionModeLabel() != "yolo" {
		t.Fatalf("label=%q after toggle", m.permissionModeLabel())
	}
	m.togglePermissionMode()
	if m.permissionModeLabel() != "confirm" {
		t.Fatalf("label=%q after second toggle", m.permissionModeLabel())
	}
}

func TestIsSessionSlashCommand(t *testing.T) {
	for _, in := range []string{"/yolo", "/help", "/Help", "help", "h"} {
		if !isSessionSlashCommand(in) {
			t.Fatalf("%q should be session command", in)
		}
	}
	if isSessionSlashCommand("hello") {
		t.Fatal("normal text should not be slash command")
	}
}
