package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Confirm colors
var (
	confirmBorder     = lipgloss.NewStyle().Border(lipgloss.DoubleBorder()).BorderForeground(lipgloss.Color("#F87171")).Padding(1, 2)
	confirmTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
	confirmDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F9FAFB"))
	confirmApproveKey = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#34D399"))
	confirmDenyKey    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
	confirmOverlayBG  = lipgloss.NewStyle().Background(lipgloss.Color("#000000"))
)

// ConfirmDialogHeight is the terminal rows reserved while the permission overlay is shown.
const ConfirmDialogHeight = 8

// ConfirmDialog renders a permission approval overlay, centered for the given width.
func ConfirmDialog(width int, level, description string, remainingSec int) string {
	if width < 40 {
		width = 40
	}
	innerW := width - 8
	if innerW < 28 {
		innerW = 28
	}

	var content strings.Builder

	title := confirmTitleStyle.Render("⚠ Permission Required")
	content.WriteString(title + "\n\n")

	content.WriteString(confirmDescStyle.Render(fmt.Sprintf("  Level:   %s", level)) + "\n")
	descLines := WrapDisplayWidth(description, innerW-2)
	for i, line := range descLines {
		prefix := "  Command: "
		if i > 0 {
			prefix = "           "
		}
		content.WriteString(confirmDescStyle.Render(prefix+line) + "\n")
	}
	content.WriteString("\n")

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
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, dialog)
}

// ConfirmOverlay wraps the dialog with a full-width dark background without stretching borders.
func ConfirmOverlay(width int, dialog string) string {
	if width < 1 {
		width = 80
	}
	block := lipgloss.NewStyle().Padding(1, 0).Render(dialog)
	lines := strings.Split(block, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		pad := width - lipgloss.Width(line)
		if pad < 0 {
			pad = 0
		}
		out = append(out, confirmOverlayBG.Render(line+strings.Repeat(" ", pad)))
	}
	return strings.Join(out, "\n")
}
