# omnidev-agent

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

**omnidev-agent** is a terminal AI coding agent (Go + [Bubbletea](https://github.com/charmbracelet/bubbletea) TUI). It reads project context, calls an LLM, runs a tool chain (read/write files, search code, shell, and more), and prompts for approval before dangerous operations. Supports OpenAI, DeepSeek, Anthropic, and any OpenAI-compatible gateway.

An autonomous terminal coding agent for diversified code generation and custom programming tasks — with an interactive TUI, tool execution, permission gates, task decomposition, and checkpoint resume.

---

## Features

| Feature | Description |
|---------|-------------|
| **TUI** | Cursor-style UI: task checklist, pipeline steps, collapsible thinking, completion banner, scrollable transcript |
| **Headless mode** | `omnidev-agent -p "task"` for one-shot runs (CI / scripts) |
| **Agent pipeline** | Intent classification → project assessment (legacy / greenfield) → task decomposition → parallel sub-agents → tool loop |
| **8 built-in tools** | List dir, read file, file/code search, write/edit/delete file, shell exec |
| **Permission model** | Read/write/search run automatically; shell, delete, etc. require **Y/N/A** confirmation (`/yolo` or `--yolo` to disable) |
| **Multi-LLM** | OpenAI Chat Completions, DeepSeek (compatible), Anthropic Messages API |
| **Gateway compat** | `compat_mode: strict` for enterprise gateways (merged system, no temperature, etc.) |
| **Context management** | Auto-summarize long sessions (configurable token limit and threshold) |
| **Checkpoint** | Resume or start fresh after interrupted multi-task runs |
| **Session persistence** | Runtime snapshots under `.ai_history/sessions/` |

---

## Architecture

```
User input (TUI / -p)
    │
    ▼
┌─────────────────────────────────────┐
│  Agent Loop (built in-house)        │
│  1. LLM classify: chat vs code mod   │
│  2. Project assess: legacy / new   │
│  3. Task decompose + parallel subs   │
│  4. LLM ↔ tool loop (max_turns)     │
└─────────────────────────────────────┘
    │
    ▼
LLM Provider ──► OpenAI / DeepSeek / Anthropic / custom gateway
Tool Registry  ──► read / write / edit / delete / shell / search
```

Core modules (agent loop, tools, permissions, sessions) are built in-house. LLM SDK and TUI libraries are third-party.

---

## Quick Start

```bash
# 1. Clone
git clone https://github.com/zayeagle/omnidev-agent.git
cd omnidev-agent

# 2. Initialize config (copy from sample; secrets are gitignored)
make config

# 3. Edit API key and model
vim .omnidev-agent.json

# 4. Build and run TUI
make run
```

---

## Deployment

### Linux / macOS (recommended)

```bash
make deploy          # rebuild + install to ~/.local/bin/omnidev-agent + make config
export PATH="$HOME/.local/bin:$PATH"
omnidev-agent
```

Update on a remote machine:

```bash
cd ~/path/to/omnidev-agent
git pull
make deploy
omnidev-agent --version
```

### Windows (PowerShell)

```powershell
.\scripts\install.ps1    # build, install to %USERPROFILE%\.local\bin, update PATH
omnidev-agent
```

### Build only (no install)

```bash
make build             # → bin/omnidev-agent
./bin/omnidev-agent
```

### Cross-compile

```bash
make build-linux-amd64
make build-darwin-arm64
make build-windows-amd64
make build-all         # all platforms
```

---

## Usage

### TUI mode (default)

```bash
omnidev-agent
```

Enter a natural-language task, e.g. `Implement a snake game in deliverables/snake-game/`.

**Built-in commands**

| Command | Description |
|---------|-------------|
| `/help` | Show help |
| `/status` | Agent, model, and permission status |
| `/model` | Current model and provider |
| `/checkpoint` | View in-progress checkpoint |
| `/sessions` | List archived sessions |
| `/session <file>` | Preview an archive |
| `/clear` | Clear transcript |
| `/yolo` | Toggle permission mode (confirm / auto-approve all) |
| `quit` / `exit` / `Ctrl+C` | Exit |

**Keyboard shortcuts**

| Key | Action |
|-----|--------|
| `↑` `↓` / `PgUp` `PgDn` | Scroll transcript (when input is empty) |
| `Home` / `End` | Jump to oldest / newest (when input is empty) |
| `Ctrl/Alt + ↑↓` | Input history |
| `Tab` / `Enter` / `Space` | Expand / collapse Thinking |
| `Esc` | Cancel current agent run |
| `Y` / `N` / `A` | Permission: approve / deny / allow all |
| `Y` / `N` | Checkpoint: resume / start fresh |

Optional mouse wheel (may interfere with terminal copy/paste):

```bash
export OMNIDEV_MOUSE_SCROLL=1
omnidev-agent
```

### Headless mode

```bash
omnidev-agent -p "run tests and summarize failures"
omnidev-agent -p "fix the bug" --yolo          # auto-approve dangerous ops
omnidev-agent -p "task" --provider deepseek --model deepseek-chat
```

Dangerous operations are **denied by default** in headless mode; `--yolo` matches TUI `/yolo`.

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

omnidev-agent reads `.omnidev-agent.json` from the **current working directory**.  
The sample file `.omnidev-agent.json.sample` is committed; `make config` copies it locally (gitignored) so secrets are never committed.

**Merge priority (highest → lowest)**

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flags | `--model gpt-4o` |
| 2 | Environment variables | `OMNIDEV_MODEL=gpt-4o` |
| 3 | Project config | `./.omnidev-agent.json` |
| 4 | Global config | `~/.omnidev-agent/config.json` |
| 5 | Built-in defaults | See table below |

### Full reference

| JSON field | Environment variable | Default | Description |
|------------|----------------------|---------|-------------|
| `provider` | `OMNIDEV_PROVIDER` | `openai` | `openai` / `deepseek` / `anthropic` |
| `base_url` | `OMNIDEV_BASE_URL` | `https://api.openai.com/v1` | API base URL |
| `api_key` | `OMNIDEV_API_KEY` | — | API key |
| `model` | `OMNIDEV_MODEL` | `gpt-4` | Model name |
| `max_tokens` | — | — | Max tokens per completion |
| `temperature` | — | — | Sampling temperature (omitted in strict mode) |
| `compat_mode` | `OMNIDEV_COMPAT_MODE` | `auto` | `auto` / `openai` / `strict` |
| `timeout` | `OMNIDEV_TIMEOUT` | `0` | HTTP timeout (seconds); `0` = SDK default |
| `max_turns` | `OMNIDEV_MAX_TURNS` | `20` | Max agent tool-loop turns |
| `log_level` | `OMNIDEV_LOG_LEVEL` | `info` | Log level |
| `session_dir` | `OMNIDEV_SESSION_DIR` | `.ai_history/sessions/` | **Runtime** session snapshots |
| `context_max_tokens` | `OMNIDEV_CONTEXT_MAX_TOKENS` | `120000` | Context window cap |
| `context_summarize_threshold` | `OMNIDEV_CONTEXT_SUMMARIZE_THRESHOLD` | `0.95` | Fraction of cap before summarization |
| `max_parallel` | `OMNIDEV_MAX_PARALLEL` | `2` | Parallel sub-agent count |
| `sub_agent_timeout` | `OMNIDEV_SUB_AGENT_TIMEOUT` | `120` | Sub-task timeout (seconds) |
| `sub_agent_max_turns` | `OMNIDEV_SUB_AGENT_MAX_TURNS` | `10` | Max turns per sub-agent |

> `OMNIDEV_LOG_DIR` is a legacy alias for `session_dir`.  
> `session_dir` stores **omnidev-agent runtime** snapshots only. Cursor/Codex dev collaboration logs use `.ai_history/logs/` (see `AGENTS.md`).

### Sample config

```json
{
  "provider":    "openai",
  "base_url":    "https://api.openai.com/v1",
  "api_key":     "sk-your-key-here",
  "model":       "gpt-4o",
  "max_tokens":  8192,
  "temperature": 0.7,
  "compat_mode": "auto",
  "timeout":     120,
  "max_turns":   20,
  "log_level":   "info",
  "session_dir": ".ai_history/sessions/",
  "max_parallel": 2,
  "sub_agent_timeout": 120,
  "sub_agent_max_turns": 10
}
```

### Global config (optional)

```json
// ~/.omnidev-agent/config.json
{
  "base_url": "https://your-proxy.com/v1",
  "api_key":  "sk-shared-key",
  "model":    "your-default-model",
  "session_dir": ".ai_history/sessions/"
}
```

### Environment variables

```bash
export OMNIDEV_PROVIDER=deepseek
export OMNIDEV_BASE_URL=https://api.deepseek.com/v1
export OMNIDEV_API_KEY=sk-xxxxxxxxxxxxxxxxxxxx
export OMNIDEV_MODEL=deepseek-chat
export OMNIDEV_TIMEOUT=120
export OMNIDEV_MAX_TURNS=20
export OMNIDEV_LOG_LEVEL=debug
export OMNIDEV_SESSION_DIR=.ai_history/sessions/
export OMNIDEV_COMPAT_MODE=auto
export OMNIDEV_MAX_PARALLEL=2
export OMNIDEV_SUB_AGENT_TIMEOUT=120
export OMNIDEV_SUB_AGENT_MAX_TURNS=10
export OMNIDEV_CONTEXT_MAX_TOKENS=120000
export OMNIDEV_CONTEXT_SUMMARIZE_THRESHOLD=0.95
# Optional TUI
export OMNIDEV_MOUSE_SCROLL=1
# LLM debug: log request/response
export OMNIDEV_LLM_DEBUG=1
```

---

## LLM providers

| Provider | `provider` value | Default `base_url` | Protocol |
|----------|------------------|--------------------|----------|
| OpenAI | `openai` | `https://api.openai.com/v1` | Chat Completions |
| DeepSeek | `deepseek` | `https://api.deepseek.com/v1` | OpenAI-compatible |
| Anthropic | `anthropic` or `claude` | `https://api.anthropic.com/v1` | Messages API |

Any **OpenAI-compatible** gateway (Ollama, vLLM, private proxy) works with `provider: "openai"` and a custom `base_url`:

```json
{
  "provider": "openai",
  "base_url": "https://your-gateway.example.com/v1",
  "api_key": "your-key",
  "model": "your-model",
  "max_tokens": 8192,
  "temperature": 0.7
}
```

### Compatibility mode (`compat_mode`)

| Mode | Behavior |
|------|----------|
| `auto` | Standard OpenAI: separate `system`, `temperature`, native `tools` |
| `openai` | Same as `auto` |
| `strict` | Merge `system` into first `user`; omit `temperature`; cap `max_tokens` at 8192; JSON tool plans instead of native `tools` on some gateways |

If an enterprise gateway returns `400` or rejects `system` / `temperature` / large `max_tokens`:

```json
{
  "provider": "openai",
  "base_url": "https://your-gateway.example.com/v1",
  "api_key": "your-key",
  "model": "your-model",
  "max_tokens": 8192,
  "compat_mode": "strict"
}
```

Anthropic example:

```json
{
  "provider": "anthropic",
  "base_url": "https://api.anthropic.com/v1",
  "api_key": "sk-ant-...",
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 8192
}
```

Request URL: `{base_url}/chat/completions` (OpenAI-style) or Anthropic Messages endpoint.

---

## Tools and permissions

| Tool | Permission | Description |
|------|------------|-------------|
| `list_dir` | Auto | Browse directories |
| `read_file` | Auto | Read file contents |
| `search_file` | Auto | Search by file name |
| `search_code` | Auto | Search file contents |
| `write_file` | **Confirm** | Create or overwrite (dialog shows diff preview) |
| `edit_file` | **Confirm** | Replace snippet (dialog shows diff preview) |
| `delete_file` | **Confirm** | Delete file |
| `shell_exec` | **Confirm** | Run shell command |

On legacy repos the agent scans the project before making changes. Greenfield / new-project code goes under `deliverables/` by default to avoid polluting the repo root.

---

## Data directories

| Path | Purpose | Written by |
|------|---------|------------|
| `.ai_history/sessions/` | Runtime TUI/headless session snapshots | omnidev-agent |
| `.ai_history/checkpoints/` | Multi-task checkpoints | omnidev-agent |
| `.ai_history/logs/` | Dev collaboration logs (Cursor, etc.) | External assistants; see `AGENTS.md` |

---

## Debugging

```bash
# Log LLM request/response (sensitive — do not leave on in production)
export OMNIDEV_LLM_DEBUG=1
omnidev-agent -p "hello"

# Generate curl payload matching agent requests
go run scripts/gen-agent-request.go
```

If concurrent gateway requests fail, reduce parallelism:

```bash
export OMNIDEV_MAX_PARALLEL=1
```

---

## Make targets

```
make                   vet + build + test
make build             Build → bin/omnidev-agent
make rebuild           Force full rebuild
make run               Build and run TUI
make test              Run tests
make vet               go vet
make clean             Remove build artifacts
make install           Install to ~/.local/bin
make config            Init .omnidev-agent.json from .sample
make deploy            rebuild + install + config
make build-all         Cross-compile all platforms
make help              Show help
```

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
docs/                  Requirements and design docs
```

See [`docs/需求文档.md`](docs/需求文档.md) for the full requirements spec (Chinese).

---

## License

MIT
