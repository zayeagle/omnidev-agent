# omnidev-agent

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Autonomous all-round agent for diversified code generation and all kinds of custom programming tasks.

An AI pair programmer that runs in your terminal — with tools to read/write files, execute shell commands, search code, and delete artifacts. All dangerous operations require explicit user approval via an interactive TUI.

## Quick Start

```bash
# 1. Clone
git clone https://github.com/zayeagle/omnidev-agent.git
cd omnidev-agent

# 2. Initialize config (copies sample → .omnidev-agent.json)
make config

# 3. Edit API key
vim .omnidev-agent.json

# 4. Build & run
make run
```

Or install globally:

```bash
# Linux / macOS
make deploy          # build + install to ~/.local/bin + config init

# Windows (PowerShell)
.\scripts\install.ps1   # build + copy to %USERPROFILE%\.local\bin + PATH
```

Then run from any directory:

```bash
omnidev-agent
```

## Configuration

omnidev-agent reads `.omnidev-agent.json` from the **working directory** by default.  
A sample file (`.omnidev-agent.json.sample`) ships with the project and is committed to git;  
`make config` copies it to `.omnidev-agent.json` (gitignored) so you never commit secrets.

Config is merged from multiple sources (highest wins):

| Priority | Source | Example |
|----------|--------|---------|
| 1 | CLI flags | `--model gpt-4o` |
| 2 | Environment variables | `OMNIDEV_MODEL=gpt-4o` |
| 3 | Working directory | `.omnidev-agent.json` |
| 4 | Machine-wide | `~/.omnidev-agent/config.json` |
| 5 | Built-in defaults | OpenAPI / gpt-4o / timeout 60s / maxTurns 20 |

### Sample config (`.omnidev-agent.json.sample`)

```json
{
  "provider":    "openai",
  "base_url":    "https://api.openai.com/v1",
  "api_key":     "sk-your-key-here",
  "model":       "gpt-4o",
  "max_tokens":  16384,
  "temperature": 0.7,
  "timeout":     120,
  "max_turns":   20,
  "log_level":   "info",
  "session_dir": ".ai_history/sessions/"
}
```

`session_dir` 是 **omnidev-agent 运行时**会话快照目录。开发本项目的 Cursor/Codex 协作日志见 `AGENTS.md`，写入 `.ai_history/logs/`（不由程序配置项控制）。

### Global config (machine-wide, optional)

```json
// ~/.omnidev-agent/config.json
{
  "base_url": "https://your-proxy.com/v1",
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
# OMNIDEV_LOG_DIR is a legacy alias for OMNIDEV_SESSION_DIR
```

### Provider support

| Provider | `provider` value | Default `base_url` | Protocol |
|----------|----------------|--------------------|----------|
| OpenAI | `openai` | `https://api.openai.com/v1` | Chat Completions |
| DeepSeek | `deepseek` | `https://api.deepseek.com/v1` | Chat Completions (OpenAI-compatible) |
| Anthropic | `anthropic` or `claude` | `https://api.anthropic.com/v1` | Messages API |

Any **OpenAI-compatible** gateway (Ollama, vLLM, private proxy) works with `provider: "openai"` and a custom `base_url`.

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

Some enterprise gateways reject `role=system`, `temperature`, or large `max_tokens`. Use **strict compatibility mode**:

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

Strict mode: merges `system` into the first `user` message, omits `temperature`, caps `max_tokens` at 8192, and accepts SSE responses on non-streaming calls.

Actual request URL: `{base_url}/chat/completions`

```json
{
  "provider": "anthropic",
  "base_url": "https://api.anthropic.com/v1",
  "api_key": "sk-ant-...",
  "model": "claude-sonnet-4-20250514",
  "max_tokens": 8192
}
```


## Make Targets

```
make                   vet + build + test
make build             编译 binary → bin/omnidev-agent
make run               编译并运行
make test              运行全量测试
make vet               静态检查
make clean             清理构建产物
make install           安装到 ~/.local/bin
make config            初始化 .omnidev-agent.json（从 .sample 复制）
make deploy            install + config
make help              显示帮助
```

## License

MIT
