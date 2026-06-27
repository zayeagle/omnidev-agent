<!-- CHANGE_LOG: 2026-06-26 Phase 0 初始创建 -->
<!-- CHANGE_LOG: 2026-06-26 权限模型变更: 文件读写降为自动执行，Shell/系统命令/删除保持高危 -->
<!-- CHANGE_LOG: 2026-06-26 新增 AI 协作日志强制写入规则 -->
<!-- CHANGE_LOG: 2026-06-26 v2.0需求补全: 项目理解拦截器(guard.go) + SubAgent并行调度(dispatcher.go)
<!-- CHANGE_LOG: 2026-06-26 v1.3需求补全: 权限收紧(写/编→高危)+上下文摘要(120K/95%)+历史项目理解先行 -->

# 00 — Project Context

## Stack & Layers
- **Type**: Greenfield (全新项目，从零构建)
- **Classification**: backend-only（TUI 终端应用，无前端 Web 界面）
- **Language**: Go 1.24.5
- **TUI Framework**: Charmbracelet Bubbletea v1.3.10 + Lipgloss v1.1.0
- **Entry Point**: `cmd/omnidev-agent/main.go`
- **Internal Modules**: agent, tools, llm, permissions, session, config, tui（均为骨架代码）

## Dependency Topology
- **Storage**: 无数据库依赖，会话日志持久化至 `./.ai_history/sessions/`（JSON 文件）
- **Third-Party**: 无 HTTP API、gRPC、SDK 依赖（LLM SDK 待引入：OpenAI/Anthropic 官方或社区库）
- **TUI Rendering**: Bubbletea + Lipgloss（已引入）

## Stability Level
- **standard** — 需求文档未明确要求高可用/稳定性，标准级别即可

## Domain Knowledge
- 需求文档定稿 v1.0（2026-06-25），8 大核心能力域
- 项目名称：omnidev-agent，对标 Claude Code / Cursor Agent / Codex CLI
- 核心约束：Agent Loop、工具系统、权限控制、会话管理四模块必须自研，禁止依赖第三方 Agent 框架
- **强制规则**：每次 turn 结束前，将本轮完整对话追加写入 `.ai_history/logs/YYYYMMDD-session.{jsonl,md}`（需求 §3.4 / §4.1-b）
- **权限模型 (v1.2)**：文件读取/搜索自动执行；文件写入/编辑/删除、Shell/系统命令需弹窗确认
- **上下文管理 (v1.3)**：默认 120K token 窗口，占用≥95%时触发 LLM 摘要压缩（`agent/context.go`），保留最近 10 条原始记录，旧记录压缩为系统摘要
- **历史项目理解规则 (v1.3)**：修改既有项目文件前，必须先完成项目结构理解（架构、代码风格、业务背景），禁止脱离项目上下文擅自发挥

- **架构决策**：Agent 与 TUI 通过 Bubbletea Msg 通道异步通信；LLM Stream→Agent→TUI 逐 chunk 渲染；权限弹窗为 TUI 组件层叠在消息区上方
- **v2.0 新增**：ProjectAwarenessGuard 硬拦截写操作直到项目理解完成；TaskDispatcher 自动拆解+SubAgent 并行执行（max 4并行, 120s超时, 10 turns/subagent）
- **项目类型检测**：Greenfield（无构建文件+无.git）跳过理解；Legacy（有构建文件+源码≥3）强制执行四步理解

<!-- CHANGE_LOG: 2026-06-26 v2.1: Agent管道重构 — 始终拆解任务+Checkpoint断点续传+SubAgent中断恢复+Rollback支持 -->
- **v2.1 Agent Pipeline**: 4-stage → 用户指令 → 项目理解(Guard) → LLM拆解(始终) → Dispatcher(Checkpoint感知) → 完成
  - 简单任务也拆解（单任务计划），统一走 Dispatcher 路径
  - Checkpoint 写入 `.ai_history/checkpoints/checkpoint.json`，每完成一个子任务即更新
  - Ctrl+C 中断保留 checkpoint，下次启动自动检测恢复
  - SubAgent 标记 `subAgent=true` 跳过管道步骤，直接走标准 Loop
  - Rollback: 支持回滚到指定任务（含传递依赖），重跑受影响链
  - TUI 新增 `/checkpoint` 命令 + `ResolveConflictMsg` 冲突提示
- **目录结构新增**: `.ai_history/checkpoints/` + `internal/agent/checkpoint.go`
- **Agent struct 新增字段**: `cpStore *CheckpointStore`, `subAgent bool`
