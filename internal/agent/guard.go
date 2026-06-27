package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

// GuardState tracks the project awareness lifecycle.
type GuardState int

const (
	GuardIdle     GuardState = 0
	GuardScanning GuardState = 1
	GuardDone     GuardState = 2
	GuardError    GuardState = 3
)

func (s GuardState) String() string {
	switch s {
	case GuardIdle:
		return "idle"
	case GuardScanning:
		return "scanning"
	case GuardDone:
		return "done"
	case GuardError:
		return "error"
	default:
		return "unknown"
	}
}

// ProjectType classifies the working directory.
type ProjectType int

const (
	ProjectGreenfield ProjectType = 0
	ProjectLegacy     ProjectType = 1
)

func (p ProjectType) String() string {
	switch p {
	case ProjectGreenfield:
		return "greenfield"
	case ProjectLegacy:
		return "legacy"
	default:
		return "unknown"
	}
}

// destructiveTools is the set of tool names that modify the filesystem.
var destructiveTools = map[string]bool{
	"write_file":  true,
	"edit_file":   true,
	"delete_file": true,
}

// ProjectAwarenessGuard intercepts destructive tool calls on legacy projects
// until a four-step project understanding scan has completed.
type ProjectAwarenessGuard struct {
	state       GuardState
	projectType ProjectType
	toolbox     *tools.Registry
	session     *session.Session
	msgCh       chan<- tea.Msg
	cwd         string
	timeout     time.Duration
	mu          sync.Mutex
}

// NewProjectAwarenessGuard creates a guard and auto-detects the project type.
func NewProjectAwarenessGuard(toolbox *tools.Registry, sess *session.Session, cwd string) *ProjectAwarenessGuard {
	g := &ProjectAwarenessGuard{
		state:       GuardIdle,
		projectType: ProjectGreenfield,
		toolbox:     toolbox,
		session:     sess,
		cwd:         cwd,
		timeout:     30 * time.Second,
	}
	g.projectType = g.detectProjectType()
	if g.projectType == ProjectGreenfield {
		g.state = GuardDone
	}
	return g
}

// SetMsgCh attaches a TUI message channel for progress reporting.
func (g *ProjectAwarenessGuard) SetMsgCh(ch chan<- tea.Msg) { g.msgCh = ch }

// State returns the current guard state.
func (g *ProjectAwarenessGuard) State() GuardState         { return g.state }
func (g *ProjectAwarenessGuard) IsAwarenessComplete() bool { return g.state == GuardDone }

func (g *ProjectAwarenessGuard) ProjectType() ProjectType  { return g.projectType }
// Allow checks whether a destructive tool call should proceed.
func (g *ProjectAwarenessGuard) Allow(toolName string) bool {
	if !destructiveTools[toolName] {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.state == GuardDone
}

// RunScan executes the project understanding flow for legacy projects.
// Mimics Cursor's approach: recursive file tree → identify key files → deep-read relevant ones.
func (g *ProjectAwarenessGuard) RunScan(ctx context.Context) {
	g.mu.Lock()
	if g.state == GuardDone || g.state == GuardScanning {
		g.mu.Unlock()
		return
	}
	g.state = GuardScanning
	g.mu.Unlock()

	if g.msgCh != nil {
		g.msgCh <- StreamChunkMsg{Content: "Scanning project structure before making changes...", Done: true}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, g.timeout)
	defer cancel()

	var analysis strings.Builder
	analysis.WriteString("[PROJECT ANALYSIS]\n")

	// Step 1: Recursive file tree (full picture, like Cursor's project indexing)
	analysis.WriteString("1. Project file tree:\n")
	tree := g.buildFileTree(timeoutCtx)
	if len(tree) > 3000 {
		tree = tree[:3000] + "\n... (truncated, " + fmt.Sprintf("%d", len(tree)) + " chars total)"
	}
	analysis.WriteString(tree)
	analysis.WriteString("\n\n")

	// Step 2: Read README / build config for high-level understanding
	readmePath := g.findReadme()
	if readmePath != "" {
		g.runStep(timeoutCtx, "read_file", map[string]interface{}{"path": filepath.Join(g.cwd, readmePath)}, &analysis, "2. README / build config")
	} else {
		analysis.WriteString("2. README / build config: not found\n\n")
	}

	// Step 3: Read entry point for architecture context
	entryPath := g.findEntryFile()
	if entryPath != "" {
		g.runStep(timeoutCtx, "read_file", map[string]interface{}{"path": filepath.Join(g.cwd, entryPath)}, &analysis, "3. Entry point")
	} else {
		analysis.WriteString("3. Entry point: not detected\n\n")
	}

	// Step 4: Search for package/module declarations (understand boundaries)
	g.runStep(timeoutCtx, "search_code", map[string]interface{}{"keyword": "^package |^import |^from |^use |^module ", "path": g.cwd}, &analysis, "4. Package/module structure")

	// Step 5: Read key config files (dependency manifests, configs)
	for _, cf := range g.findConfigFiles() {
		select {
		case <-timeoutCtx.Done():
			break
		default:
			g.runStep(timeoutCtx, "read_file", map[string]interface{}{"path": filepath.Join(g.cwd, cf)}, &analysis, "5. Config: "+cf)
		}
	}

	g.mu.Lock()
	g.state = GuardDone
	g.mu.Unlock()

	g.session.Add(session.Entry{
		Timestamp: time.Now(),
		Role:      "system",
		Content:   analysis.String(),
		State:     "analyzed",
	})

	if g.msgCh != nil {
		g.msgCh <- StreamChunkMsg{Content: "Project analysis complete. Ready to make changes.", Done: true}
	}
}

// runStep executes a single tool call and appends results to the analysis builder.
func (g *ProjectAwarenessGuard) runStep(ctx context.Context, toolName string, args map[string]interface{}, analysis *strings.Builder, label string) bool {
	tool, ok := g.toolbox.Get(toolName)
	if !ok {
		analysis.WriteString(fmt.Sprintf("%s: tool not found\n\n", label))
		return true
	}

	select {
	case <-ctx.Done():
		analysis.WriteString(fmt.Sprintf("%s: skipped (timeout)\n\n", label))
		return false
	default:
	}

	result := tool.Execute(ctx, args)
	analysis.WriteString(fmt.Sprintf("%s:\n", label))
	if result.Success {
		data := result.Data
		if len(data) > 2000 {
			data = data[:2000] + "\n... (truncated)"
		}
		analysis.WriteString(data)
	} else {
		analysis.WriteString(fmt.Sprintf("(error: %s)", result.Error))
	}
	analysis.WriteString("\n\n")
	return true
}

// finishWithPartial records a partial analysis on timeout/error.
func (g *ProjectAwarenessGuard) finishWithPartial(analysis string) {
	g.mu.Lock()
	g.state = GuardDone
	g.mu.Unlock()
	g.session.Add(session.Entry{
		Timestamp: time.Now(),
		Role:      "system",
		Content:   analysis + "(partial — timed out)",
		State:     "analyzed-partial",
	})
	if g.msgCh != nil {
		g.msgCh <- StreamChunkMsg{Content: "Project analysis timed out. Proceeding with partial information.", Done: true}
	}
}

// detectProjectType examines the working directory to classify the project.
func (g *ProjectAwarenessGuard) detectProjectType() ProjectType {
	buildFiles := []string{
		"go.mod", "package.json", "Cargo.toml", "pom.xml",
		"build.gradle", "build.gradle.kts", "Makefile", "CMakeLists.txt",
		"requirements.txt", "pyproject.toml", "setup.py",
	}

	hasBuildFile := false
	for _, bf := range buildFiles {
		if _, err := os.Stat(filepath.Join(g.cwd, bf)); err == nil {
			hasBuildFile = true
			break
		}
	}

	// Count source files
	extensions := map[string]bool{
		".go": true, ".js": true, ".ts": true, ".py": true,
		".rs": true, ".java": true, ".c": true, ".cpp": true,
		".h": true, ".hpp": true,
	}
	srcCount := 0
	filepath.Walk(g.cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && (info.Name() == ".git" || info.Name() == "node_modules" || info.Name() == "vendor") {
				return filepath.SkipDir
			}
			return nil
		}
		if extensions[filepath.Ext(path)] {
			srcCount++
		}
		if srcCount >= 3 {
			return filepath.SkipAll
		}
		return nil
	})

	if hasBuildFile && srcCount >= 3 {
		return ProjectLegacy
	}
	// Edge case: has structure but not enough sources — treat as legacy to be safe
	return ProjectGreenfield
}

// findReadme locates README.md, README, or similar in the working directory.
func (g *ProjectAwarenessGuard) findReadme() string {
	candidates := []string{"README.md", "README", "readme.md", "README.txt", "README.rst"}
	for _, name := range candidates {
		p := filepath.Join(g.cwd, name)
		if _, err := os.Stat(p); err == nil {
			return name
		}
	}
	buildCandidates := []string{"go.mod", "Makefile", "package.json", "Cargo.toml"}
	for _, name := range buildCandidates {
		p := filepath.Join(g.cwd, name)
		if _, err := os.Stat(p); err == nil {
			return name
		}
	}
	return ""
}

// findEntryFile locates the main program entry point.
func (g *ProjectAwarenessGuard) findEntryFile() string {
	candidates := []string{
		"cmd/omnidev-agent/main.go",
		"main.go",
		"cmd/main.go",
		"src/main.go",
		"index.js",
		"src/index.js",
		"src/main.ts",
		"main.py",
		"src/main.py",
		"app/main.py",
		"src/main.rs",
	}
	for _, name := range candidates {
		p := filepath.Join(g.cwd, name)
		if _, err := os.Stat(p); err == nil {
			return name
		}
	}
	return ""
}

// IsDestructive reports whether a tool at the given level modifies the filesystem.
func IsDestructive(toolName string, level permissions.Level) bool {
	return level == permissions.LevelDangerous && destructiveTools[toolName]
}

// buildFileTree recursively walks the project directory and builds a tree representation.
// Skips .git, node_modules, vendor, .ai_history, deliverables.
func (g *ProjectAwarenessGuard) buildFileTree(ctx context.Context) string {
	var sb strings.Builder
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		".ai_history": true, "deliverables": true, ".idea": true,
		".vscode": true, "__pycache__": true, "target": true,
		"bin": true, "dist": true, "build": true,
	}
	maxFiles := 200
	count := 0

	filepath.Walk(g.cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return filepath.SkipAll
		default:
		}

		rel, _ := filepath.Rel(g.cwd, path)
		if rel == "." {
			return nil
		}

		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			sb.WriteString(fmt.Sprintf("d %s\n", rel))
			return nil
		}

		count++
		if count > maxFiles {
			return filepath.SkipAll
		}
		size := info.Size()
		sb.WriteString(fmt.Sprintf("f %s (%dB)\n", rel, size))
		return nil
	})

	if count > maxFiles {
		sb.WriteString(fmt.Sprintf("... (%d+ files, listing truncated)\n", maxFiles))
	}
	return sb.String()
}

// findConfigFiles returns paths to dependency/config files worth reading.
func (g *ProjectAwarenessGuard) findConfigFiles() []string {
	candidates := []string{
		"go.mod", "go.sum",
		"package.json", "tsconfig.json",
		"Cargo.toml",
		"pyproject.toml", "requirements.txt",
		"docker-compose.yml", "Dockerfile",
		".env.example",
	}
	var found []string
	for _, c := range candidates {
		if _, err := os.Stat(filepath.Join(g.cwd, c)); err == nil {
			found = append(found, c)
		}
		if len(found) >= 3 {
			break
		}
	}
	return found
}
