package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tui"
)

func runTUI(a *agent.Agent, cfg *config.Config, guard *agent.ProjectAwarenessGuard, store *session.Store) {
	var opts []tea.ProgramOption
	// Default: inline mode so the terminal can select/copy text. Set OMNIDEV_ALT_SCREEN=1 for full-screen.
	if os.Getenv("OMNIDEV_ALT_SCREEN") == "1" {
		opts = append(opts, tea.WithAltScreen())
	}
	if os.Getenv("OMNIDEV_NO_MOUSE") != "1" {
		opts = append(opts, tea.WithMouseCellMotion())
	}

	p := tea.NewProgram(tui.New(a, cfg, guard, store, appVersion, buildTime), opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "💡 TUI failed to start. Use headless mode:")
		fmt.Fprintln(os.Stderr, "       omnidev-agent -p \"your task here\"")
		os.Exit(1)
	}
}
