# omnidev-agent

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**omnidev-agent** is a terminal AI coding agent (Go + [Bubbletea](https://github.com/charmbracelet/bubbletea) TUI). It reads project context, calls an LLM, runs a tool chain (read/write files, search code, shell, and more), and prompts for approval before dangerous operations. Supports OpenAI, DeepSeek, Anthropic, and any OpenAI-compatible gateway.

---

## Features

| Feature | Description |
|---------|-------------|
| **TUI** | Interactive terminal UI with task checklist, pipeline steps, and scrollable transcript |
| **Headless mode** | `omnidev-agent -p "task"` for one-shot runs (CI / scripts) |
| **Agent pipeline** | Intent → project assess → task plan → parallel sub-agents → tool loop |
| **Multi-LLM** | OpenAI, DeepSeek, Anthropic, and OpenAI-compatible gateways |
| **Checkpoint** | Resume or start fresh after interrupted multi-task runs |
| **Session persistence** | Runtime snapshots under `.ai_history/sessions/` |

---

## Quick Start

**Install and run from any directory:**

| Environment | Command |
|-------------|---------|
| Linux / macOS / WSL / Git Bash | `make deploy` |
| Windows (PowerShell) | `.\scripts\install.ps1` |

Then edit global config (`~/.omnidev-agent/config.json` or `%USERPROFILE%\.omnidev-agent\config.json`) — set `api_key`, `provider`, and `model` — and run:

```bash
omnidev-agent
```

**Develop in this repo** (build/run without installing):

```bash
git clone https://github.com/zayeagle/omnidev-agent.git
cd omnidev-agent
make config-local          # optional project override in ./.omnidev-agent.json
make config                # or global ~/.omnidev-agent/config.json
# edit the config file you created — set api_key, provider, model
make run                   # build bin/omnidev-agent and start TUI
```

---

## Usage

### TUI mode (default)

```bash
omnidev-agent
```

Enter a natural-language task, e.g. `Implement a snake game in deliverables/snake-game/`.

Common commands: `/help`, `/status`, `/model`, `/sessions`, `/clear`, `/yolo`, `quit`.

### Headless mode

```bash
omnidev-agent -p "run tests and summarize failures"
omnidev-agent -p "fix the bug" --yolo
omnidev-agent -p "task" --provider deepseek --model deepseek-chat
```

### CLI flags

| Flag | Description |
|------|-------------|
| `-p`, `--prompt` | Headless: run one prompt and exit |
| `--provider` | Override config provider |
| `--model` | Override config model |
| `--yolo` | Skip permission prompts |
| `-v`, `--version` | Print version and build time |

---

## Configuration

Minimal example (global or project `.omnidev-agent.json`):

```json
{
  "provider": "openai",
  "base_url": "https://api.openai.com/v1",
  "api_key":  "sk-your-key-here",
  "model":    "gpt-4o"
}
```

**Config locations**

| Scope | Path |
|-------|------|
| Project | `./.omnidev-agent.json` (cwd) |
| Global | `~/.omnidev-agent/config.json` (Windows: `%USERPROFILE%\.omnidev-agent\config.json`) |

**Priority:** CLI flags → environment variables → project config → global config → defaults.

See [docs/configuration-guide.md](docs/configuration-guide.md) for the full reference (all fields, LLM providers, tools, permissions, skills, MCP, debugging).

---

## Documentation

| Document | Description |
|----------|-------------|
| [Configuration & Usage Guide](docs/configuration-guide.md) | Full config, providers, tools, permissions, skills, MCP |
| [Requirements Spec](docs/需求文档.md) | Product requirements (Chinese) |

---

## Project layout

```
cmd/omnidev-agent/     Entry, TUI launcher, headless, CLI
internal/agent/        Agent loop, classify, decompose, checkpoint, sub-agents
internal/llm/          OpenAI / Anthropic providers, gateway adapters
internal/tools/        Tool registry and implementations
internal/tui/          Bubbletea UI
internal/config/       Layered config loading
internal/session/      Session store and export
docs/                  Requirements and guides
```

---

## License

MIT
