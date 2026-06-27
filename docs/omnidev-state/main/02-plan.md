<!-- CHANGE_LOG: 2026-06-26 因需求变更同步更新: 新增 T18 ProjectAwarenessGuard + T19 TaskDispatcher + T20 组装测试 -->
<!-- CHANGE_LOG: 2026-06-26 Phase 2 初始创建 -->

# 02 — Task Plan

---
total_tasks: 17
parallel_groups: 5
critical_path: T1 → T6 → T11 → T13 → T16
frontend_impact: no
---

## Group 1 (并行 — 无前置依赖)

基础设施层，可同时开工。

- [ ] **T1** [config] 配置分层加载 · outputs: `internal/config/layers.go`
  - 实现 CLI flags > env > 项目配置 > 全局配置 > 默认值 的优先级合并
  - Config 结构已含 Provider/BaseURL/APIKey/Model/Timeout/LogLevel

- [ ] **T2** [session] Session.Entry 增强 + Store MD 导出 · outputs: `internal/session/session.go`, `internal/session/store.go`
  - Entry 新增 State(string)、Tokens(int) 字段
  - Store 新增 `Export(session) → .md` 方法（人类可读）

- [ ] **T3** [permissions] TUI 弹窗确认逻辑 · outputs: `internal/permissions/prompt.go`
  - `PromptChecker` 通过 channel 向 TUI 发送 `ConfirmRequestMsg`
  - 超时默认拒绝（30s）

- [ ] **T4** [stream] 流式解析 + 超时重试 · outputs: `internal/stream/stream.go`
  - `RetryChat(ctx, provider, req, maxRetries) → Response` — 指数退避 1s/2s/4s
  - SSE chunk 解析器

- [ ] **T5** [agent] TUI 通信协议消息类型 · outputs: `internal/agent/messages.go`
  - UserInputMsg, ConfirmResponseMsg, AgentStateMsg, StreamChunkMsg, ToolCallMsg, ToolResultMsg, ConfirmRequestMsg, ErrorMsg

---

## Group 2 (并行 — 依赖 Group 1)

LLM 实现层 + 工具系统。依赖 Config(T1)、Stream(T4)、Permissions(T3)。

- [ ] **T6** [llm] OpenAI Provider 实现 + DeepSeek 封装 · outputs: `internal/llm/openai.go`, `internal/llm/deepseek.go`
  - 基于 Config.BaseURL 构造 HTTP client
  - 支持 Chat / Stream 两种模式
  - Tool Call 解析（JSON → llm.ToolCall）
  - DeepSeek 仅封装默认 BaseURL 和模型常量，复用 openai.go
  - **depends**: T1, T4

- [ ] **T7** [llm] Mock Provider 实现 · outputs: `internal/llm/mock.go`
  - 预设对话脚本，支持 tool_call 模拟
  - 用于单元测试，无网络依赖
  - **depends**: T4

- [ ] **T8** [tools] 只读工具集 · outputs: `internal/tools/read.go`
  - list_dir(path) — 遍历目录树，返回文件列表
  - read_file(path) — 读取文件完整内容
  - search_file(pattern) — 按名称模糊匹配
  - search_code(keyword, path) — 关键词/正则搜索
  - 全部 LevelSafe，实现 Tool 接口
  - **depends**: T3 (Level 常量), 现有 tools.Tool 接口

- [ ] **T9** [tools] 文件写入工具集 · outputs: `internal/tools/write.go`
  - write_file(path, content) — 新建/覆盖文件
  - edit_file(path, old_snippet, new_snippet) — 增量替换
  - 全部 LevelSafe
  - **depends**: T3, 现有 tools.Tool 接口

- [ ] **T10** [tools] 高危工具集 · outputs: `internal/tools/shell.go`, `internal/tools/delete.go`
  - shell_exec(cmd, workdir) — 带 30s timeout 的 Shell 执行
  - delete_file(path) — 删除文件/目录
  - 全部 LevelDangerous
  - **depends**: T3, 现有 tools.Tool 接口

---

## Group 3 (并行 — 依赖 Group 2)

核心 Agent Loop + TUI 基础组件。

- [ ] **T11** [agent] Agent Loop 主循环 · outputs: `internal/agent/loop.go`
  - 实现完整推理闭环：session → buildMessages → LLM → tool dispatch → 结果回传
  - 状态机 6 态 (Idle/Thinking/Executing/WaitingApproval/Done/Error)
  - 通过 T5 的消息类型与 TUI 通信（channel + tea.Msg）
  - 高危工具调用前发 ConfirmRequestMsg，等待 ConfirmResponseMsg
  - maxTurns 上限 + 终止判断
  - **depends**: T5, T6, T8, T9, T10

- [ ] **T12** [tui] TUI 样式 + 子组件 · outputs: `internal/tui/styles.go`, `internal/tui/components/{titlebar,messages,input,confirm,status}.go`
  - styles.go — Lipgloss 样式集中定义（title, status, user, agent, tool, error, prompt, help）
  - titlebar.go — 紫色标题栏 + 状态标签
  - messages.go — 可滚动消息列表（自动裁剪 + 流式追加）
  - input.go — 输入行（可编辑 + 历史）
  - confirm.go — 权限确认弹窗覆盖层（Y/N + 超时倒计时）
  - status.go — 状态标签着色映射
  - **depends**: T5 (消息类型用于渲染)

---

## Group 4 (并行 — 依赖 Group 3)

TUI 整合 + 主入口装配 + 严格网关兼容。

- [ ] **T13** [tui] TUI 整合 — Update/View 全链路 · outputs: `internal/tui/update.go`, `internal/tui/render.go`
  - update.go — 处理全部 tea.Msg 类型，连接 Agent goroutine
  - render.go — View() 方法，组合所有子组件
  - 重构现有 tui.go 为 model.go（保持 model struct）
  - **depends**: T11, T12

- [ ] **T14** [cmd] main.go 完整装配线 · outputs: `cmd/omnidev-agent/main.go`
  - Config → LLM Provider → Permissions → Tools Registry → Session → Agent → TUI
  - 启动 Agent goroutine，TUI 主循环
  - 退出信号处理（Ctrl+C 保存 session）
  - **depends**: T1, T6, T11, T13

- [ ] **T15** [llm] 结构化计划解析器 · outputs: `internal/llm/structured_plan.go`
  - 解析 LLM 响应中的 structured plan JSON block
  - 降级策略：解析失败 → 普通 function calling
  - **depends**: T6

---

## Group 5 (并行 — 依赖 Group 4)

测试 + 验证产物。

- [ ] **T16** [tests] 全量测试用例 · outputs: `tests/`
  1. Agent 主循环完整流转（Mock LLM + 多轮推理）
  2. 工具调用发起及结果回传链路
  3. 高危操作权限确认 + 拒绝分支
  4. 多层级配置读取优先级合并
  5. Mock LLM 兼容适配
  - **depends**: T7, T11, T14

- [ ] **T17** [deliverables] 运行验证产物 · outputs: `deliverables/demo_screenshot/`
  - 使用本 Agent 完成终端贪吃蛇小游戏开发
  - 采集全流程 TUI 运行截图
  - **depends**: T14, T16

---

## Phase 3 Extension — v2.0 增强功能

total_extra_tasks: 3
dependencies: Group 1-5 completed

- [ ] **T18** [agent] 历史项目理解拦截器 — ProjectAwarenessGuard · outputs: 
  - 项目类型检测: Greenfield/Legacy/Mixed
  - 四步理解流程自动执行 (list_dir → read_file(README) → search_code → read_file(入口))
  - 硬拦截 write_file/edit_file/delete_file 直到 AwarenessComplete
  - 超时 30s + Step 失败容错
  - 理解结果注入 session context (role: system, [PROJECT ANALYSIS] prefix)
  - **depends**: Group 1-5 completed

- [ ] **T19** [agent] SubAgent 并行调度引擎 — TaskDispatcher · outputs: 
  - TaskPlanner: LLM-driven 任务拆解 (prompt: 输出 JSON )
  - TaskGraph: DAG 依赖解析 → 找出无依赖的 root tasks
  - RunParallel: sync.WaitGroup + goroutine per SubAgent, max 4 并行
  - SubAgent: 独立 session context (继承父 Agent project understanding summary)
  - 父 Agent 汇总所有 SubAgent 结果 → 生成最终回复
  - 超时 120s/subagent, 10 turns max
  - **depends**: T11, T18, Group 1-5 completed

- [ ] **T20** [cmd+tui+tests] 组装 + 测试 · outputs: , , 
  - main.go: Guard + Dispatcher 装配到 Agent
  - TUI: 并行任务进度展示 (multi-task progress bars)
  - TUI: 项目理解状态指示 (scanning/complete)
  - tests: Guard 拦截测试, Dispatcher 并行测试
  - **depends**: T18, T19
