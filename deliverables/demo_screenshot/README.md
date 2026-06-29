# Demo Screenshots (§4.3)

Capture TUI screenshots while running **omnidev-agent** on a small dev task (e.g. terminal snake game or a multi-file legacy change).

## Required scenes

| File | Scene |
|------|--------|
| `01-guard-scan.png` | Legacy project detected — Guard four-step scan in progress |
| `02-requirements.png` | Requirements analysis visible in transcript (not folded Thinking) |
| `03-task-plan.png` | Decomposed task list + plan confirm overlay (Enter/Esc) |
| `04-parallel-exec.png` | Multiple sub-tasks running (To-dos panel) |
| `05-file-changes.png` | Turn complete — file change summary with `(+N -M)` |
| `06-completion.png` | Completion banner with project directory |

## How to capture

1. Build: `go build -o bin/omnidev-agent ./cmd/omnidev-agent`
2. Run TUI from a legacy repo root with API key configured.
3. Submit a multi-file feature request.
4. Save screenshots to this directory using the names above.

> Screenshots are verification artifacts only; generated app code lives under `deliverables/<project>/`, not here.
