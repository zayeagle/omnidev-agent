package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// exportPlainScreen returns the full visible transcript as plain text (no ANSI).
func (m *model) exportPlainScreen() string {
	w := effectiveWidth(m.width)
	m.turns.SetWidth(w)
	ex := m.scrollExtras()
	all := m.turns.CombinedLines(ex.prefix, ex.suffix)

	var b strings.Builder
	for _, block := range []string{
		components.AgentHeader(m.headerInfo(), w),
		strings.Join(all, "\n"),
	} {
		block = strings.TrimRight(block, "\n")
		if block == "" {
			continue
		}
		for _, line := range strings.Split(block, "\n") {
			b.WriteString(components.StripANSI(line))
			b.WriteByte('\n')
		}
	}
	if hint := components.FooterExitHint(w, m.isInSession()); hint != "" {
		for _, line := range strings.Split(hint, "\n") {
			b.WriteString(components.StripANSI(line))
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *model) copyScreen() {
	text := m.exportPlainScreen()
	if text == "" {
		return
	}
	path := m.writeExportFile(text)
	clipErr := copyToSystemClipboard(text)
	t := m.newTurn("/copy")
	switch {
	case clipErr == nil && path != "":
		t.SetCommandOutput(fmt.Sprintf("Copied %d lines to clipboard.\nAlso saved: %s", strings.Count(text, "\n")+1, path))
	case clipErr == nil:
		t.SetCommandOutput(fmt.Sprintf("Copied %d lines to clipboard.", strings.Count(text, "\n")+1))
	case path != "":
		t.SetCommandOutput(fmt.Sprintf("Clipboard unavailable (%v).\nPlain text saved: %s", clipErr, path))
	default:
		t.SetCommandOutput("Copy failed: " + clipErr.Error())
	}
}

func (m *model) writeExportFile(text string) string {
	dir := m.cfg.RuntimeSessionDir()
	if dir == "" {
		dir = ".ai_history/sessions"
	}
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "last-screen.txt")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return ""
	}
	return path
}

func copyToSystemClipboard(text string) error {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("clip")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if _, err := exec.LookPath("pbcopy"); err == nil {
		cmd := exec.Command("pbcopy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		cmd := exec.Command("xclip", "-selection", "clipboard")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}
	return fmt.Errorf("no clipboard tool found")
}
