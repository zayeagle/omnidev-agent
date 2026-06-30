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
	StepAnalyze
	StepScan
	StepScaffold
	StepPlan
	StepThink
	StepExec
	StepVerify
)

func (s StepType) String() string {
	switch s {
	case StepClassify:
		return "classify"
	case StepAnalyze:
		return "analyze"
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
	case StepVerify:
		return "verify"
	default:
		return "unknown"
	}
}

func (s StepType) Label() string {
	switch s {
	case StepClassify:
		return "Classifying"
	case StepAnalyze:
		return "Analyzing requirements"
	case StepScan:
		return "Scanning"
	case StepScaffold:
		return "Scaffolding"
	case StepPlan:
		return "Planning"
	case StepThink:
		return "Thinking"
	case StepExec:
		return "Working"
	case StepVerify:
		return "Working"
	default:
		return "Working"
	}
}

func (s StepType) Icon() string {
	switch s {
	case StepClassify:
		return "~"
	case StepAnalyze:
		return "◆"
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
	case StepVerify:
		return "?"
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
	StatusSkipped
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
	case StatusSkipped:
		return "-"
	default:
		return " "
	}
}

// ── Tool Entry ───────────────────────────────────────────────────────────────

type ToolEntry struct {
	Name      string
	Args      string
	SubtaskID string
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

	// Live activity detail for the working indicator (e.g. "Calling model turn 3/20")
	activityDetail string

	ToolCalls []*ToolEntry
	Tasks     []*TaskEntry
	planSummary string

	// User-facing assistant reply (conversation / final answer)
	replyOutput strings.Builder
	chatMode    bool

	// Pipeline status (architecture, workspace ready, etc.) — separate from LLM reply
	statusLines      []string
	pendingContentGap bool

	// Shown when all tasks complete
	completionMsg string
	projectDir    string
	TasksExpanded bool // collapsed under completion banner by default

	acceptanceDetail   string
	AcceptanceExpanded bool
	acceptancePassed   bool
	acceptancePassedN  int
	acceptanceTotalN   int

	fileChanges []FileChange

	// Live acceptance checklist (○ → ✓ during verify phase)
	acceptanceChecks []AcceptanceCheckItem
}

// AcceptanceCheckItem is one row in the acceptance criteria checklist.
type AcceptanceCheckItem struct {
	Text     string
	Status   ItemStatus
	Evidence string
}

func (t *Turn) InitAcceptanceChecklist(criteria []string) {
	t.acceptanceChecks = make([]AcceptanceCheckItem, 0, len(criteria))
	for _, c := range criteria {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		t.acceptanceChecks = append(t.acceptanceChecks, AcceptanceCheckItem{
			Text:   c,
			Status: StatusPending,
		})
	}
	t.StartStep(StepVerify)
}

func (t *Turn) AppendAcceptanceCheck(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	t.acceptanceChecks = append(t.acceptanceChecks, AcceptanceCheckItem{
		Text:   text,
		Status: StatusPending,
	})
}

func (t *Turn) UpdateAcceptanceCheck(index int, met bool, evidence string) {
	if index < 0 || index >= len(t.acceptanceChecks) {
		return
	}
	evidence = strings.TrimSpace(evidence)
	if isSkippedEvidence(evidence) {
		t.acceptanceChecks[index].Status = StatusSkipped
	} else if met {
		t.acceptanceChecks[index].Status = StatusSuccess
	} else {
		t.acceptanceChecks[index].Status = StatusFailed
	}
	t.acceptanceChecks[index].Evidence = evidence
}

func isSkippedEvidence(evidence string) bool {
	lower := strings.ToLower(evidence)
	return strings.Contains(lower, "skipped") ||
		strings.Contains(lower, "n/a") ||
		strings.Contains(evidence, "不适用")
}

func (t *Turn) FinishAcceptanceChecklist(passed, total int, allMet bool) {
	summary := "verification"
	if total > 0 {
		summary = fmt.Sprintf("%d/%d criteria", passed, total)
	}
	if allMet {
		t.CompleteStep(StepVerify, summary)
	} else {
		t.FailStep(StepVerify, summary)
	}
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
	if text == "" {
		return
	}
	if t.pendingContentGap {
		t.beginContentGap(&t.llmOutput)
		t.pendingContentGap = false
	}
	t.llmOutput.WriteString(text)
	t.streaming = true
	t.ThinkExpanded = true
}

func (t *Turn) FlushLLM(text string) {
	if text != "" {
		t.AppendLLM(text)
	}
	t.finishContentSegment(&t.llmOutput)
	t.streaming = false
}

func (t *Turn) AddToolCall(name, args string) *ToolEntry {
	return t.AddToolCallSubtask("", name, args)
}

func (t *Turn) AddToolCallSubtask(subtaskID, name, args string) *ToolEntry {
	te := &ToolEntry{
		Name:      name,
		Args:      args,
		SubtaskID: subtaskID,
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

func (t *Turn) SetTasks(tasks []*TaskEntry) {
	t.Tasks = tasks
	SortTasksByID(t.Tasks)
	t.RecomputeTaskBlocked()
}

func (t *Turn) RecomputeTaskBlocked() {
	if len(t.Tasks) == 0 {
		return
	}
	done := make(map[string]bool, len(t.Tasks))
	for _, tk := range t.Tasks {
		if tk.Status == StatusSuccess {
			done[tk.ID] = true
		}
	}
	for _, tk := range t.Tasks {
		if tk.Status == StatusRunning || tk.Status == StatusSuccess || tk.Status == StatusFailed {
			continue
		}
		blocked := false
		for _, dep := range tk.DependsOn {
			if !done[dep] {
				blocked = true
				break
			}
		}
		if blocked {
			tk.Status = StatusBlocked
		} else if tk.Status == StatusBlocked {
			tk.Status = StatusPending
		}
	}
}

func (t *Turn) AddOrUpdateTask(id, description string, status ItemStatus) {
	for _, tk := range t.Tasks {
		if tk.ID == id {
			tk.Status = status
			if description != "" {
				tk.Description = description
			}
			t.RecomputeTaskBlocked()
			return
		}
	}
	t.Tasks = append(t.Tasks, &TaskEntry{ID: id, Description: description, Status: status})
	t.RecomputeTaskBlocked()
}

func (t *Turn) UpdateTaskStatus(id string, status ItemStatus, result string) {
	for _, tk := range t.Tasks {
		if tk.ID == id {
			tk.Status = status
			if result != "" {
				tk.Result = result
			}
			t.RecomputeTaskBlocked()
			return
		}
	}
}

func (t *Turn) SetPlanSummary(s string) { t.planSummary = s }
func (t *Turn) SetActive(v bool) { t.active = v }

func (t *Turn) SetActivityDetail(detail string) {
	t.activityDetail = strings.TrimSpace(detail)
}
func (t *Turn) IsActive() bool          { return t.active }
func (t *Turn) SetChatMode(v bool) { t.chatMode = v }
func (t *Turn) IsChatMode() bool   { return t.chatMode }

func (t *Turn) AppendReply(text string) {
	if text == "" {
		return
	}
	if t.pendingContentGap {
		t.beginContentGap(&t.replyOutput)
		t.pendingContentGap = false
	}
	t.replyOutput.WriteString(text)
}

func (t *Turn) FlushReply(text string) {
	if text != "" {
		t.AppendReply(text)
	}
	t.finishContentSegment(&t.replyOutput)
}

// SetCommandOutput renders built-in slash-command results as visible reply text (not Thinking).
func (t *Turn) SetCommandOutput(text string) {
	if text != "" {
		t.AppendReply(text)
	}
	t.FlushReply("")
	t.MarkDone()
}

func (t *Turn) AddStatusBlock(block string) {
	block = strings.TrimSpace(block)
	if block == "" {
		return
	}
	for _, line := range strings.Split(block, "\n") {
		t.AddStatusLine(line)
	}
}

// ShowAcceptanceReport prints the full verification analysis (always visible, not collapsed).
func (t *Turn) ShowAcceptanceReport(detail string, allMet bool, passed, total int) {
	if detail == "" {
		return
	}
	t.StartStep(StepVerify)
	t.AddStatusBlock(detail)
	summary := "verification"
	if total > 0 {
		summary = fmt.Sprintf("%d/%d criteria", passed, total)
	}
	if allMet {
		t.CompleteStep(StepVerify, summary)
	} else {
		t.FailStep(StepVerify, summary+" — see report above")
	}
}

func (t *Turn) AddStatusLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if len(t.statusLines) > 0 && t.statusLines[len(t.statusLines)-1] == line {
		return
	}
	t.statusLines = append(t.statusLines, line)
	t.pendingContentGap = true
}

func (t *Turn) beginContentGap(b *strings.Builder) {
	s := strings.TrimRight(b.String(), " \t")
	if s == "" {
		return
	}
	b.Reset()
	b.WriteString(s)
	b.WriteString("\n\n")
}

func (t *Turn) finishContentSegment(b *strings.Builder) {
	s := b.String()
	if s == "" {
		return
	}
	if !strings.HasSuffix(s, "\n") {
		b.WriteString("\n")
	}
}

func (t *Turn) SetCompletion(summary, projectDir string) {
	t.SetCompletionAcceptance(summary, projectDir, "", true, 0, 0)
}

func (t *Turn) SetCompletionAcceptance(summary, projectDir, acceptanceDetail string, passed bool, passedN, totalN int) {
	t.projectDir = projectDir
	t.acceptanceDetail = strings.TrimSpace(acceptanceDetail)
	t.acceptancePassed = passed
	t.acceptancePassedN = passedN
	t.acceptanceTotalN = totalN
	t.AcceptanceExpanded = false
	t.TasksExpanded = false
	if t.acceptanceDetail != "" {
		t.completionMsg = strings.TrimSpace(summary)
	} else {
		t.completionMsg = summary
	}
}

func (t *Turn) ToggleAcceptanceExpanded() {
	if strings.TrimSpace(t.acceptanceDetail) == "" {
		return
	}
	t.AcceptanceExpanded = !t.AcceptanceExpanded
}

func (t *Turn) AcceptanceDetailText() string { return t.acceptanceDetail }

func (t *Turn) HasAcceptanceChecklist() bool {
	return len(t.acceptanceChecks) > 0 && !t.HasCompletion()
}

func (t *Turn) ToggleTasksExpanded() {
	if len(t.Tasks) == 0 {
		return
	}
	t.TasksExpanded = !t.TasksExpanded
}

func (t *Turn) HasCompletion() bool {
	return strings.TrimSpace(t.projectDir) != "" || strings.TrimSpace(t.completionMsg) != ""
}

func (t *Turn) ReplyText() string       { return t.replyOutput.String() }
func (t *Turn) LLMText() string         { return t.llmOutput.String() }
func (t *Turn) CompletionText() string  { return t.completionMsg }
func (t *Turn) ProjectDirText() string  { return t.projectDir }

func (t *Turn) RestoreReply(text string) {
	t.replyOutput.Reset()
	t.replyOutput.WriteString(text)
}

func (t *Turn) RestoreLLM(text string) {
	t.llmOutput.Reset()
	t.llmOutput.WriteString(text)
}

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

// normalizeActivityLabel ensures task execution activity reads as Working, not bare "Calling model".
func normalizeActivityLabel(detail string) string {
	d := strings.TrimSpace(detail)
	if d == "" {
		return "Working"
	}
	if strings.HasPrefix(d, "Working") || strings.HasPrefix(d, "Waiting") || strings.HasPrefix(d, "Thinking") {
		return d
	}
	if strings.Contains(strings.ToLower(d), "calling model") {
		return "Working · " + d
	}
	return d
}

// WorkingLabel returns a Cursor-style status line for the working indicator.
func (t *Turn) WorkingLabel(agentState string) string {
	if t == nil {
		return "Working"
	}
	switch agentState {
	case "WaitingApproval":
		return "Waiting for approval"
	case "Verifying":
		if t.activityDetail != "" {
			return normalizeActivityLabel(t.activityDetail)
		}
		return "Working · verifying acceptance…"
	case "Thinking":
		if t.activityDetail != "" {
			return normalizeActivityLabel(t.activityDetail)
		}
		if t.streaming {
			return "Thinking · streaming…"
		}
		return "Thinking · waiting for model…"
	case "Executing", "Working":
		if t.activityDetail != "" {
			return normalizeActivityLabel(t.activityDetail)
		}
		for i := len(t.ToolCalls) - 1; i >= 0; i-- {
			tc := t.ToolCalls[i]
			if tc.Status == StatusRunning {
				if tc.Args != "" {
					return fmt.Sprintf("Working · %s %s", tc.Name, truncateStr(tc.Args, 48))
				}
				return "Working · " + tc.Name
			}
		}
		return "Working"
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
	omitTasksFrom      *Turn // set during View; hide duplicate task box in scroll body
	omitAcceptanceFrom *Turn
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
		skipAcceptance := tl.omitAcceptanceFrom != nil && t == tl.omitAcceptanceFrom
		allLines = append(allLines, t.render(tl.width, skipTasks, skipAcceptance)...)
	}
	return allLines
}

// AcceptancePanelLines renders the sticky acceptance checklist during verify.
func AcceptancePanelLines(t *Turn, width int) []string {
	if t == nil || len(t.acceptanceChecks) == 0 || t.HasCompletion() {
		return nil
	}
	return t.renderAcceptanceChecklist(width)
}

// TaskPanelLines renders the sticky To-dos panel for a turn.
func TaskPanelLines(t *Turn, width int, liveStatus string) []string {
	if t == nil || len(t.Tasks) == 0 || t.HasCompletion() {
		return nil
	}
	collapse := t.FinalStatus == TurnDone && !t.IsActive()
	return RenderTodoList(t.Tasks, width, collapse, liveStatus)
}

// View renders the scrollable transcript. Pass pinTasksTurn when its task box
// is shown in the sticky panel above (avoids duplicate rendering).
func (tl *TurnList) View(viewportHeight int, pinTasksTurn *Turn) string {
	if viewportHeight < 1 {
		viewportHeight = 3
	}
	prev := tl.omitTasksFrom
	tl.omitTasksFrom = pinTasksTurn
	if pinTasksTurn != nil && len(pinTasksTurn.acceptanceChecks) > 0 && !pinTasksTurn.HasCompletion() {
		tl.omitAcceptanceFrom = pinTasksTurn
	} else {
		tl.omitAcceptanceFrom = nil
	}
	allLines := tl.buildAllLines()
	tl.omitTasksFrom = prev
	tl.omitAcceptanceFrom = nil

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

func (tl *TurnList) AllTurns() []*Turn {
	out := make([]*Turn, len(tl.turns))
	copy(out, tl.turns)
	return out
}

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

const thinkPreviewLines = 2 // collapsed shows first N lines + expand hint

func (t *Turn) render(width int, skipTasks bool, skipAcceptance bool) []string {
	if width < 40 {
		width = 80
	}
	cw := width - 2
	var lines []string

	for _, wl := range wrapLine(t.UserInput, cw) {
		lines = append(lines, turnUserStyle.Render(wl))
	}
	lines = append(lines, "")

	if len(t.Steps) > 0 {
		lines = append(lines, t.renderPipelineSteps(cw)...)
	}

	if len(t.statusLines) > 0 {
		for _, sl := range t.statusLines {
			for _, wl := range wrapLine(sl, cw) {
				lines = append(lines, turnStepStyle.Render(wl))
			}
			lines = append(lines, "")
		}
	}

	if !skipAcceptance && len(t.acceptanceChecks) > 0 && !t.HasCompletion() {
		lines = append(lines, t.renderAcceptanceChecklist(cw)...)
	}

	if !skipTasks && len(t.Tasks) > 0 && !t.HasCompletion() {
		collapse := t.FinalStatus == TurnDone
		lines = append(lines, RenderTodoList(t.Tasks, width, collapse, "")...)
	}

	if t.llmOutput.Len() > 0 && !t.chatMode && !t.HasCompletion() {
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

	if t.FinalStatus == TurnDone && len(t.fileChanges) > 0 {
		lines = append(lines, ChangePanelLines(t, width)...)
	}

	return lines
}

func (t *Turn) renderToolCalls(cw int) []string {
	var lines []string
	argsIndent := "      "
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
		if tc.SubtaskID != "" {
			head = fmt.Sprintf("  %s [task %s] %s", icon, tc.SubtaskID, tc.Name)
		}
		for _, wl := range wrapLine(head, cw) {
			lines = append(lines, style.Render(wl))
		}
		if tc.Args != "" {
			args := tc.Args
			if len(args) > cw*3 {
				args = TruncateMiddle(args, cw*3)
			}
			argsWidth := cw - len(argsIndent)
			if argsWidth < 12 {
				argsWidth = 12
			}
			for _, wl := range wrapLine(args, argsWidth) {
				lines = append(lines, turnToolStyle.Render(argsIndent+wl))
			}
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
		if thinkPreviewLines > 0 {
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
	}
	lines = append(lines, "")
	return lines
}

func (t *Turn) renderAcceptanceChecklist(cw int) []string {
	var lines []string
	lines = append(lines, turnStepRunStyle.Render("  Acceptance criteria:"))
	for _, ac := range t.acceptanceChecks {
		icon, style := todoStatusIcon(ac.Status)
		if ac.Status == StatusSkipped {
			style = turnPendingStyle
		}
		label := fmt.Sprintf("  %s %s", icon, ac.Text)
		for _, wl := range wrapLine(label, cw) {
			lines = append(lines, style.Render(wl))
		}
		if ac.Evidence != "" && ac.Status != StatusPending {
			prefix := "→ "
			if ac.Status == StatusSkipped {
				prefix = "skip: "
			}
			for _, wl := range wrapLine("      "+prefix+ac.Evidence, cw) {
				lines = append(lines, turnStepStyle.Render(wl))
			}
		}
	}
	lines = append(lines, "")
	return lines
}

func (t *Turn) renderPipelineSteps(cw int) []string {
	var lines []string
	for _, s := range t.Steps {
		var icon string
		var style lipgloss.Style
		switch s.Status {
		case StatusSuccess:
			icon, style = "✓", turnStepOKStyle
		case StatusFailed:
			icon, style = "✗", turnStepErrStyle
		case StatusRunning:
			icon, style = "◌", turnStepRunStyle
		default:
			icon, style = "○", turnPendingStyle
		}
		label := fmt.Sprintf("  %s %s %s", icon, s.Type.Icon(), s.Type.Label())
		if s.summary != "" && s.Status != StatusRunning {
			label += " — " + truncateStr(s.summary, 40)
		}
		for _, wl := range wrapLine(label, cw) {
			lines = append(lines, style.Render(wl))
		}
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return lines
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
