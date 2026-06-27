package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ── Turn Step Types ──────────────────────────────────────────────────────────

type StepType int

const (
	StepClassify StepType = iota
	StepScan
	StepScaffold
	StepPlan
	StepThink
	StepExec
)

func (s StepType) String() string {
	switch s {
	case StepClassify:
		return "classify"
	case StepScan:
		return "scan"
	case StepScaffold:
		return "scaffold"
	case StepPlan:
		return "plan"
	case StepThink:
		return "think"
	case StepExec:
		return "exec"
	default:
		return "unknown"
	}
}

func (s StepType) Label() string {
	switch s {
	case StepClassify:
		return "Classifying"
	case StepScan:
		return "Scanning"
	case StepScaffold:
		return "Scaffolding"
	case StepPlan:
		return "Planning"
	case StepThink:
		return "Thinking"
	case StepExec:
		return "Executing"
	default:
		return "Working"
	}
}

func (s StepType) Icon() string {
	switch s {
	case StepClassify:
		return "~"
	case StepScan:
		return ">"
	case StepScaffold:
		return "#"
	case StepPlan:
		return "="
	case StepThink:
		return "*"
	case StepExec:
		return "@"
	default:
		return "."
	}
}

// ── Status ───────────────────────────────────────────────────────────────────

type ItemStatus int

const (
	StatusPending ItemStatus = iota
	StatusRunning
	StatusSuccess
	StatusFailed
	StatusBlocked
)

func (s ItemStatus) Icon() string {
	switch s {
	case StatusPending:
		return "o"
	case StatusRunning:
		return ">"
	case StatusSuccess:
		return "v"
	case StatusFailed:
		return "x"
	case StatusBlocked:
		return "!"
	default:
		return " "
	}
}

// ── Tool Entry ───────────────────────────────────────────────────────────────

type ToolEntry struct {
	Name      string
	Args      string
	Status    ItemStatus
	Summary   string
	Error     string
	startedAt time.Time
}

// ── Task Entry ───────────────────────────────────────────────────────────────

type TaskEntry struct {
	ID          string
	Description string
	Status      ItemStatus
	DependsOn   []string
	Result      string
}

// ── Pipeline Step ────────────────────────────────────────────────────────────

type PipelineStep struct {
	Type    StepType
	Status  ItemStatus
	summary string
}

// ── Turn ─────────────────────────────────────────────────────────────────────

type TurnFinalStatus int

const (
	TurnRunning TurnFinalStatus = iota
	TurnDone
	TurnError
	TurnCancelled
)

type Turn struct {
	ID           int
	UserInput    string
	startedAt    time.Time
	finishedAt   time.Time
	FinalStatus  TurnFinalStatus
	ErrorMsg     string
	active       bool
	spinnerFrame int
	viaAgent     bool // true when backed by agent.RunLoop (turn log written there)

	Steps       []*PipelineStep
	currentStep StepType

	llmOutput strings.Builder
	streaming bool

	// Thinking collapsed state (false = collapsed, true = expanded)
	ThinkExpanded bool

	ToolCalls []*ToolEntry
	Tasks     []*TaskEntry
	planSummary string

	// User-facing assistant reply (conversation / final answer)
	replyOutput strings.Builder
	chatMode    bool

	// Shown when all tasks complete
	completionMsg string
	projectDir    string
}

func NewTurn(id int, input string) *Turn {
	return &Turn{
		ID:          id,
		UserInput:   input,
		startedAt:   time.Now(),
		FinalStatus: TurnRunning,
	}
}

// ── Turn mutation methods ────────────────────────────────────────────────────

func (t *Turn) SetViaAgent(v bool) { t.viaAgent = v }
func (t *Turn) ViaAgent() bool     { return t.viaAgent }
func (t *Turn) StartedAt() time.Time { return t.startedAt }

// LogContent returns the assistant-visible text for collaboration logs.
func (t *Turn) LogContent() string {
	if t.replyOutput.Len() > 0 {
		return t.replyOutput.String()
	}
	if t.completionMsg != "" {
		return t.completionMsg
	}
	if t.llmOutput.Len() > 0 {
		return t.llmOutput.String()
	}
	if t.ErrorMsg != "" {
		return t.ErrorMsg
	}
	return ""
}

func (t *Turn) ensureStep(typ StepType) *PipelineStep {
	for _, s := range t.Steps {
		if s.Type == typ {
			return s
		}
	}
	s := &PipelineStep{Type: typ, Status: StatusRunning}
	t.Steps = append(t.Steps, s)
	t.currentStep = typ
	return s
}

func (t *Turn) StartStep(typ StepType)  { t.ensureStep(typ) }

func (t *Turn) CompleteStep(typ StepType, summary string) {
	s := t.ensureStep(typ)
	s.Status = StatusSuccess
	if summary != "" {
		s.summary = summary
	}
}

func (t *Turn) FailStep(typ StepType, err string) {
	s := t.ensureStep(typ)
	s.Status = StatusFailed
	s.summary = err
}

func (t *Turn) AppendLLM(text string) {
	t.llmOutput.WriteString(text)
	t.streaming = true
}

func (t *Turn) FlushLLM(text string) {
	if text != "" {
		t.llmOutput.WriteString(text)
	}
	t.streaming = false
}

func (t *Turn) AddToolCall(name, args string) *ToolEntry {
	te := &ToolEntry{
		Name:      name,
		Args:      args,
		Status:    StatusRunning,
		startedAt: time.Now(),
	}
	t.ToolCalls = append(t.ToolCalls, te)
	return te
}

func (t *Turn) CompleteToolCall(name string, success bool, output string) {
	for i := len(t.ToolCalls) - 1; i >= 0; i-- {
		te := t.ToolCalls[i]
		if te.Status != StatusRunning {
			continue
		}
		if name == "" || te.Name == name {
			if success {
				te.Status = StatusSuccess
				te.Summary = output
			} else {
				te.Status = StatusFailed
				te.Error = output
			}
			return
		}
	}
}

func (t *Turn) SetTasks(tasks []*TaskEntry) { t.Tasks = tasks }

func (t *Turn) AddOrUpdateTask(id, description string, status ItemStatus) {
	for _, tk := range t.Tasks {
		if tk.ID == id {
			tk.Status = status
			if description != "" {
				tk.Description = description
			}
			return
		}
	}
	t.Tasks = append(t.Tasks, &TaskEntry{ID: id, Description: description, Status: status})
}

func (t *Turn) UpdateTaskStatus(id string, status ItemStatus, result string) {
	for _, tk := range t.Tasks {
		if tk.ID == id {
			tk.Status = status
			if result != "" {
				tk.Result = result
			}
			return
		}
	}
}

func (t *Turn) SetPlanSummary(s string) { t.planSummary = s }
func (t *Turn) SetActive(v bool)        { t.active = v }
func (t *Turn) IsActive() bool          { return t.active }
func (t *Turn) SetChatMode(v bool) { t.chatMode = v }
func (t *Turn) IsChatMode() bool   { return t.chatMode }

func (t *Turn) AppendReply(text string) {
	t.replyOutput.WriteString(text)
}

func (t *Turn) FlushReply(text string) {
	if text != "" {
		t.replyOutput.WriteString(text)
	}
}

func (t *Turn) SetCompletion(summary, projectDir string) {
	t.projectDir = projectDir
	t.completionMsg = summary
}

func (t *Turn) HasCompletion() bool { return strings.TrimSpace(t.completionMsg) != "" }

func (t *Turn) TickSpinner() {
	if t.active {
		t.spinnerFrame = (t.spinnerFrame + 1) % 4
	}
}

func (t *Turn) SpinnerIcon() string {
	if !t.active {
		return ""
	}
	frames := []string{">", ">>", ">>>", ">>"}
	return frames[t.spinnerFrame%4]
}

func (t *Turn) ToggleThinkExpanded() {
	t.ThinkExpanded = !t.ThinkExpanded
}

func (t *Turn) HasCollapsibleThinking() bool {
	return t.llmOutput.Len() > 0 && !t.chatMode && !t.streaming
}

func (t *Turn) MarkDone() {
	t.FinalStatus = TurnDone
	t.finishedAt = time.Now()
	t.active = false
	for _, s := range t.Steps {
		if s.Status == StatusRunning {
			s.Status = StatusSuccess
		}
	}
}

func (t *Turn) MarkError(err string) {
	t.FinalStatus = TurnError
	t.ErrorMsg = err
	t.finishedAt = time.Now()
	t.active = false
}

func (t *Turn) MarkCancelled() {
	t.FinalStatus = TurnCancelled
	t.finishedAt = time.Now()
	t.active = false
}

// WorkingLabel returns a Cursor-style status line for the working indicator.
func (t *Turn) WorkingLabel(agentState string) string {
	if t == nil {
		return "Working"
	}
	switch agentState {
	case "WaitingApproval":
		return "Waiting for approval"
	case "Thinking":
		return "Thinking"
	case "Executing":
		for i := len(t.ToolCalls) - 1; i >= 0; i-- {
			tc := t.ToolCalls[i]
			if tc.Status == StatusRunning {
				if tc.Args != "" {
					return fmt.Sprintf("Running %s %s", tc.Name, truncateStr(tc.Args, 48))
				}
				return "Running " + tc.Name
			}
		}
		return "Executing"
	}
	if t.currentStep != 0 {
		return t.currentStep.Label()
	}
	if t.streaming {
		return "Thinking"
	}
	return "Working"
}

func (t *Turn) Duration() time.Duration {
	if t.FinalStatus == TurnRunning {
		return time.Since(t.startedAt)
	}
	return t.finishedAt.Sub(t.startedAt)
}

// ── TurnList ─────────────────────────────────────────────────────────────────

func wrapLine(text string, width int) []string {
	return WrapDisplayWidth(text, width)
}

type TurnList struct {
	turns          []*Turn
	maxTurns       int
	width          int
	pinnedToBottom bool // true = follow latest output
	firstVisible   int  // top line index when pinnedToBottom is false
	omitTasksFrom  *Turn // set during View; hide duplicate task box in scroll body
}

func NewTurnList(maxTurns int) *TurnList {
	return &TurnList{
		turns:          make([]*Turn, 0, maxTurns),
		maxTurns:       maxTurns,
		pinnedToBottom: true,
	}
}

// ScrollUp moves the viewport toward older content.
func (tl *TurnList) ScrollUp(lines, viewportHeight int) {
	if lines < 1 {
		lines = 1
	}
	all := tl.buildAllLines()
	total := len(all)
	if total <= viewportHeight {
		return
	}
	maxStart := total - viewportHeight
	if tl.pinnedToBottom {
		tl.pinnedToBottom = false
		tl.firstVisible = maxStart
	}
	tl.firstVisible -= lines
	if tl.firstVisible < 0 {
		tl.firstVisible = 0
	}
	if tl.firstVisible > maxStart {
		tl.firstVisible = maxStart
	}
}

// ScrollDown moves the viewport toward newer content.
func (tl *TurnList) ScrollDown(lines, viewportHeight int) {
	if lines < 1 {
		lines = 1
	}
	all := tl.buildAllLines()
	total := len(all)
	if total <= viewportHeight {
		tl.pinnedToBottom = true
		tl.firstVisible = 0
		return
	}
	maxStart := total - viewportHeight
	if tl.pinnedToBottom {
		return
	}
	tl.firstVisible += lines
	if tl.firstVisible >= maxStart {
		tl.firstVisible = maxStart
		tl.pinnedToBottom = true
	}
}

// ScrollToBottom pins the viewport to the latest output.
func (tl *TurnList) ScrollToBottom() {
	tl.pinnedToBottom = true
	tl.firstVisible = 0
}

// AtBottom reports whether the viewport follows the latest output.
func (tl *TurnList) AtBottom() bool { return tl.pinnedToBottom }

// ScrollHint returns a short footer hint when not pinned to bottom.
func (tl *TurnList) ScrollHint(viewportHeight int) string {
	if tl.pinnedToBottom {
		return ""
	}
	total := len(tl.buildAllLines())
	maxStart := total - viewportHeight
	if maxStart < 1 {
		return ""
	}
	if tl.firstVisible > maxStart {
		tl.firstVisible = maxStart
	}
	return fmt.Sprintf("scroll %d/%d · End latest", tl.firstVisible, maxStart)
}

func (tl *TurnList) viewStart(total, viewportHeight int) int {
	if total <= viewportHeight {
		return 0
	}
	maxStart := total - viewportHeight
	if tl.pinnedToBottom {
		return maxStart
	}
	start := tl.firstVisible
	if start > maxStart {
		start = maxStart
	}
	if start < 0 {
		start = 0
	}
	return start
}

func (tl *TurnList) buildAllLines() []string {
	if len(tl.turns) == 0 {
		return nil
	}
	var allLines []string
	for i, t := range tl.turns {
		if i > 0 {
			allLines = append(allLines, "", "")
		}
		skipTasks := tl.omitTasksFrom != nil && t == tl.omitTasksFrom
		allLines = append(allLines, t.render(tl.width, skipTasks)...)
	}
	return allLines
}

// TaskPanelLines renders the sticky To-dos panel for a turn.
func TaskPanelLines(t *Turn, width int) []string {
	if t == nil || len(t.Tasks) == 0 {
		return nil
	}
	return RenderTodoList(t.Tasks, width)
}

// View renders the scrollable transcript. Pass pinTasksTurn when its task box
// is shown in the sticky panel above (avoids duplicate rendering).
func (tl *TurnList) View(viewportHeight int, pinTasksTurn *Turn) string {
	if viewportHeight < 1 {
		viewportHeight = 3
	}
	prev := tl.omitTasksFrom
	tl.omitTasksFrom = pinTasksTurn
	allLines := tl.buildAllLines()
	tl.omitTasksFrom = prev

	if len(allLines) == 0 {
		return ""
	}
	total := len(allLines)
	if total <= viewportHeight {
		return strings.Join(allLines, "\n")
	}

	start := tl.viewStart(total, viewportHeight)
	if !tl.pinnedToBottom {
		tl.firstVisible = start
	}
	return strings.Join(allLines[start:start+viewportHeight], "\n")
}

func (tl *TurnList) AddTurn(id int, input string) *Turn {
	t := NewTurn(id, input)
	tl.turns = append(tl.turns, t)
	tl.trim()
	return t
}

func (tl *TurnList) LastTurn() *Turn {
	if len(tl.turns) == 0 {
		return nil
	}
	return tl.turns[len(tl.turns)-1]
}

func (tl *TurnList) SetWidth(w int) {
	if w < 40 {
		w = 80
	}
	tl.width = w
}

func (tl *TurnList) Count() int { return len(tl.turns) }

func (tl *TurnList) TickSpinner() {
	if t := tl.LastTurn(); t != nil {
		t.TickSpinner()
	}
}

func (tl *TurnList) trim() {
	if len(tl.turns) > tl.maxTurns*2 {
		tl.turns = tl.turns[len(tl.turns)-tl.maxTurns:]
	}
}

// ── Rendering ────────────────────────────────────────────────────────────────

var (
	turnUserStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	turnAssistStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#D1D5DB"))
	turnStepStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	turnStepOKStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	turnStepErrStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	turnStepRunStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FBBF24"))
	turnToolStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	turnOkStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	turnErrStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#F87171"))
	turnPendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	turnThinkLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
)

const thinkPreviewLines = 3 // used by tests / future preview mode

func (t *Turn) render(width int, skipTasks bool) []string {
	if width < 40 {
		width = 80
	}
	cw := width - 2
	var lines []string

	for _, wl := range wrapLine(t.UserInput, cw) {
		lines = append(lines, turnUserStyle.Render(wl))
	}
	lines = append(lines, "")

	if !skipTasks && len(t.Tasks) > 0 {
		lines = append(lines, RenderTodoList(t.Tasks, width)...)
	}

	if t.llmOutput.Len() > 0 && !t.chatMode {
		lines = append(lines, t.renderThinking(cw)...)
	}

	if len(t.ToolCalls) > 0 {
		lines = append(lines, t.renderToolCalls(cw)...)
	}

	if t.replyOutput.Len() > 0 {
		output := strings.TrimRight(t.replyOutput.String(), "\n")
		style := turnAssistStyle
		if t.chatMode {
			style = turnUserStyle
		}
		for _, line := range strings.Split(output, "\n") {
			for _, wl := range wrapLine(line, cw) {
				lines = append(lines, style.Render(wl))
			}
		}
		lines = append(lines, "")
	}

	if t.FinalStatus == TurnCancelled {
		lines = append(lines, turnErrStyle.Render("  Cancelled"))
		lines = append(lines, "")
	}
	if t.FinalStatus == TurnError && t.ErrorMsg != "" {
		for _, wl := range wrapLine(t.ErrorMsg, cw) {
			lines = append(lines, turnErrStyle.Render(wl))
		}
		lines = append(lines, "")
	}

	return lines
}

func (t *Turn) renderToolCalls(cw int) []string {
	var lines []string
	for _, tc := range t.ToolCalls {
		var icon string
		var style lipgloss.Style
		switch tc.Status {
		case StatusSuccess:
			icon, style = "✓", turnStepOKStyle
		case StatusFailed:
			icon, style = "✗", turnStepErrStyle
		case StatusRunning:
			icon, style = "◌", turnStepRunStyle
		default:
			icon, style = "○", turnPendingStyle
		}
		head := fmt.Sprintf("  %s %s", icon, tc.Name)
		if tc.Args != "" {
			head += " " + tc.Args
		}
		for _, wl := range wrapLine(head, cw) {
			lines = append(lines, style.Render(wl))
		}
		detail := tc.Summary
		if tc.Status == StatusFailed && tc.Error != "" {
			detail = tc.Error
		}
		if detail != "" {
			for _, wl := range wrapLine("    → "+detail, cw) {
				lines = append(lines, turnToolStyle.Render(wl))
			}
		}
	}
	lines = append(lines, "")
	return lines
}

func (t *Turn) renderThinking(cw int) []string {
	var lines []string
	output := strings.TrimRight(t.llmOutput.String(), "\n")
	allThinkLines := strings.Split(output, "\n")
	lineCount := len(allThinkLines)

	if t.streaming {
		lines = append(lines, turnThinkLabel.Render("  Thinking..."))
		for _, line := range allThinkLines {
			for _, wl := range wrapLine(line, cw) {
				lines = append(lines, turnAssistStyle.Render(wl))
			}
		}
		return lines
	}

	if t.ThinkExpanded {
		lines = append(lines, turnThinkLabel.Render(fmt.Sprintf("  v Thinking (%d lines)  Tab/Enter to collapse", lineCount)))
		for _, line := range allThinkLines {
			for _, wl := range wrapLine(line, cw) {
				lines = append(lines, turnAssistStyle.Render(wl))
			}
		}
	} else {
		lines = append(lines, turnThinkLabel.Render(fmt.Sprintf("  > Thinking (%d lines)  Tab/Enter to expand", lineCount)))
		preview := thinkPreviewLines
		if lineCount < preview {
			preview = lineCount
		}
		for i := 0; i < preview; i++ {
			for _, wl := range wrapLine(allThinkLines[i], cw) {
				lines = append(lines, turnAssistStyle.Render(wl))
			}
		}
		if lineCount > preview {
			lines = append(lines, turnThinkLabel.Render(fmt.Sprintf("    … %d more lines", lineCount-preview)))
		}
	}
	lines = append(lines, "")
	return lines
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
