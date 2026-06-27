package agent

import (
	"context"
	"runtime"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

// ProjectLayout describes how much structure a new project should use.
type ProjectLayout string

const (
	// LayoutMinimal — tiny programs (calculator, CLI, single-file demos).
	// No DDD scaffold; prefer one file or a handful of files.
	LayoutMinimal ProjectLayout = "minimal"

	// LayoutDDD — layered architecture for full-stack or substantial HTTP services.
	LayoutDDD ProjectLayout = "ddd"
)

const complexityPrompt = `Classify how much project structure is needed for this development request.

Categories (reply with ONLY one word):

1. minimal — small standalone program: calculator, script, CLI tool, hello-world demo, or a simple HTTP handler that fits in one file. Use the smallest correct solution (often a single file).

2. ddd — multi-layer application worth domain-driven design: frontend + backend together, or a substantial HTTP/REST backend service with multiple concerns (domain logic, persistence, HTTP handlers) that should live in separate layers (domain, application, infrastructure, interfaces).

Examples:
- "build a calculator" → minimal
- "create a todo REST API with React frontend" → ddd
- "implement user auth microservice with database" → ddd
- "write a hello world Go program" → minimal`

// ComplexityClassifier uses an LLM call to decide minimal vs DDD layout.
type ComplexityClassifier struct {
	provider llm.Provider
}

// NewComplexityClassifier creates a classifier using the given LLM provider.
func NewComplexityClassifier(provider llm.Provider) *ComplexityClassifier {
	return &ComplexityClassifier{provider: provider}
}

// Classify returns the recommended project layout for a new workspace.
func (c *ComplexityClassifier) Classify(ctx context.Context, instruction string) ProjectLayout {
	messages := []llm.Message{
		{Role: "system", Content: complexityPrompt},
		{Role: "user", Content: instruction},
	}

	resp, err := stream.RetryChat(ctx, c.provider, &llm.Request{Messages: messages})
	if err != nil {
		return layoutFromHeuristic(instruction)
	}

	content := strings.TrimSpace(strings.ToLower(resp.Content))
	if strings.Contains(content, "ddd") {
		return LayoutDDD
	}
	if strings.Contains(content, "minimal") {
		return LayoutMinimal
	}
	return layoutFromHeuristic(instruction)
}

// layoutFromHeuristic provides a fast offline fallback when the LLM is unavailable.
func layoutFromHeuristic(instruction string) ProjectLayout {
	lower := strings.ToLower(instruction)

	dddSignals := []string{
		"frontend", "backend", "full stack", "fullstack", "full-stack",
		"web app", "webapp", "react", "vue", "angular", "前后端", "前端", "后端",
		"rest api", "http api", "microservice", "微服务",
	}
	for _, s := range dddSignals {
		if strings.Contains(lower, s) {
			return LayoutDDD
		}
	}

	minimalSignals := []string{
		"calculator", "计算器", "single file", "one file", "hello world",
		"script", "cli tool", "command line",
	}
	for _, s := range minimalSignals {
		if strings.Contains(lower, s) {
			return LayoutMinimal
		}
	}

	// Simple HTTP-only demos without a frontend stay minimal.
	if strings.Contains(lower, "http") || strings.Contains(lower, "server") {
		return LayoutMinimal
	}

	return LayoutMinimal
}

func layoutGuidance(layout ProjectLayout) string {
	switch layout {
	case LayoutDDD:
		return `PROJECT LAYOUT: DDD (layered architecture).
Organize code under cmd/, internal/domain/, internal/application/, internal/infrastructure/, and internal/interfaces/.
Use this for frontend+backend apps or substantial HTTP backend services with separated concerns.`
	default:
		return `PROJECT LAYOUT: minimal.
Use the smallest correct solution — often a single file (e.g. main.go) or at most 2–3 files in the workspace.
Do NOT create DDD layer directories (domain/application/infrastructure/interfaces) unless the user explicitly asks for them.
When the deliverable is a runnable Go program or game, finish by running: go build -o <binary-name> .
Interactive terminal programs must work on the current OS without refusing to run (no "Windows not supported" exits).` + platformGuidance()
	}
}

func platformGuidance() string {
	switch runtime.GOOS {
	case "windows":
		return `

PLATFORM (Windows): Do not use Unix-only APIs (stty, /dev/tty) and do not exit early on Windows. Enable ENABLE_VIRTUAL_TERMINAL_PROCESSING for ANSI output and put stdin in raw/console mode via syscall/kernel32 or golang.org/x/term. After coding, run go build -o <name> . — the binary must run directly as .\\<name>.exe in PowerShell or cmd.`
	default:
		return ""
	}
}
