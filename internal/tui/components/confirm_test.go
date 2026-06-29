package components

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestConfirmDialogDoesNotExceedTerminalWidth(t *testing.T) {
	const termW = 100
	dialog := ConfirmDialog(termW, "dangerous", "write_file: deliverables/snake-game/index.html", "", 30)
	if lipgloss.Width(dialog) > termW {
		t.Fatalf("dialog width %d exceeds terminal %d", lipgloss.Width(dialog), termW)
	}
	if !strings.Contains(dialog, "Approval required") {
		t.Fatal("missing dialog title")
	}
}

func TestConfirmOverlayDoesNotStretchBorder(t *testing.T) {
	const termW = 100
	dialog := ConfirmDialog(termW, "dangerous", "write_file: test.go", "+ package main\n", 30)
	overlay := ConfirmOverlay(termW, dialog)
	for _, line := range strings.Split(overlay, "\n") {
		if lipgloss.Width(line) > termW {
			t.Fatalf("overlay line wider than terminal: %d > %d", lipgloss.Width(line), termW)
		}
	}
	// Rounded border corners should appear once per box edge, not stretched across the row.
	if strings.Count(overlay, "╮") > 1 {
		t.Fatalf("expected single top-right corner, got stretched border")
	}
}
