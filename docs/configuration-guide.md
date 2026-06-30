# Configuration & Usage Guide

Detailed reference for omnidev-agent configuration, LLM providers, tools, permissions, skills, and MCP. For a quick overview, see the [README](../README.md).

---

## Runtime environment

What you need on the machine where **omnidev-agent** runs (TUI or headless).

### Platform support

| OS | TUI | Headless (`-p`) | Notes |
|----|-----|-----------------|-------|
| **Linux** | ✅ Recommended | ✅ | Native PTY; best-tested |
| **macOS** | ✅ | ✅ | Same as Linux |
| **Windows** | ✅ | ✅ | Use **Windows Terminal** or modern conhost; build/run via `scripts/install.ps1` or `bin/omnidev-agent.exe` |
| **WSL** | ✅ | ✅ | Run the Linux binary inside WSL; do **not** use Bubbletea `WithInputTTY()` (breaks CJK IME) |

Supported CPU architectures: **amd64**, **arm64** (see `make build-<os>-<arch>`).

### Build from source (optional)

Only required if you compile locally (`make build` / `make deploy`):

| Dependency | Version | Purpose |
|------------|---------|---------|
| [Go](https://go.dev/dl/) | **1.24.5+** (see `go.mod`) | Build the binary |
| `make`, `bash` | — | Makefile targets on Linux/macOS |
| `git` | — | Clone / update the repo |

Running a **pre-built** binary (`make install`, release artifact, or `scripts/install.ps1`) does **not** require Go at runtime.

### TUI terminal requirements

The interactive UI uses [Bubbletea](https://github.com/charmbracelet/bubbletea) with **alternate screen** mode.

| Requirement | Details |
|-------------|---------|
| **Interactive TTY** | Run in a real terminal (SSH session, local console, Windows Terminal). Piping stdout or running under non-TTY CI without `-p` will not show the TUI. |
| **Terminal size** | At least **80×24** columns/rows recommended; layout adapts to window resize. |
| **Unicode / UTF-8** | User input and LLM output may include non-ASCII text; set locale/encoding to UTF-8. |
| **ANSI colors** | True-color or 256-color terminals work best; basic ANSI is supported. |
| **Mouse** (optional) | Set `OMNIDEV_MOUSE_SCROLL=1` for wheel scrolling; may interfere with native terminal copy/paste. |

If the TUI fails to start, use headless mode: `omnidev-agent -p "your task"`.

### Network and LLM API

| Requirement | Details |
|-------------|---------|
| **Outbound HTTPS** | Access to your configured `base_url` (e.g. OpenAI, DeepSeek, Anthropic, or corporate gateway). |
| **API credentials** | Valid `api_key` in `.omnidev-agent.json`, `~/.omnidev-agent/config.json`, or `OMNIDEV_API_KEY`. |
| **Proxy / firewall** | If the gateway is internal, ensure the host can reach it; configure OS-level `HTTP_PROXY` / `HTTPS_PROXY` if needed (standard Go `net/http`). |

No local LLM runtime (Ollama, etc.) is bundled — point `base_url` at your own server if you use one.

### Shell and tools (agent `shell_exec`)

When the agent runs shell commands (after your approval):

| OS | Shell used |
|----|------------|
| Linux / macOS | `$SHELL`, or `/bin/sh` if unset |
| Windows | `%ComSpec%` (typically `cmd.exe /C …`) |

Timeout: **30 seconds** per command (hard limit in `shell_exec`).

The agent itself does **not** require `git`, `rg`, `grep`, or Node installed — file/code search uses built-in Go walks. The **LLM may still invoke** project tools (`go`, `npm`, etc.) via `shell_exec` depending on your task; install those in the project environment as usual.

### Filesystem and permissions

| Path / action | Purpose |
|---------------|---------|
| **Current working directory** | Project root the agent reads and writes; run `omnidev-agent` from your repo or task folder. |
| `.omnidev-agent.json` | Optional **project** override (`make config-local`); used when cwd is the project root. |
| `~/.omnidev-agent/config.json` (or `%USERPROFILE%\.omnidev-agent\config.json` on Windows) | **Global** config (`make config` / `make deploy` / `install.ps1`). Resolved via `os.UserHomeDir()`. |
| `.ai_history/sessions/` | Session snapshots (auto-created). |
| `.ai_history/checkpoints/` | Multi-task checkpoints (auto-created). |
| `deliverables/` | Default output workspace for new/greenfield projects. |

The OS user running omnidev-agent must have read access to the project and write access where the agent is allowed to create or modify files.

### Environment variables (runtime)

Besides config overrides (see [Configuration](#configuration)), these affect runtime behavior only:

```bash
OMNIDEV_MOUSE_SCROLL=1    # optional TUI mouse wheel
OMNIDEV_LLM_DEBUG=1         # log LLM request/response to stderr (debug)
HTTP_PROXY / HTTPS_PROXY  # optional corporate proxy (Go standard)
```

---

## Configuration

**Project config** — file `.omnidev-agent.json` in the **current working directory** (only when you run from that directory).

**Global config** — `<user-home>/.omnidev-agent/config.json`, where user home is from `os.UserHomeDir()` (`$HOME` on Linux/macOS/Git Bash/WSL, `%USERPROFILE%` on Windows). This matches paths created by `make deploy` and `scripts/install.ps1`.

The sample `.omnidev-agent.json.sample` is committed; generated configs are gitignored.

**Merge priority (highest → lowest)**

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flags | `--model gpt-4o` |
| 2 | Environment variables | `OMNIDEV_MODEL=gpt-4o` |
| 3 | Project config | `./.omnidev-agent.json` |
| 4 | Global config | `~/.omnidev-agent/config.json` (Windows: `%USERPROFILE%\.omnidev-agent\config.json`) |
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
| `context_max_tokens` | `OMNIDEV_CONTEXT_MAX_TOKENS` | `120000` | Context window cap (token estimate) |
| `context_summarize_threshold` | `OMNIDEV_CONTEXT_SUMMARIZE_THRESHOLD` | `0.95` | Compress early history when usage exceeds this fraction of cap (**95%** default) |
| `max_parallel` | `OMNIDEV_MAX_PARALLEL` | `4` | Parallel sub-agent count |
| `sub_agent_timeout` | `OMNIDEV_SUB_AGENT_TIMEOUT` | `120` | Sub-task timeout (seconds) |
| `sub_agent_max_turns` | `OMNIDEV_SUB_AGENT_MAX_TURNS` | `10` | Max LLM↔tool turns per sub-agent |
| `sub_agent_max_retries` | `OMNIDEV_SUB_AGENT_MAX_RETRIES` | `0` | Re-run a failed sub-task up to N extra times |
| `llm_max_retries` | `OMNIDEV_LLM_MAX_RETRIES` | `3` | LLM API retries after the first attempt (4 calls total) |
| `llm_retry_backoff_sec` | `OMNIDEV_LLM_RETRY_BACKOFF_SEC` | `1,2,4` | Seconds to wait before each LLM retry (comma-separated) |
| `max_consecutive_tool_denials` | `OMNIDEV_MAX_CONSECUTIVE_TOOL_DENIALS` | `3` | Abort after N turns with denied tools; `0` = never abort |
| `tool_result_max_chars` | `OMNIDEV_TOOL_RESULT_MAX_CHARS` | `8000` | Inline char budget per tool result sent to the LLM |
| `tool_spool_dir` | `OMNIDEV_TOOL_SPOOL_DIR` | `.ai_history/tool_spool/` | Full tool payloads when output exceeds inline budget |
| `search_code_max_lines` | `OMNIDEV_SEARCH_CODE_MAX_LINES` | `100` | Max match lines for `search_code` before PARTIAL |
| `list_dir_max_entries` | `OMNIDEV_LIST_DIR_MAX_ENTRIES` | `200` | Max directory entries before PARTIAL |
| `read_file_default_limit` | `OMNIDEV_READ_FILE_DEFAULT_LIMIT` | `300` | Default lines when `read_file` omits `limit` |
| `context_tool_results_keep_full` | `OMNIDEV_CONTEXT_TOOL_RESULTS_KEEP_FULL` | `3` | Only the last N tool results stay full in LLM context; older ones become refs |
| `context_min_keep_entries` | `OMNIDEV_CONTEXT_MIN_KEEP_ENTRIES` | `10` | Recent entries kept verbatim during compaction |
| `guard_analysis_max_chars` | `OMNIDEV_GUARD_ANALYSIS_MAX_CHARS` | `4000` | Cap `[PROJECT ANALYSIS]` stored in session |
| `pipeline_use_llm_classifier` | `OMNIDEV_PIPELINE_USE_LLM_CLASSIFIER` | `false` | LLM intent classification (default: strict chat heuristics) |
| `pipeline_use_llm_requirements` | `OMNIDEV_PIPELINE_USE_LLM_REQUIREMENTS` | `false` | LLM requirements pass before decomposition |
| `pipeline_use_llm_complexity` | `OMNIDEV_PIPELINE_USE_LLM_COMPLEXITY` | `false` | LLM layout classifier (default: keyword heuristic) |
| `pipeline_plan_mode` | `OMNIDEV_PIPELINE_PLAN_MODE` | `0` | `0`=LLM decides 1 vs N tasks (default), `1`=same as 0, `2`=skip plan (always single task) |
| `skills_dirs` | — | see [Skills & MCP](#skills--mcp) | Extra directories to scan for `SKILL.md` |
| `mcp_servers` | — | — | MCP stdio servers → dynamic agent tools |

> `OMNIDEV_LOG_DIR` is a legacy alias for `session_dir`.  
> `session_dir` stores **omnidev-agent runtime** snapshots only. Cursor/Codex dev collaboration logs use `.ai_history/logs/` (see `AGENTS.md`).

### Retry & execution limits

These settings control how the agent recovers from transient failures and when it stops trying.

| Layer | Config | Default behavior |
|-------|--------|------------------|
| **LLM API** | `llm_max_retries`, `llm_retry_backoff_sec` | On network/5xx errors, retry up to 3 times with 1s / 2s / 4s backoff. 4xx errors are not retried. Set `llm_max_retries` to `0` for a single attempt. |
| **Main agent loop** | `max_turns` | Up to 20 rounds of LLM ↔ tool execution per user turn. Tool failures are fed back to the model within this budget (not a separate retry counter). |
| **Sub-agent** | `sub_agent_max_turns`, `sub_agent_timeout` | Each parallel sub-task gets up to 10 tool-loop turns and 120s wall time. |
| **Sub-task failure** | `sub_agent_max_retries` | When a sub-task exits with error, re-run it from scratch up to N additional times (`0` = no retry). |
| **Permission denials** | `max_consecutive_tool_denials` | Abort after 3 consecutive turns where the user denied tool operations. Set to `0` to disable this guard. |

Environment example for a more aggressive retry profile:

```bash
export OMNIDEV_LLM_MAX_RETRIES=5
export OMNIDEV_LLM_RETRY_BACKOFF_SEC=2,4,8,16,32
export OMNIDEV_SUB_AGENT_MAX_RETRIES=2
export OMNIDEV_MAX_CONSECUTIVE_TOOL_DENIALS=5
```

### Tool output (PARTIAL + spool)

Large tool outputs are **never silently dropped**. When a result exceeds `tool_result_max_chars`, the agent:

1. Writes the **full payload** to `tool_spool_dir` (or references the original file path for `read_file`).
2. Returns a **head+tail preview** with a `[PARTIAL …]` banner and an explicit **Continue** hint (offset/limit pagination or `read_file` on the spool path).

| Tool | Continuation |
|------|----------------|
| `read_file` | `offset` + `limit` (1-based lines) |
| `search_code` / `list_dir` | Narrow query/path; capped by `search_code_max_lines` / `list_dir_max_entries` |
| `shell_exec`, `git_*` | `read_file` on spool path |

```bash
export OMNIDEV_TOOL_RESULT_MAX_CHARS=8000
export OMNIDEV_TOOL_SPOOL_DIR=.ai_history/tool_spool/
```

### Token optimization (defaults on)

These behaviors reduce context size **without dropping data** (PARTIAL/spool still applies):

| Mechanism | What it does |
|-----------|----------------|
| **Pipeline heuristics** | Skips classifier / requirements / complexity LLM by default |
| **Task plan (default)** | LLM planner decides 1 task vs multi-task split; set `pipeline_plan_mode: 2` to skip |
| **Write arg slimming** | `write_file` / `edit_file` tool-call args stored as `[omitted N chars]` — not re-sent every turn |
| **Tool result aging** | Only last 3 tool results stay full; older ones become one-line reload refs |
| **Guard condensation** | `[PROJECT ANALYSIS]` compressed before session storage |
| **Sub-agent parent hint** | Sub-tasks receive condensed project summary + parent user request |
| **Compaction input slim** | Early history summarized using archived refs, not full tool payloads |
| **Tighter tool caps** | 8k inline, 300-line read default, 100 search lines, 200 list entries |

Re-enable optional LLM pipeline stages when you need them:

```bash
export OMNIDEV_PIPELINE_USE_LLM_REQUIREMENTS=true
```

Skip task planning entirely (always one task, saves one LLM call):

```bash
export OMNIDEV_PIPELINE_PLAN_MODE=2
```

### Context window

When estimated tokens exceed `context_max_tokens × context_summarize_threshold`, older messages are summarized via LLM into a single `[EARLY CONTEXT SUMMARY]` entry (recent 10 entries are kept verbatim). The footer context % uses the same estimator and cap.

| Setting | Default | Example override |
|---------|---------|------------------|
| `context_max_tokens` | `120000` | `"context_max_tokens": 200000` |
| `context_summarize_threshold` | `0.95` (95%) | `"context_summarize_threshold": 0.90` for 90% |

```bash
export OMNIDEV_CONTEXT_MAX_TOKENS=120000
export OMNIDEV_CONTEXT_SUMMARIZE_THRESHOLD=0.90
```

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
  "context_max_tokens": 120000,
  "context_summarize_threshold": 0.95,
  "max_parallel": 4,
  "sub_agent_timeout": 120,
  "sub_agent_max_turns": 10,
  "sub_agent_max_retries": 0,
  "tool_result_max_chars": 8000,
  "tool_spool_dir": ".ai_history/tool_spool/",
  "search_code_max_lines": 100,
  "list_dir_max_entries": 200,
  "read_file_default_limit": 300,
  "context_tool_results_keep_full": 3,
  "guard_analysis_max_chars": 4000,
  "pipeline_plan_mode": 0,
  "skills_dirs": [".omnidev-agent/skills"],
  "mcp_servers": {
    "playwright": {
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest"],
      "tool_level": "dangerous",
      "disabled": true
    }
  },
  "llm_max_retries": 3,
  "llm_retry_backoff_sec": [1, 2, 4],
  "max_consecutive_tool_denials": 3
}
```

### Global config

Created automatically by `make deploy` / `make config` / `scripts/install.ps1` if missing:

```json
// Linux/macOS/WSL/Git Bash: ~/.omnidev-agent/config.json
// Windows:               %USERPROFILE%\.omnidev-agent\config.json
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
export OMNIDEV_LLM_MAX_RETRIES=3
export OMNIDEV_LLM_RETRY_BACKOFF_SEC=1,2,4
export OMNIDEV_SUB_AGENT_MAX_RETRIES=0
export OMNIDEV_MAX_CONSECUTIVE_TOOL_DENIALS=3
export OMNIDEV_LOG_LEVEL=debug
export OMNIDEV_SESSION_DIR=.ai_history/sessions/
export OMNIDEV_COMPAT_MODE=auto
export OMNIDEV_MAX_PARALLEL=4
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
| `list_skills` | Auto | List loaded SKILL.md workflows |
| `load_skill` | Auto | Load skill instructions into context |
| `mcp_*` | Configurable | External MCP server tools (`tool_level` in config) |

On legacy repos the agent scans the project before making changes. Greenfield / new-project code goes under `deliverables/` by default to avoid polluting the repo root.

Use `/yolo` in TUI or `--yolo` on CLI to auto-approve dangerous operations.

---

## Skills & MCP

### Skills (SKILL.md)

Skills are reusable instruction packs (compatible with Cursor-style `SKILL.md` layout). The agent discovers them at startup and exposes two tools:

| Tool | Permission | Description |
|------|------------|-------------|
| `list_skills` | Auto | List loaded skills with descriptions |
| `load_skill` | Auto | Load full skill instructions into the conversation |

**Search paths** (first match wins on name collision; project overrides global):

1. Paths in config `skills_dirs` (if set)
2. Default: `~/.omnidev-agent/skills/<name>/SKILL.md`
3. Default: `.omnidev-agent/skills/<name>/SKILL.md` (project-local)

**TUI**

```text
/skills              # list loaded skills
/skill example       # inject skill body into session as [SKILL: example]
```

**Layout example**

```
.omnidev-agent/skills/
└── my-workflow/
    └── SKILL.md      # name = directory "my-workflow"
```

A sample skill ships at `.omnidev-agent/skills/example/SKILL.md`.

**Config**

```json
{
  "skills_dirs": [
    ".omnidev-agent/skills",
    "/shared/team-skills"
  ]
}
```

Ask the agent naturally: *"load the example skill and follow it"* — it will call `load_skill`.

### MCP (Model Context Protocol)

Connect external MCP servers over **stdio**; their tools are registered as `mcp_<server>__<tool_name>` (e.g. `mcp_playwright__browser_navigate`).

**Config** (global or project `.omnidev-agent.json`):

```json
{
  "mcp_servers": {
    "playwright": {
      "command": "npx",
      "args": ["-y", "@playwright/mcp@latest"],
      "tool_level": "dangerous",
      "cwd": ".",
      "env": {},
      "disabled": false
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "tool_level": "safe"
    }
  }
}
```

| Field | Description |
|-------|-------------|
| `command` | Executable to spawn (must speak MCP over stdin/stdout) |
| `args` | Arguments passed to the command |
| `env` | Extra environment variables |
| `cwd` | Working directory for the subprocess |
| `tool_level` | `safe` (auto-run) or `dangerous` (confirm in TUI; denied in headless unless `--yolo`) |
| `disabled` | Skip this server when `true` |

**Runtime**

- Servers start when omnidev-agent launches; failures are logged and skipped (other servers continue).
- MCP tool output uses the same **PARTIAL + spool** limits as built-in tools.
- `/status` shows connected MCP servers and tool counts.

**Requirements**

- The MCP server binary must be on `PATH` (or use full path in `command`).
- Node-based servers typically need `npx` / `node` installed.
- Headless: MCP tools with `tool_level: dangerous` follow the same rules as `shell_exec` (use `--yolo` to auto-approve).

**Disable MCP**

Omit `mcp_servers` or set `"disabled": true` per server.

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

Linux, macOS, Git Bash, and WSL only (native Windows PowerShell: use `scripts/install.ps1` instead of `make deploy`).

```
make                   vet + build + test
make build             Build → bin/omnidev-agent
make rebuild           Force full rebuild
make run               Build and run TUI
make test              Run tests
make vet               go vet
make clean             Remove bin/
make install           Build and install to ~/.local/bin
make config            Init global config (~/.omnidev-agent/config.json)
make config-local      Init project config (./.omnidev-agent.json)
make deploy            rebuild + install + global & project config
make build-all         Cross-compile all platforms
make help              Show help
```
