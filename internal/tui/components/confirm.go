package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	dialogBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#52525B")).
				Padding(0, 2).
				Margin(0, 1)
	dialogTitleWarn  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
	dialogTitleInfo  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#60A5FA"))
	dialogTitlePlan  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4ADE80"))
	dialogLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#71717A"))
	dialogValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#E4E4E7"))
	dialogCmdStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))
	dialogHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#71717A")).Italic(true)
	dialogKeyLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("#A1A1AA"))
	dialogKeyApprove = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#4ADE80"))
	dialogKeyDeny    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F87171"))
	dialogKeyAlt     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#60A5FA"))
	dialogOverlayBG  = lipgloss.NewStyle().Background(lipgloss.Color("#0C0C0E"))
)

// ConfirmDialogHeight is a conservative fallback when overlay height is not measured.
const ConfirmDialogHeight = 10

func renderDialogKey(key, label string, keyStyle lipgloss.Style) string {
	return keyStyle.Render("["+key+"]") + dialogKeyLabel.Render(" "+label)
}

func renderDialogActions(actions ...string) string {
	return strings.Join(actions, dialogKeyLabel.Render("  "))
}

func levelStyle(level string) lipgloss.Style {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "dangerous", "danger":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	case "safe", "read":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#4ADE80"))
	default:
		return dialogValueStyle
	}
}

func writeLabelValue(b *strings.Builder, label, value string, valueStyle lipgloss.Style) {
	b.WriteString(dialogLabelStyle.Render(fmt.Sprintf("%-9s", label)))
	b.WriteString(valueStyle.Render(value))
	b.WriteString("\n")
}

func writeWrappedField(b *strings.Builder, label string, lines []string, lineStyle lipgloss.Style) {
	if len(lines) == 0 {
		return
	}
	b.WriteString(dialogLabelStyle.Render(fmt.Sprintf("%-9s", label)))
	b.WriteString(lineStyle.Render(lines[0]))
	b.WriteString("\n")
	pad := strings.Repeat(" ", 9)
	for _, line := range lines[1:] {
		b.WriteString(pad)
		b.WriteString(lineStyle.Render(line))
		b.WriteString("\n")
	}
}

// ConfirmDialog renders a permission approval overlay, centered for the given width.
func ConfirmDialog(width int, level, description, preview string, remainingSec int) string {
	if width < 40 {
		width = 40
	}
	innerW := width - 10
	if innerW < 28 {
		innerW = 28
	}

	tool, detail := SplitToolDescription(description)
	if tool == "" {
		tool = "command"
	}

	var content strings.Builder
	content.WriteString(dialogTitleWarn.Render("Approval required"))
	content.WriteString("\n\n")

	writeLabelValue(&content, "Level", level, levelStyle(level))
	writeLabelValue(&content, "Tool", tool, dialogValueStyle)

	cmdLines := wrapCommandLines(detail, innerW-9, 4)
	writeWrappedField(&content, "Command", cmdLines, dialogCmdStyle)

	if strings.TrimSpace(preview) != "" {
		content.WriteString("\n")
		prevLines := wrapCommandLines(preview, innerW-9, 6)
		writeWrappedField(&content, "Preview", prevLines, dialogCmdStyle)
	}

	content.WriteString("\n")
	content.WriteString(renderDialogActions(
		renderDialogKey("Y", "Approve", dialogKeyApprove),
		renderDialogKey("N", "Deny", dialogKeyDeny),
		renderDialogKey("A", "Allow all", dialogKeyAlt),
	))
	content.WriteString("\n")

	if remainingSec > 0 {
		content.WriteString(dialogHintStyle.Render(fmt.Sprintf("auto-deny in %ds", remainingSec)))
	}

	dialog := dialogBorderStyle.Render(content.String())
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, dialog)
}

// CheckpointDialog renders resume-or-restart prompt for in-progress checkpoints.
func CheckpointDialog(width int, phase string, completed, total int) string {
	if width < 40 {
		width = 40
	}
	var content strings.Builder
	content.WriteString(dialogTitleInfo.Render("Checkpoint found"))
	content.WriteString("\n\n")
	writeLabelValue(&content, "Phase", phase, dialogValueStyle)
	writeLabelValue(&content, "Progress", fmt.Sprintf("%d/%d tasks done", completed, total), dialogValueStyle)
	content.WriteString("\n")
	content.WriteString(renderDialogActions(
		renderDialogKey("Y", "Resume", dialogKeyApprove),
		renderDialogKey("N", "Start fresh", dialogKeyDeny),
	))
	content.WriteString("\n")
	dialog := dialogBorderStyle.Render(content.String())
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, dialog)
}

// PlanConfirmDialog renders approve/cancel prompt for a decomposed task plan.
func PlanConfirmDialog(width int, taskCount int) string {
	if width < 40 {
		width = 40
	}
	var content strings.Builder
	content.WriteString(dialogTitlePlan.Render("Task plan ready"))
	content.WriteString("\n\n")
	writeLabelValue(&content, "Tasks", fmt.Sprintf("%d sub-tasks — review the list above", taskCount), dialogValueStyle)
	content.WriteString("\n")
	content.WriteString(renderDialogActions(
		renderDialogKey("Enter", "Execute", dialogKeyApprove),
		renderDialogKey("Esc", "Cancel", dialogKeyDeny),
	))
	content.WriteString("\n")
	dialog := dialogBorderStyle.Render(content.String())
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
		out = append(out, dialogOverlayBG.Render(line+strings.Repeat(" ", pad)))
	}
	return strings.Join(out, "\n")
}
