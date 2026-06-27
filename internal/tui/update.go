package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/agent"
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
	case agent.CheckpointPromptMsg:
		m.handleAgentMsg(msg)
	case agent.AllCompleteMsg:
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
	}
	return m, nil
}

// handleKey processes keyboard events.
func (m *model) handleKey(msg tea.KeyMsg) tea.Cmd {
	// During confirmation or checkpoint prompt, only Y/N/A/Esc handled
	if m.confirming || m.checkpointing {
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
				if m.checkpointReply != nil {
					m.checkpointReply <- agent.CheckpointResponse{Resume: false}
				}
				m.checkpointing = false
				m.checkpointReply = nil
				return nil
			}
			if m.confirmReply != nil {
				m.confirmReply <- permissions.ConfirmResponse{Granted: false, Reason: "user denied"}
			}
			m.confirming = false
			m.confirmReply = nil
		}
		return nil
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		if m.cancel != nil {
			m.cancel()
		}
		return tea.Quit

	case tea.KeyEnter:
		input := strings.TrimSpace(m.input.Text())
		if input == "" {
			if t := m.currentTurn(); t != nil && t.HasCollapsibleThinking() {
				t.ToggleThinkExpanded()
			}
			return nil
		}
		if m.isWorking() {
			return nil
		}
		m.input.Submit()
		switch input {
		case "/help":
			m.showHelp()
			return nil
		case "/clear":
			m.turns = components.NewTurnList(50)
			m.turnCount = 0
			return nil
		case "/sessions":
			m.showSessions()
			return nil
		case "/model":
			t := m.newTurn("/model")
			t.StartStep(components.StepThink)
			t.AppendLLM("Current model: " + m.cfg.Model + " (" + m.cfg.Provider + ")")
			t.FlushLLM("")
			t.MarkDone()
			return nil
		case "/status":
			t := m.newTurn("/status")
			t.StartStep(components.StepThink)
			info := NewStatusInfo(m.agent, m.cfg)
			t.AppendLLM(info)
			t.FlushLLM("")
			t.MarkDone()
			return nil
		case "/checkpoint":
			t := m.newTurn("/checkpoint")
			t.StartStep(components.StepThink)
			info := m.buildCheckpointInfo()
			t.AppendLLM(info)
			t.FlushLLM("")
			t.MarkDone()
			return nil
		case "/yolo":
			current := m.agent.Permissions().Interactive()
			m.agent.Permissions().SetInteractive(!current)
			t := m.newTurn("/yolo")
			t.StartStep(components.StepThink)
			if !current {
				t.AppendLLM("Permission mode: interactive (confirm before dangerous ops)")
			} else {
				t.AppendLLM("Permission mode: yolo (all operations auto-approved)")
			}
			t.FlushLLM("")
			t.MarkDone()
			return nil
		case "quit", "exit":
			m.quitting = true
			if m.cancel != nil {
				m.cancel()
			}
			return tea.Quit
		default:
			if strings.HasPrefix(input, "/session ") {
				m.showSession(strings.TrimSpace(strings.TrimPrefix(input, "/session")))
				return nil
			}
			m.newTurn(input)
			return m.startAgentLoop(input)
		}

	case tea.KeyTab:
		if t := m.currentTurn(); t != nil {
			t.ToggleThinkExpanded()
		}
	case tea.KeyEsc:
		if !m.confirming && m.isWorking() && m.cancel != nil {
			m.cancel()
			if t := m.currentTurn(); t != nil {
				t.MarkCancelled()
			}
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
			m.turns.ScrollUp(1<<30, m.transcriptViewportHeight())
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
		if keyUsesInputHistory(msg) {
			m.input.HistPrev()
		} else {
			m.turns.ScrollUp(3, m.transcriptViewportHeight())
		}
	case tea.KeyDown:
		if keyUsesInputHistory(msg) {
			m.input.HistNext()
		} else {
			m.turns.ScrollDown(3, m.transcriptViewportHeight())
		}
	case tea.KeyPgUp:
		m.turns.ScrollUp(m.transcriptViewportHeight()-1, m.transcriptViewportHeight())
	case tea.KeyPgDown:
		m.turns.ScrollDown(m.transcriptViewportHeight()-1, m.transcriptViewportHeight())
	case tea.KeySpace:
		if strings.TrimSpace(m.input.Text()) == "" {
			if t := m.currentTurn(); t != nil && t.HasCollapsibleThinking() {
				t.ToggleThinkExpanded()
				return nil
			}
		}
		m.input.Insert(' ')
	case tea.KeyRunes:
		if m.isWorking() {
			return nil
		}
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

	// ── Pipeline: Classification ──
	case agent.StreamChunkMsg:
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

	case agent.AllCompleteMsg:
		t.SetCompletion(msg.Summary, msg.ProjectDir)
		for _, tk := range t.Tasks {
			if tk.Status != components.StatusFailed {
				tk.Status = components.StatusSuccess
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
		t.MarkError(msg.Error)

	// ── Done ──
	case agent.DoneMsg:
		t.MarkDone()
	}

	if followScroll {
		m.turns.ScrollToBottom()
	}
}

func (m *model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	vh := m.transcriptViewportHeight()
	switch msg.Type {
	case tea.MouseWheelUp:
		m.turns.ScrollUp(3, vh)
	case tea.MouseWheelDown:
		m.turns.ScrollDown(3, vh)
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
	t.StartStep(components.StepThink)
	t.AppendLLM("Built-in commands:\n" +
		"  /help       — show this help\n" +
		"  /clear      — clear all turns\n" +
		"  /sessions   — list archived session/log files\n" +
		"  /session <file> — preview an archive (.md name or path)\n" +
		"  /model      — show current model\n" +
		"  /status     — show agent status\n" +
		"  /yolo       — toggle permission mode (confirm / auto-approve all)\n" +
		"  [A]         — during a permission prompt: allow all remaining ops\n" +
		"  /checkpoint — show in-progress checkpoint\n" +
		"  quit, exit, Ctrl+C — exit\n" +
		"\n" +
		"Keyboard: ↑↓/PgUp/PgDn scroll · Home/End jump (empty input) · Ctrl/Alt+↑↓ history\n" +
		"          Tab/Enter/Space expand Thinking · Esc cancel run · Y/N/A confirm")
	t.FlushLLM("")
	t.MarkDone()
}

// ── Status ───────────────────────────────────────────────────────────────────

func NewStatusInfo(a *agent.Agent, cfg *config.Config) string {
	sb := &strings.Builder{}
	sb.WriteString("Agent Status\n")
	sb.WriteString("  State:        " + a.State().String() + "\n")
	sb.WriteString("  Provider:     " + cfg.Provider + "\n")
	sb.WriteString("  Model:        " + cfg.Model + "\n")
	sb.WriteString("  Base URL:     " + cfg.BaseURL + "\n")
	sb.WriteString("  Max turns:    " + fmt.Sprintf("%d", cfg.MaxTurns) + "\n")
	sb.WriteString("  Timeout:      " + fmt.Sprintf("%d", cfg.Timeout) + "s\n")
	sb.WriteString("  Tools:        " + fmt.Sprintf("%d", a.Toolbox().Count()) + " registered\n")
	if a.Permissions().Interactive() {
		sb.WriteString("  Permissions:  interactive = on")
	} else {
		sb.WriteString("  Permissions:  interactive = off")
	}
	return sb.String()
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
	return sb.String()
}

// prepareStreamChunk classifies a stream chunk. Pipeline markers are trimmed for
// matching; display text keeps original spacing so leading spaces in SSE deltas
// are not stripped (e.g. " How" must not become "How").
func prepareStreamChunk(content string) (appendText, pipelineMarker string, ok bool) {
	if content == "" {
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

func keyUsesInputHistory(msg tea.KeyMsg) bool {
	s := strings.ToLower(msg.String())
	return strings.HasPrefix(s, "ctrl+") || strings.HasPrefix(s, "alt+")
}

func (m *model) showSessions() {
	t := m.newTurn("/sessions")
	t.StartStep(components.StepThink)

	files, err := session.ListArchives(".ai_history/sessions", ".ai_history/logs", 20)
	if err != nil {
		t.AppendLLM("Failed to list archives: " + err.Error())
	} else if len(files) == 0 {
		t.AppendLLM("No archived sessions under .ai_history/sessions/ or .ai_history/logs/")
	} else {
		var sb strings.Builder
		sb.WriteString("Archived sessions (newest first):\n\n")
		for i, f := range files {
			sb.WriteString(fmt.Sprintf("  %d. [%s] %s  (%s, %d bytes)\n",
				i+1, f.Kind, f.Name, f.ModTime.Format("2006-01-02 15:04"), f.Size))
		}
		sb.WriteString("\nPreview: /session <filename>\n")
		sb.WriteString("Full logs: .ai_history/logs/YYYYMMDD-session.md (one file per day)")
		t.AppendLLM(sb.String())
	}
	t.FlushLLM("")
	t.MarkDone()
}

func (m *model) showSession(arg string) {
	t := m.newTurn("/session " + arg)
	t.StartStep(components.StepThink)

	if arg == "" {
		t.AppendLLM("Usage: /session <filename>  e.g. /session 20260626-233232.md")
		t.FlushLLM("")
		t.MarkDone()
		return
	}

	path := arg
	if !strings.ContainsAny(arg, `/\`) {
		for _, dir := range []string{".ai_history/sessions", ".ai_history/logs"} {
			candidate := filepath.Join(dir, arg)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}

	preview, err := session.ReadArchivePreview(path, 12000)
	if err != nil {
		t.AppendLLM("Cannot read archive: " + err.Error())
	} else {
		t.AppendLLM(fmt.Sprintf("File: %s\n\n%s", path, preview))
	}
	t.FlushLLM("")
	t.MarkDone()
}
