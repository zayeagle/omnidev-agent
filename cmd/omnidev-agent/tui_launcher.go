package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/tui"
)

func runTUI(a *agent.Agent, cfg *config.Config, guard *agent.ProjectAwarenessGuard) {
	opts := []tea.ProgramOption{
		tea.WithAltScreen(),
	}
	if os.Getenv("OMNIDEV_MOUSE_SCROLL") == "1" {
		opts = append(opts, tea.WithMouseCellMotion())
	}

	p := tea.NewProgram(tui.New(a, cfg, guard, appVersion, buildTime), opts...)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "💡 TUI failed to start. Use headless mode:")
		fmt.Fprintln(os.Stderr, "       omnidev-agent -p \"your task here\"")
		os.Exit(1)
	}
}
