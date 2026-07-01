package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
	"github.com/zayeagle/omnidev-agent/internal/commands"
	"github.com/zayeagle/omnidev-agent/internal/config"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/tui/components"
)

// Update handles all tea.Msg types for the TUI model.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Unwrap agentMsgBatch — dispatch inner msg, re-cue channel read
	if batch, ok := msg.(agentMsgBatch); ok {
		m.handleAgentMsg(batch.msg)
		m.agentCh = batch.ch
		if batch.ch != nil {
			return m, readAgentMsg(batch.ch)
		}
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	// ── Direct agent messages (first message, before channel loop starts) ──
	case agent.AgentStateMsg:
		m.handleAgentMsg(msg)
	case agent.StreamChunkMsg:
		m.handleAgentMsg(msg)
	case agent.ToolCallMsg:
		m.handleAgentMsg(msg)
	case agent.ToolResultMsg:
		m.handleAgentMsg(msg)
	case agent.SubtaskMsg:
		m.handleAgentMsg(msg)
	case agent.TaskPlanMsg:
		m.handleAgentMsg(msg)
	case agent.TaskPlanConfirmMsg:
		m.handleAgentMsg(msg)
	case agent.CheckpointPromptMsg:
		m.handleAgentMsg(msg)
	case agent.AllCompleteMsg:
		m.handleAgentMsg(msg)
	case agent.VerificationProgressMsg:
		m.handleAgentMsg(msg)
	case agent.PartialCompleteMsg:
		m.handleAgentMsg(msg)
	case agent.ConfirmRequestMsg:
		m.handleAgentMsg(msg)
		return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return confirmTickMsg{}
		})
	case agent.ResolveConflictMsg:
		m.handleAgentMsg(msg)
	case agent.ErrorMsg:
		m.handleAgentMsg(msg)
	case agent.DoneMsg:
		m.handleAgentMsg(msg)

	// ── Confirm timeout tick ──
	case confirmTickMsg:
		if !m.confirming {
			return m, nil
		}
		m.confirmTimeout--
		if m.confirmTimeout <= 0 {
			if m.confirmReply != nil {
				m.confirmReply <- permissions.ConfirmResponse{Granted: false, Reason: "timeout"}
			}
			m.confirming = false
			m.confirmReply = nil
			return m, nil
		}
		return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return confirmTickMsg{}
		})

	// ── Spinner animation tick ──
	case spinnerTickMsg:
		if m.isWorking() {
			m.spinnerFrame = (m.spinnerFrame + 1) % 10
			m.turns.TickSpinner()
			return m, tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
				return spinnerTickMsg{}
			})
		}
		return m, nil

	// ── Keyboard input ──
	case tea.KeyMsg:
		return m, m.handleKey(msg)

	// ── Mouse wheel scroll (when enabled by terminal) ──
	case tea.MouseMsg:
		return m, m.handleMouse(msg)

	// ── Agent loop started (from startAgentLoop tea.Cmd) ──
	case agentLoopStartedMsg:
		m.agentCh = msg.ch
		return m, readAgentMsg(msg.ch)

	case agentLoopEndedMsg:
		m.agentCh = nil
		if t := m.currentTurn(); t != nil && t.FinalStatus == components.TurnRunning {
			t.MarkDone()
		}
		m.persistActiveSession()
	}
	return m, nil
}

// handleKey processes keyboard events.
func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyCtrlY {
		m.applyPermissionToggle()
		return nil
	}

	if msg.Type == tea.KeyEnter && isQuitCommand(m.input.Text()) {
		return m.quitSession()
	}

	// During confirmation, checkpoint, or plan review — limited keys only
	if m.confirming || m.checkpointing || m.planConfirming {
		if msg.Type == tea.KeyCtrlC {
			return m.handleCtrlC()
		}
		if m.planConfirming {
			switch msg.Type {
			case tea.KeyEnter:
				if strings.TrimSpace(m.input.Text()) != "" {
					return nil
				}
				if m.planConfirmReply != nil {
					m.planConfirmReply <- agent.TaskPlanConfirmResponse{Confirmed: true}
				}
				m.planConfirming = false
				m.planConfirmReply = nil
				return nil
			case tea.KeyEsc:
				m.interruptSession()
				return nil
			}
			// Allow typing quit/exit in the input while reviewing the plan.
		} else if strings.TrimSpace(m.input.Text()) == "" {
			switch strings.ToLower(msg.String()) {
			case "y":
				if m.checkpointing {
					if m.checkpointReply != nil {
						m.checkpointReply <- agent.CheckpointResponse{Resume: true}
					}
					m.checkpointing = false
					m.checkpointReply = nil
					return nil
				}
				if m.confirmReply != nil {
					m.confirmReply <- permissions.ConfirmResponse{Granted: true, Reason: "user approved"}
				}
				m.confirming = false
				m.confirmReply = nil
			case "a":
				if m.checkpointing {
					return nil
				}
				m.agent.Permissions().SetInteractive(false)
				if m.confirmReply != nil {
					m.confirmReply <- permissions.ConfirmResponse{
						Granted:  true,
						Reason:   "allow all",
						AllowAll: true,
					}
				}
				m.confirming = false
				m.confirmReply = nil
			case "n", "ctrl+c", "esc":
				if m.checkpointing {
					m.checkpointing = false
					m.checkpointReply = nil
					m.interruptSession()
					return nil
				}
				if m.confirmReply != nil {
					m.confirmReply <- permissions.ConfirmResponse{Granted: false, Reason: "user denied"}
				}
				m.confirming = false
				m.confirmReply = nil
				return nil
			}
		}
		if m.confirming || m.checkpointing {
			return nil
		}
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		return m.handleCtrlC()

	case tea.KeyEnter:
		input := strings.TrimSpace(m.input.Text())
		if input != "" && isSessionSlashCommand(input) {
			m.applyBuiltinCommand(input)
			return nil
		}
		if input == "" {
			if t := m.currentTurn(); t != nil && t.HasCollapsibleThinking() {
				t.ToggleThinkExpanded()
				return nil
			}
			return nil
		}
		if m.isWorking() {
			return nil
		}
		m.input.PushHistory(input)
		m.input.Submit()
		t := m.newTurn(input)
		t.SetViaAgent(true)
		return m.startAgentLoop(input)

	case tea.KeyTab:
		if t := m.currentTurn(); t != nil {
			t.ToggleThinkExpanded()
		}
	case tea.KeyEsc:
		if !m.confirming && m.isWorking() {
			m.interruptSession()
		}
	case tea.KeyBackspace:
		if m.isWorking() {
			return nil
		}
		m.input.DeleteBefore()
	case tea.KeyDelete:
		if m.isWorking() {
			return nil
		}
		m.input.DeleteAfter()
	case tea.KeyLeft:
		if m.isWorking() {
			return nil
		}
		m.input.MoveLeft()
	case tea.KeyRight:
		if m.isWorking() {
			return nil
		}
		m.input.MoveRight()
	case tea.KeyHome:
		if m.input.Text() == "" {
			m.scrollUp(1 << 30)
		} else {
			m.input.MoveHome()
		}
	case tea.KeyEnd:
		if m.input.Text() == "" {
			m.turns.ScrollToBottom()
		} else {
			m.input.MoveEnd()
		}
	case tea.KeyUp:
		if m.isWorking() || keyScrollsTranscript(msg) {
			m.scrollUp(3)
		} else {
			m.input.HistPrev()
		}
	case tea.KeyDown:
		if m.isWorking() || keyScrollsTranscript(msg) {
			m.scrollDown(3)
		} else {
			m.input.HistNext()
		}
	case tea.KeyPgUp:
		m.scrollUp(m.scrollViewportHeight() - 1)
	case tea.KeyPgDown:
		m.scrollDown(m.scrollViewportHeight() - 1)
	case tea.KeySpace:
		if strings.TrimSpace(m.input.Text()) == "" {
			if t := m.currentTurn(); t != nil && t.HasCollapsibleThinking() {
				t.ToggleThinkExpanded()
				return nil
			}
		}
		m.input.Insert(' ')
	case tea.KeyRunes:
		for _, r := range msg.Runes {
			// Some Windows terminals emit DEL/BS as runes instead of KeyBackspace.
			if r == '\b' || r == 127 {
				m.input.DeleteBefore()
				continue
			}
			m.input.Insert(r)
		}
	default:
		// Fallback for terminals that alias backspace to ctrl+h.
		if msg.String() == "ctrl+h" {
			m.input.DeleteBefore()
		}
	}
	return nil
}

// handleAgentMsg dispatches individual agent messages to update the Turn structure.
func (m *model) handleAgentMsg(msg tea.Msg) {
	t := m.currentTurn()
	if t == nil {
		return
	}
	followScroll := m.turns.AtBottom()
	defer func() {
		if followScroll {
			m.turns.ScrollToBottom()
		}
	}()

	switch msg := msg.(type) {
	case agent.AgentStateMsg:
		m.agentState = msg.State.String()

	case agent.ActivityMsg:
		if t != nil {
			t.SetActivityDetail(msg.Detail)
			if isMajorProcessMessage(msg.Detail) {
				t.AddStatusLine("→ " + msg.Detail)
			}
		}

	// ── Pipeline: Classification ──
	case agent.StreamChunkMsg:
		trimmed := strings.TrimSpace(msg.Content)
		if strings.HasPrefix(trimmed, "── Acceptance verification") || strings.HasPrefix(trimmed, "── Acceptance failure") {
			t.ShowAcceptanceReport(trimmed, false, 0, 0)
			break
		}
		if strings.Contains(trimmed, "Requirements analysis") {
			t.StartStep(components.StepAnalyze)
			t.CompleteStep(components.StepAnalyze, "requirements")
			body := strings.TrimPrefix(trimmed, "Requirements analysis:")
			body = strings.TrimSpace(body)
			if body != "" {
				t.AppendReply(body)
			}
			if msg.Done {
				t.FlushReply("")
			}
			break
		}
		if isAgentProcessMessage(trimmed) {
			t.AddStatusLine("→ " + trimmed)
			break
		}
		if appendText, marker, ok := prepareStreamChunk(msg.Content); ok {
			if marker != "" {
				t.AddStatusLine(marker)
				handlePipelineMarker(t, marker)
			} else if t.IsChatMode() {
				if msg.Done {
					t.FlushReply(appendText)
				} else {
					t.AppendReply(appendText)
				}
			} else {
				t.StartStep(components.StepThink)
				if msg.Done {
					t.FlushLLM(appendText)
				} else {
					t.AppendLLM(appendText)
				}
			}
		}

	case agent.TaskPlanMsg:
		items := make([]*components.TaskEntry, 0, len(msg.Tasks))
		for _, item := range msg.Tasks {
			items = append(items, &components.TaskEntry{
				ID:          item.ID,
				Description: item.Description,
				Status:      components.StatusPending,
				DependsOn:   append([]string(nil), item.DependsOn...),
			})
		}
		t.SetTasks(items)
		t.StartStep(components.StepPlan)
		t.CompleteStep(components.StepPlan, fmt.Sprintf("%d tasks", len(items)))

	case agent.TaskPlanConfirmMsg:
		m.planConfirming = true
		m.planConfirmReply = msg.Reply
		if t != nil {
			t.AddStatusLine(fmt.Sprintf("Review %d sub-tasks — Enter to execute, Esc to cancel", msg.TaskCount))
		}

	case agent.AllCompleteMsg:
		if !t.IsChatMode() {
			if strings.TrimSpace(msg.AcceptanceDetail) != "" {
				t.SetCompletionAcceptance(msg.Summary, msg.ProjectDir, msg.AcceptanceDetail, msg.AcceptancePassed, msg.AcceptancePassedN, msg.AcceptanceTotalN)
			} else {
				t.SetCompletion(msg.Summary, msg.ProjectDir)
			}
			for _, tk := range t.Tasks {
				if tk.Status != components.StatusFailed && tk.Status != components.StatusSuccess {
					tk.Status = components.StatusSuccess
				}
			}
		}
		m.turns.ScrollToBottom()

	case agent.VerificationProgressMsg:
		if !t.IsChatMode() {
			if msg.InitChecklist {
				texts := make([]string, len(msg.Criteria))
				for i, c := range msg.Criteria {
					texts[i] = c.Text
				}
				t.InitAcceptanceChecklist(texts)
				break
			}
			if msg.AppendText != "" {
				t.AppendAcceptanceCheck(msg.AppendText)
			}
			if msg.CheckedIndex >= 0 && msg.CheckedIndex < len(msg.Criteria) {
				c := msg.Criteria[msg.CheckedIndex]
				t.UpdateAcceptanceCheck(msg.CheckedIndex, c.Met, c.Evidence)
			}
			if msg.Finalize && msg.Total > 0 {
				t.FinishAcceptanceChecklist(msg.Passed, msg.Total, msg.AllMet)
			}
			// Full report is shown in collapsible completion panel, not inline during success path.
			if msg.Detail != "" && !msg.AllMet {
				t.AddStatusBlock(msg.Detail)
			}
		}

	case agent.PartialCompleteMsg:
		if !t.IsChatMode() {
			detail := strings.TrimSpace(msg.AcceptanceDetail)
			passedN, totalN := msg.AcceptancePassedN, msg.AcceptanceTotalN
			if detail == "" && len(msg.Criteria) > 0 {
				detail = formatPartialAcceptanceDetail(msg.Criteria)
				passedN = 0
				for _, c := range msg.Criteria {
					if c.Met {
						passedN++
					}
				}
				totalN = len(msg.Criteria)
			}
			if detail != "" {
				t.SetCompletionReport(msg.Summary, msg.ProjectDir, detail, msg.AcceptancePassed, passedN, totalN, true, msg.Reason)
			} else {
				t.SetCompletionReport(msg.Summary, msg.ProjectDir, "", false, 0, 0, true, msg.Reason)
			}
			t.MarkError(msg.Reason)
			for _, tk := range t.Tasks {
				if tk.Status == components.StatusRunning || tk.Status == components.StatusPending {
					tk.Status = components.StatusFailed
				}
			}
			if msg.Resumable {
				t.AddStatusLine("Acceptance incomplete — restart and choose Resume to continue from checkpoint.")
			}
		}
		m.turns.ScrollToBottom()

	// ── Tool calls ──
	case agent.ToolCallMsg:
		t.StartStep(components.StepExec)
		t.AddToolCallSubtask(msg.SubtaskID, msg.Name, toolArgsSummary(msg.Args))

	case agent.ToolResultMsg:
		toolName := t.PendingToolName()
		if msg.Success {
			summary := components.SummarizeToolResult(toolName, true, msg.Data, "")
			t.CompleteToolCall("", true, summary)
			if fc, ok := components.ParseChangeFromToolResult(toolName, msg.Data); ok {
				t.RecordFileChange(fc)
			}
		} else {
			summary := components.SummarizeToolResult(toolName, false, "", msg.Error)
			t.CompleteToolCall("", false, summary)
		}

	// ── Subtask (dispatcher) ──
	case agent.SubtaskMsg:
		t.StartStep(components.StepPlan)
		desc := msg.Label
		switch msg.Status {
		case "running":
			t.AddOrUpdateTask(msg.TaskID, desc, components.StatusRunning)
		case "done":
			t.UpdateTaskStatus(msg.TaskID, components.StatusSuccess, "")
		case "error":
			t.UpdateTaskStatus(msg.TaskID, components.StatusFailed, desc)
		}
		t.RecomputeTaskBlocked()

	// ── Conflict / checkpoint ──
	case agent.ResolveConflictMsg:
		if msg.HasInProgress {
			t.AppendLLM("Found in-progress checkpoint (phase: " + msg.LastPhase + "). Resume or start fresh.")
			t.FlushLLM("")
		}

	case agent.CheckpointPromptMsg:
		m.checkpointing = true
		m.checkpointPhase = msg.Phase
		m.checkpointDone = msg.Completed
		m.checkpointTotal = msg.Total
		m.checkpointReply = msg.Reply
		if t != nil {
			t.AddStatusLine(fmt.Sprintf("Checkpoint: %d/%d tasks done (%s)", msg.Completed, msg.Total, msg.Phase))
			if msg.AcceptanceIncomplete {
				t.AddStatusLine("Previous run stopped at acceptance gate — Resume to continue.")
			}
		}

	// ── Confirmation ──
	case agent.ConfirmRequestMsg:
		m.confirming = true
		m.confirmLevel = msg.Level.String()
		m.confirmDescription = msg.Description
		m.confirmPreview = msg.Preview
		m.confirmReply = msg.Reply
		m.confirmTimeout = 30

	// ── Error ──
	case agent.ErrorMsg:
		if msg.Error == "interrupted" || msg.Error == "cancelled" {
			if t.FinalStatus == components.TurnRunning {
				t.MarkInterrupted()
			}
		} else {
			t.MarkError(msg.Error)
		}

	// ── Done ──
	case agent.DoneMsg:
		t.MarkDone()
	}

	if followScroll {
		m.turns.ScrollToBottom()
	}
}

func (m *model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	switch msg.Type {
	case tea.MouseWheelUp:
		m.scrollUp(3)
	case tea.MouseWheelDown:
		m.scrollDown(3)
	}
	return nil
}

// toolArgsSummary produces a short human-readable summary of tool arguments.
func toolArgsSummary(args map[string]interface{}) string {
	if path, ok := args["path"].(string); ok {
		return path
	}
	if cmd, ok := args["cmd"].(string); ok {
		if len(cmd) > 50 {
			return cmd[:50] + "..."
		}
		return cmd
	}
	if keyword, ok := args["keyword"].(string); ok {
		return "search: " + keyword
	}
	if len(args) == 0 {
		return ""
	}
	// Generic: just show first key=value
	for k, v := range args {
		return fmt.Sprintf("%s=%v", k, v)
	}
	return ""
}

// parseTaskUpdate extracts task ID and status from a status report line.
func parseTaskUpdate(t *components.Turn, line string, status components.ItemStatus) {
	// Format: "[Task 1] running: description"
	rest := line
	if idx := strings.Index(rest, "Task "); idx >= 0 {
		rest = rest[idx+5:] // after "Task "
	}
	if idx := strings.Index(rest, "]"); idx > 0 {
		id := rest[:idx]
		desc := ""
		if colonIdx := strings.Index(rest, ": "); colonIdx >= 0 {
			desc = rest[colonIdx+2:]
		}
		if t.Tasks == nil || len(t.Tasks) == 0 {
			t.AddOrUpdateTask(id, desc, status)
		} else {
			t.UpdateTaskStatus(id, status, "")
		}
	}
}

// ── startAgentLoop / readAgentMsg ────────────────────────────────────────────

func (m *model) startAgentLoop(instruction string) tea.Cmd {
	instruction = m.agent.PrepareRunInstruction(instruction)
	spawn := func() tea.Msg {
		msgCh := make(chan tea.Msg, 64)
		ctx, cancel := context.WithCancel(context.Background())
		m.ctx = ctx
		m.cancel = cancel

		go func() {
			defer close(msgCh)
			_ = m.agent.RunLoop(ctx, instruction, msgCh)
		}()

		return agentLoopStartedMsg{ch: msgCh}
	}

	return tea.Batch(
		spawn,
		tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
			return spinnerTickMsg{}
		}),
	)
}

func (m *model) startCheckpointRollback(taskID string) tea.Cmd {
	if taskID == "" {
		t := m.newTurn("/checkpoint rollback")
		t.SetCommandOutput("Usage: /checkpoint rollback <task_id>")
		return nil
	}
	d := m.agent.Dispatcher()
	if d == nil {
		t := m.newTurn("/checkpoint rollback " + taskID)
		t.SetCommandOutput("Dispatcher not available.")
		return nil
	}
	m.newTurn("/checkpoint rollback " + taskID)
	spawn := func() tea.Msg {
		msgCh := make(chan tea.Msg, 64)
		ctx, cancel := context.WithCancel(context.Background())
		m.ctx = ctx
		m.cancel = cancel
		go func() {
			defer close(msgCh)
			_, _ = d.Rollback(ctx, taskID, msgCh)
			msgCh <- agent.DoneMsg{}
		}()
		return agentLoopStartedMsg{ch: msgCh}
	}
	return tea.Batch(
		spawn,
		tea.Tick(150*time.Millisecond, func(t time.Time) tea.Msg {
			return spinnerTickMsg{}
		}),
	)
}

func readAgentMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return agentLoopEndedMsg{}
		}
		return agentMsgBatch{msg: msg, ch: ch}
	}
}

// ── Help ─────────────────────────────────────────────────────────────────────

func (m *model) showHelp() {
	t := m.newTurn("/help")
	t.SetCommandOutput(commands.HelpText())
}

// ── Status ───────────────────────────────────────────────────────────────────

func NewStatusInfo(a *agent.Agent, cfg *config.Config) string {
	sb := &strings.Builder{}
	sb.WriteString("Agent Status\n")
	sb.WriteString("  State:        " + a.State().String() + "\n")
	sb.WriteString("  Provider:     " + cfg.Provider + "\n")
	sb.WriteString("  Model:        " + cfg.Model + "\n")
	sb.WriteString("  Base URL:     " + cfg.BaseURL + "\n")
	sb.WriteString("  Max turns:    " + config.FormatTurnLimit(cfg.MaxTurns) + "\n")
	sb.WriteString("  Sub-agent max turns: " + config.FormatTurnLimit(cfg.SubAgentMaxTurns) + "\n")
	sb.WriteString("  Timeout:      " + fmt.Sprintf("%d", cfg.Timeout) + "s\n")
	sb.WriteString("  Tools:        " + fmt.Sprintf("%d", a.Toolbox().Count()) + " registered\n")
	if a.Permissions().Interactive() {
		sb.WriteString("  Permissions:  confirm (dangerous ops need approval)\n")
	} else {
		sb.WriteString("  Permissions:  yolo (auto-approve all dangerous ops)\n")
	}
	maxTok := cfg.EffectiveContextMaxTokens()
	thresh := cfg.EffectiveContextSummarizeThreshold()
	sb.WriteString(fmt.Sprintf("  Context:      %.0f%% of %d tokens (compact at %.0f%%)\n",
		a.ContextUsagePct(), maxTok, thresh*100))
	if cat := a.SkillCatalog(); cat != nil {
		sb.WriteString(fmt.Sprintf("  Skills:       %d loaded\n", cat.Count()))
	}
	if mgr := a.MCPManager(); mgr != nil {
		sb.WriteString("  " + mgr.Summary() + "\n")
	}
	return sb.String()
}

func (m *model) showSkills() {
	t := m.newTurn("/skills")
	cat := m.agent.SkillCatalog()
	if cat == nil || cat.Count() == 0 {
		t.SetCommandOutput("No skills loaded.\n\nAdd SKILL.md under:\n  ~/.omnidev-agent/skills/<name>/SKILL.md\n  .omnidev-agent/skills/<name>/SKILL.md\n\nOr set skills_dirs in config.")
		return
	}
	var sb strings.Builder
	sb.WriteString("Loaded skills:\n")
	for _, sk := range cat.List() {
		sb.WriteString(fmt.Sprintf("  • %s — %s\n    %s\n", sk.Name, sk.Description, sk.Path))
	}
	sb.WriteString("\nLoad: /skill <name>  or ask the agent to load_skill")
	t.SetCommandOutput(sb.String())
}

func (m *model) loadSkillCommand(name string) {
	t := m.newTurn("/skill " + name)
	cat := m.agent.SkillCatalog()
	if cat == nil {
		t.SetCommandOutput("Skill catalog not initialized.")
		return
	}
	sk, ok := cat.Get(name)
	if !ok {
		t.SetCommandOutput("Skill not found: " + name + "\nUse /skills to list.")
		return
	}
	m.agent.Session().AddWithState("system", "[SKILL: "+sk.Name+"]\n"+sk.Body, "skill", 0)
	t.SetCommandOutput(fmt.Sprintf("Loaded skill %q into session context.\n%s", sk.Name, sk.Description))
}

func (m *model) buildCheckpointInfo() string {
	cpStore := agent.NewCheckpointStore(".ai_history/checkpoints/")
	cp, err := cpStore.Load()
	if err != nil {
		return "Checkpoint read error: " + err.Error()
	}
	if cp == nil {
		return "No checkpoint found."
	}
	sb := &strings.Builder{}
	sb.WriteString(fmt.Sprintf("Checkpoint — Phase: %s | Tasks: %d total, %d completed\n",
		cp.Phase, len(cp.Tasks), len(cp.Results)))
	for _, r := range cp.Results {
		status := "OK"
		if !r.Success {
			status = "FAILED"
		}
		sb.WriteString(fmt.Sprintf("  [%s] task %s: %s\n", status, r.TaskID, r.Content))
	}
	for _, t := range cp.Tasks {
		found := false
		for _, r := range cp.Results {
			if r.TaskID == t.ID {
				found = true
				break
			}
		}
		if !found {
			sb.WriteString(fmt.Sprintf("  [pending] task %s: %s\n", t.ID, t.Description))
		}
	}
	sb.WriteString("\nRollback: /checkpoint rollback <task_id>\n")
	return sb.String()
}

// prepareStreamChunk classifies a stream chunk. Pipeline markers are trimmed for
// matching; display text keeps original spacing so leading spaces in SSE deltas
// are not stripped (e.g. " How" must not become "How").
func prepareStreamChunk(content string) (appendText, pipelineMarker string, ok bool) {
	if content == "" {
		return "", "", false
	}
	if looksLikeDSMLToolMarkup(content) {
		return "", "", false
	}
	trimmed := strings.TrimSpace(content)
	if trimmed != "" && isPipelineNoise(trimmed) {
		return "", trimmed, true
	}
	return content, "", true
}

func isPipelineNoise(content string) bool {
	markers := []string{
		"Conversation mode",
		"Code modification mode",
		"Legacy project detected",
		"New project detected",
		"Project workspace ready",
		"Standalone project workspace",
		"Project analysis complete",
		"Architecture:",
		"Single task — executing",
		"Task planning failed",
		"DDD structure ready",
		"Decomposed into",
		"Falling back to sequential",
		"Task Plan:",
		"Found in-progress checkpoint",
		"Scanning project structure",
		"Initializing DDD",
		"Scaffold warning:",
		"Workspace warning:",
		"All sub-tasks completed",
		"[Task ",
		"[SUB-TASK RESULTS]",
	}
	for _, m := range markers {
		if strings.Contains(content, m) {
			return true
		}
	}
	return false
}

// looksLikeDSMLToolMarkup detects raw model tool-call markup that should not appear in chat UI.
func looksLikeDSMLToolMarkup(content string) bool {
	if !strings.Contains(content, "DSML") {
		return false
	}
	return strings.Contains(content, "invoke name=") ||
		strings.Contains(content, "tool_calls") ||
		strings.Contains(content, "parameter name=")
}

func handlePipelineMarker(t *components.Turn, content string) {
	switch {
	case strings.Contains(content, "Conversation mode"):
		t.SetChatMode(true)
		t.StartStep(components.StepClassify)
		t.CompleteStep(components.StepClassify, "conversation")
	case strings.Contains(content, "Code modification mode"):
		t.StartStep(components.StepClassify)
		t.CompleteStep(components.StepClassify, "code change")
	case strings.Contains(content, "Legacy project detected"), strings.Contains(content, "Scanning project structure"):
		t.StartStep(components.StepScan)
	case strings.Contains(content, "Project analysis complete"):
		t.CompleteStep(components.StepScan, "analysis done")
	case strings.Contains(content, "Initializing DDD"), strings.Contains(content, "DDD structure ready"):
		t.StartStep(components.StepScaffold)
		t.CompleteStep(components.StepScaffold, "scaffold")
	case strings.Contains(content, "Architecture:"):
		t.StartStep(components.StepPlan)
	case strings.Contains(content, "Project workspace ready"), strings.Contains(content, "Standalone project workspace"):
		t.SetPlanSummary(content)
		t.CompleteStep(components.StepScaffold, "workspace ready")
	case strings.Contains(content, "[Task "):
		parseAllTaskLines(t, content)
		t.RecomputeTaskBlocked()
	}
}

// parseAllTaskLines updates task status from each "[Task N] status: ..." line independently.
func parseAllTaskLines(t *components.Turn, content string) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "[Task ") {
			continue
		}
		var status components.ItemStatus
		switch {
		case strings.Contains(line, "running:"):
			status = components.StatusRunning
		case strings.Contains(line, "done:"):
			status = components.StatusSuccess
		case strings.Contains(line, "waiting:"):
			status = components.StatusPending
		case strings.Contains(line, "blocked:"):
			status = components.StatusBlocked
		default:
			continue
		}
		parseTaskUpdate(t, line, status)
	}
}

func isAgentProcessMessage(content string) bool {
	return isMajorProcessMessage(content)
}

func isMajorProcessMessage(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	if strings.Contains(strings.ToLower(content), "calling model") {
		return false
	}
	markers := []string{
		"Checking acceptance",
		"Final acceptance",
		"Working ·",
		"Acceptance ",
		"recovery round",
		"autonomous recovery",
		"Review incomplete",
		"Network error",
		"LLM unreachable",
		"auto-reconnecting",
		"Stopped:",
		"── Acceptance",
		"[ACCEPTANCE",
	}
	for _, m := range markers {
		if strings.Contains(content, m) {
			return true
		}
	}
	return false
}

func keyScrollsTranscript(msg tea.KeyMsg) bool {
	s := strings.ToLower(msg.String())
	return strings.HasPrefix(s, "ctrl+") || strings.HasPrefix(s, "alt+")
}

func (m *model) showSessions() {
	t := m.newTurn("/sessions")

	dir := m.cfg.RuntimeSessionDir()
	summaries, err := session.ListSessionSummaries(dir, 20)
	if err != nil {
		t.SetCommandOutput("Failed to list sessions: " + err.Error())
		return
	}
	if len(summaries) == 0 {
		t.SetCommandOutput("No saved sessions in " + dir + "\n\nSessions are saved when a turn completes.")
		return
	}
	var sb strings.Builder
	sb.WriteString("Recent sessions (newest first):\n\n")
	for i, s := range summaries {
		sb.WriteString(session.FormatSessionSummaryLine(i+1, s))
		sb.WriteString("\n")
	}
	sb.WriteString("\nOpen detail: /session <id>  e.g. /session ")
	sb.WriteString(summaries[0].ID)
	sb.WriteString("\nFiles: ")
	sb.WriteString(dir)
	t.SetCommandOutput(sb.String())
}

func (m *model) showSession(arg string) {
	t := m.newTurn("/session " + arg)

	if arg == "" {
		t.SetCommandOutput("Usage: /session <id>  e.g. /session 20260626-233232")
		return
	}

	dir := m.cfg.RuntimeSessionDir()
	detail, err := session.LoadSessionDetail(dir, arg, 8)
	if err != nil {
		// Fallback: markdown export if JSON missing
		path := arg
		if !strings.ContainsAny(arg, `/\`) {
			path = filepath.Join(dir, arg)
			if !strings.HasSuffix(path, ".md") && !strings.HasSuffix(path, ".json") {
				path = filepath.Join(dir, arg+".md")
			}
		}
		preview, readErr := session.ReadArchivePreview(path, 12000)
		if readErr != nil {
			t.SetCommandOutput("Cannot load session: " + err.Error())
			return
		}
		t.SetCommandOutput(fmt.Sprintf("File: %s\n\n%s", path, preview))
		return
	}
	t.SetCommandOutput(detail)
}

func formatPartialAcceptanceDetail(criteria []agent.CriterionStatus) string {
	var b strings.Builder
	b.WriteString("── Acceptance failure detail ──\n")
	for _, c := range criteria {
		mark := "FAIL"
		if c.Met {
			mark = "PASS"
		}
		b.WriteString(fmt.Sprintf("[%s] %s\n", mark, c.Text))
		if ev := strings.TrimSpace(c.Evidence); ev != "" {
			b.WriteString(fmt.Sprintf("      reason: %s\n", ev))
		}
	}
	return strings.TrimSpace(b.String())
}
