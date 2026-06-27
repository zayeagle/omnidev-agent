# CLAUDE.md

## 项目概述
omnidev-agent — 从零实现最小可用 TUI 终端编码 Agent（Go 1.24.5 + Bubbletea）。

需求文档：`docs/需求文档.md`

## 强制规则

### 存储目录（两套，勿混淆）

| 目录 | 用途 | 谁写入 |
|------|------|--------|
| `.ai_history/logs/` | **开发 omnidev-agent 期间**，外部代码助手（Cursor、Codex、Claude Code 等）的协作对话与工具记录 | 外部助手在每个 turn 结束前**追加** |
| `.ai_history/sessions/` | **omnidev-agent 程序运行时**（TUI / headless `-p`）的历史会话快照 | omnidev-agent 自动 `Save` / `Export` |

- `logs/` ≠ `sessions/`：前者是「写这个项目的 AI 助手」留档；后者是「跑起来的 omnidev-agent」留档。
- **禁止**把 omnidev-agent 运行时 TUI 会话写入 `logs/`。
- **禁止**把 Cursor/Codex 开发协作记录写入 `sessions/`（除非人工归档副本）。

### 协作日志写入（仅外部开发助手）

在**开发 omnidev-agent 本项目**时，Cursor / Codex 等外部助手每次 turn 结束前，必须将本轮完整对话**追加**写入 `.ai_history/logs/`：

- JSONL：`.ai_history/logs/YYYYMMDD-session.jsonl`（每行一条 entry，`timestamp` / `role` / `content` / 可选 `tool_calls`）
- Markdown：`.ai_history/logs/YYYYMMDD-session.md`（人类可读，在已有文件末尾追加）
- 内容：用户输入与助手回复原文、工具调用清单、关键决策；**追加**到当日文件，不覆盖历史行
- 此为需求文档 §3.4 / §4.1-b 对「AI 协作全量日志」的强制要求，不可跳过

### 开发约束
- Agent Loop、工具系统、权限控制、会话管理四个核心模块必须自研，禁止依赖第三方 Agent 框架（LangChain、LlamaIndex、AutoGen 等）
- LLM SDK 和 TUI 库允许使用第三方库

### 权限模型 (v1.1)
- 自动执行：文件读取、文件写入、文件编辑、目录浏览、代码检索
- 需弹窗确认：Shell 命令执行、系统级命令、删除文件/目录

### LLM 支持
- OpenAI + DeepSeek + Anthropic（DeepSeek/OpenAI 网关走 Chat Completions；Anthropic 走 Messages API）
- Config 结构需含 `Provider`、`BaseURL`、`APIKey`、`Model`、`MaxTokens`、`Temperature` 字段
