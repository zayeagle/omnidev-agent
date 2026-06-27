package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Confirm colors
var (
	confirmBorder      = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("#F87171")).Padding(1, 2)
	confirmTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
	confirmDescStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB"))
	confirmApproveKey  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399"))
	confirmDenyKey     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
	confirmOverlay     = lipgloss.NewStyle().Background(lipgloss.Color("#000000")).Foreground(lipgloss.Color("#F9FAFB"))
)

// ConfirmDialogHeight is the terminal rows reserved while the permission overlay is shown.
const ConfirmDialogHeight = 8

// ConfirmDialog renders a permission approval overlay.
// Returns the dialog string, centered for the given width.
func ConfirmDialog(width int, level, description string, remainingSec int) string {
	if width < 40 {
		width = 40
	}

	var content strings.Builder

	title := confirmTitleStyle.Render("⚠ Permission Required")
	content.WriteString(title + "\n\n")

	content.WriteString(confirmDescStyle.Render(fmt.Sprintf("  Level:   %s", level)) + "\n")
	content.WriteString(confirmDescStyle.Render(fmt.Sprintf("  Command: %s", description)) + "\n\n")

	content.WriteString("  ")
	content.WriteString(confirmApproveKey.Render("[Y] Approve"))
	content.WriteString(confirmDescStyle.Render("   "))
	content.WriteString(confirmDenyKey.Render("[N] Deny"))
	content.WriteString(confirmDescStyle.Render("   "))
	content.WriteString(confirmApproveKey.Render("[A] Allow all"))
	content.WriteString("\n")

	if remainingSec > 0 {
		content.WriteString(confirmDescStyle.Render(fmt.Sprintf("  (auto-deny in %ds)", remainingSec)))
	}

	dialog := confirmBorder.Render(content.String())
	dialogWidth := lipgloss.Width(dialog)

	// Center horizontally
	pad := (width - dialogWidth) / 2
	if pad < 0 {
		pad = 0
	}

	return strings.Repeat(" ", pad) + dialog
}

// ConfirmOverlay wraps the dialog with a full-width dark background.
func ConfirmOverlay(width int, dialog string) string {
	padded := lipgloss.NewStyle().Padding(1, 0).Render(dialog)
	return confirmOverlay.Width(width).Render(padded)
}
