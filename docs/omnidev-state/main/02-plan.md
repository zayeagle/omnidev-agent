<!-- CHANGE_LOG: 2026-06-27 因 /od re 深度缺口分析同步更新: 新增 Phase 4 Cursor Agent 对标补齐计划 -->
<!-- CHANGE_LOG: 2026-06-26 因需求变更同步更新: 新增 T18 ProjectAwarenessGuard + T19 TaskDispatcher + T20 组装测试 -->
<!-- CHANGE_LOG: 2026-06-26 Phase 2 初始创建 -->

# 02 — Task Plan

---
branch: main
last_updated: 2026-06-27
phase: 4-gap-remediation
complexity: L
total_tasks: 12
parallel_groups: 4
critical_path: T21 → T22 → T24 → T26 → T30
benchmark: Cursor Agent + 需求文档 v1.3/v2.0
---

## 现状摘要（/od re 2026-06-27）

**已实现（可运行）**
- Agent Loop、Classifier、StandardLoop、Context 摘要（120K/95%）
- Legacy Guard：修改前扫描 + 写操作硬拦截（`guard.go` + `loop.go`）
- TaskDispatcher + SubAgent 并行（DAG + semaphore + checkpoint）
- TUI：Pipeline 步骤、Todo 面板、Thinking 折叠、权限弹窗、流式输出
- 8 工具 + 权限分级 + 多 Provider + strict gateway

**核心缺口（对标 Cursor Agent + 需求文档）**

| 优先级 | 缺口 | 影响 |
|--------|------|------|
| P0 | 并行调度仅 `LayoutDDD` 触发，Legacy/Minimal 走单任务串行 | v2.0 §7 未覆盖主场景 |
| P0 | 文件变更无 `+N -M` 行数展示 | Cursor 级 diff 反馈缺失 |
| P0 | 无显式「需求分析」阶段 | 用户看不到分析→拆分→执行链路 |
| P1 | Guard 扫描流程 ≠ §6.3 四步规范 | 理解流程与文档不一致 |
| P1 | `max_parallel` 默认 2（spec 要求 4） | 并行度不足 |
| P1 | Checkpoint Rollback 未暴露 TUI/CLI | v2.1 能力不可用 |
| P1 | 运行时日志路径与 §3.4 文档冲突 | 验收风险 |
| P2 | 无计划编辑、语义搜索、Git diff 工具 | Cursor 高级能力 |
| P2 | `deliverables/` 验证截图缺失 | §4.3 未满足 |

---

## Phase 4 — Cursor Agent 对标补齐计划

### Group A — 调度与需求分析（P0，可部分并行）

- [x] **T21** [agent] 解除 Dispatcher 的 DDD 门禁 · outputs: `internal/agent/loop.go`, `internal/agent/dispatcher.go`
  - 规则：`code_modification` + 任务数 ≥2 或 LLM 判定「可并行」→ 走 Dispatcher
  - Legacy 项目：Guard 扫描完成后再拆解（保持写拦截）
  - Minimal 小任务（单文件）：仍允许单任务 fast path
  - 默认 `max_parallel: 4`（对齐 v2.0 §7.2）
  - **depends**: 现有 T18/T19

- [x] **T22** [agent] 显式需求分析阶段 · outputs: `internal/agent/requirements.go`, `loop.go`, `messages.go`
  - 在 Guard/Decompose 之前插入 LLM 调用：输出结构化摘要
    - 用户诉求 / 验收标准 / 影响范围 / 风险点
  - 注入 session + TUI status line（非 Thinking 折叠）
  - TUI 新增 pipeline step：`StepAnalyze`（需求分析）
  - **depends**: T21

- [x] **T23** [agent] 任务计划可审阅 · outputs: `internal/tui/update.go`, `components/todolist.go`
  - Decompose 完成后展示任务列表，用户 Enter 确认或 Esc 取消
  - 可选：数字键跳过某子任务（P2 可延后）
  - **depends**: T22

### Group B — 文件变更展示（P0，Cursor 对标核心）

- [x] **T24** [tools+tui] 文件变更行数统计
- [x] **T25** [tui] 变更汇总面板

### Group C — Legacy 理解对齐（P1）

- [x] **T26** [agent] Guard 扫描对齐 §6.3
- [x] **T27** [agent+tui] Rollback 暴露
- [x] **T28** [session] 运行时日志路径澄清（双目录文档对齐）
- [x] **T29** [tests] 并行 + 变更展示集成测试
- [x] **T30** [deliverables] 验证截图（README 清单）
- [x] **T31** [tools] Git 状态/diff 工具
- [x] **T32** [tools] 增强代码检索（rg 优先）

---

## 执行顺序建议

```
Week 1 (P0):  T24 → T25 → T21 → T22
Week 2 (P1):  T26 → T23 → T27 → T29
Week 3 (验收): T28(文档确认) → T30 → T31/T32(可选)
```

## 验收标准（Phase 4 完成定义）

1. Legacy 仓库收到「多文件功能需求」时：先 Guard 理解 → 需求分析可见 → 任务拆分 → ≥2 子任务并行执行
2. 每次 write/edit/delete 后 TUI 显示 `path (+N -M)`；Turn 结束有变更文件汇总
3. `max_parallel` 默认 4，checkpoint 可 resume
4. Guard 四步与 §6.3 一致（测试覆盖）
5. `deliverables/demo_screenshot/` 有完整流程截图

---

## 历史计划（Phase 1–3 + v2.0，已基本完成）

> 以下 T1–T20 为初版计划，多数已实现。保留作归档参考；新工作以 Phase 4（T21–T32）为准。

<details>
<summary>T1–T20 原始任务列表（点击展开）</summary>

### Group 1–5 + Phase 3 Extension (T1–T20)

- T1–T17：配置/Session/LLM/Tools/Agent/TUI/Tests — **已完成**
- T18 ProjectAwarenessGuard — **已完成**（需 T26 对齐 §6.3）
- T19 TaskDispatcher — **已完成**（需 T21 解除 DDD 门禁）
- T20 组装测试 — **部分完成**（需 T29 补充 Legacy 并行场景）

</details>
