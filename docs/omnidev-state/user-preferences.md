---
last_updated: 2026-06-26T10:20:00+08:00
---

## 项目约束 (Project Constraints)
- ai_history_log: **开发 omnidev-agent 时**，Cursor/Codex 等外部助手每次 turn 结束前，将协作对话追加写入 `.ai_history/logs/YYYYMMDD-session.{jsonl,md}`。勿与 omnidev-agent 运行时快照目录 `.ai_history/sessions/` 混淆。
- omnidev_agent_sessions: omnidev-agent 程序自身 TUI/headless 历史会话 → `.ai_history/sessions/{id}.json|.md`，由程序自动 Save/Export，不写入 logs/。
- permission_model: 文件读取/搜索自动执行；文件写入/编辑/删除、Shell/系统级命令需弹窗确认（需求 v1.2）

## 交互偏好
- language: 中文回复
- log_format: JSONL + Markdown 双轨，每 turn 一个 entry，原文原样
- llm_provider: 默认支持 OpenAI + DeepSeek（OpenAI 协议兼容），通过 BaseURL 区分
