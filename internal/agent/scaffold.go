package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// DDDPackageTemplates maps directory → filename → content for a Go DDD layout.
// The agent creates this structure before allowing code modifications on greenfield projects.
var DDDPackageTemplates = map[string]map[string]string{
	"cmd/server": {
		"main.go": `package main

import (
	"log"
)

func main() {
	log.Println("Starting server...")
}
`,
	},
	"internal/domain": {
		"doc.go": `// Package domain contains the core business entities and value objects.
// This layer has zero dependencies on infrastructure or application concerns.
package domain
`,
	},
	"internal/application": {
		"doc.go": `// Package application contains use-case orchestrators (service layer).
// It depends on domain types and defines ports (interfaces) that
// infrastructure adapters must implement.
package application
`,
	},
	"internal/infrastructure": {
		"doc.go": `// Package infrastructure contains adapters for external systems:
// database, HTTP clients, message queues, file I/O, etc.
// Each sub-package implements one or more ports defined by the application layer.
package infrastructure
`,
	},
	"internal/interfaces": {
		"doc.go": `// Package interfaces contains inbound adapters:
// HTTP handlers, gRPC servers, CLI commands, event consumers.
// These translate external requests into application-layer calls.
package interfaces
`,
	},
}

// Scaffolder initializes a greenfield project with a default DDD layout.
type Scaffolder struct {
	cwd string
}

// NewScaffolder creates a scaffolder for the given working directory.
func NewScaffolder(cwd string) *Scaffolder {
	return &Scaffolder{cwd: cwd}
}

// InitDDD creates the DDD directory structure and placeholder files.
// Returns the list of created paths.
func (s *Scaffolder) InitDDD(ctx context.Context, msgCh chan<- tea.Msg) ([]string, error) {
	if msgCh != nil {
		msgCh <- StreamChunkMsg{
			Content: "Initializing DDD project structure...",
			Done:    true,
		}
	}

	var created []string
	for dir, files := range DDDPackageTemplates {
		targetDir := filepath.Join(s.cwd, dir)
		if err := os.MkdirAll(targetDir, 0o755); err != nil {
			if msgCh != nil {
				msgCh <- StreamChunkMsg{
					Content: fmt.Sprintf("Failed to create %s: %v", dir, err),
					Done:    true,
				}
			}
			return created, fmt.Errorf("mkdir %s: %w", dir, err)
		}
		created = append(created, targetDir+"/")

		for name, content := range files {
			targetFile := filepath.Join(targetDir, name)
			// Do not overwrite existing files
			if _, err := os.Stat(targetFile); err == nil {
				continue
			}
			if err := os.WriteFile(targetFile, []byte(content), 0o644); err != nil {
				return created, fmt.Errorf("write %s: %w", targetFile, err)
			}
			created = append(created, targetFile)
		}
	}

	if msgCh != nil {
		msgCh <- StreamChunkMsg{
			Content: fmt.Sprintf("DDD structure ready (%d items created).", len(created)),
			Done:    true,
		}
	}
	return created, nil
}
