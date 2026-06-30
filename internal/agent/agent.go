package agent

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/mcp"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/runlog"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/skills"
	"github.com/zayeagle/omnidev-agent/internal/stream"
	"github.com/zayeagle/omnidev-agent/internal/tools"
)

type State int

const (
	StateIdle            State = 0
	StateThinking        State = 1
	StateExecuting       State = 2
	StateWaitingApproval State = 3
	StateDone            State = 4
	StateError           State = 5
	StateVerifying       State = 6
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateThinking:
		return "Thinking"
	case StateExecuting:
		return "Working"
	case StateWaitingApproval:
		return "WaitingApproval"
	case StateDone:
		return "Done"
	case StateError:
		return "Error"
	case StateVerifying:
		return "Working"
	default:
		return "Unknown"
	}
}

type Agent struct {
	state       State
	provider    llm.Provider
	permChecker *permissions.Checker
	toolbox     *tools.Registry
	session      *session.Session
	store        *session.Store
	turnLogStore *session.Store
	maxTurns     int
	ctxMgr               *ContextManager
	classifier           *Classifier
	complexityClassifier *ComplexityClassifier
	guard                *ProjectAwarenessGuard
	dispatcher           *TaskDispatcher
	cpStore              *CheckpointStore
	mu                   sync.Mutex
	subAgent             bool // true when this is a sub-agent (skip pipeline steps)
	activeSubtaskID      string // task ID when running as sub-agent (for TUI labels)
	outputDir            string // generated project workspace root
	projectLayout        ProjectLayout
	retryConfig          stream.RetryConfig
	maxConsecutiveDenials int
	pipelineOpts         PipelineOptions
	contextSlim          ContextSlimOptions
	skillCatalog         *skills.Catalog
	mcpManager           *mcp.Manager
	acceptancePlan       *AcceptancePlan
	acceptanceStrict     bool
	readCache            *sessionReadCache
	runLog               *runlog.Logger
	pendingMech          mechanicalVerifyResult
}

func New(provider llm.Provider, permChecker *permissions.Checker, toolbox *tools.Registry, sess *session.Session) *Agent {
	return &Agent{
		state:                 StateIdle,
		provider:              provider,
		permChecker:           permChecker,
		toolbox:               toolbox,
		session:               sess,
		maxTurns:              20,
		retryConfig:           stream.DefaultRetryConfig(),
		maxConsecutiveDenials: 3,
		pipelineOpts:          DefaultPipelineOptions(),
		contextSlim:           DefaultContextSlimOptions(),
		acceptanceStrict:      true,
		readCache:             newSessionReadCache(),
	}
}

func (a *Agent) SetMaxTurns(n int)                     { a.maxTurns = n }
func (a *Agent) SetContextManager(cm *ContextManager)   { a.ctxMgr = cm }
func (a *Agent) SetGuard(g *ProjectAwarenessGuard)      { a.guard = g }
func (a *Agent) SetClassifier(c *Classifier)                   { a.classifier = c }
func (a *Agent) SetComplexityClassifier(c *ComplexityClassifier) { a.complexityClassifier = c }
func (a *Agent) SetDispatcher(d *TaskDispatcher)                 { a.dispatcher = d }
func (a *Agent) Dispatcher() *TaskDispatcher                     { return a.dispatcher }
func (a *Agent) SetCheckpointStore(cs *CheckpointStore) { a.cpStore = cs }
func (a *Agent) SetStore(s *session.Store)                 { a.store = s }
func (a *Agent) SetSession(sess *session.Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.session = sess
	if a.readCache != nil {
		a.readCache.Reset()
	} else {
		a.readCache = newSessionReadCache()
	}
}
func (a *Agent) SetSubAgent(v bool)                     { a.subAgent = v }
func (a *Agent) SetActiveSubtaskID(id string)           { a.activeSubtaskID = id }
func (a *Agent) ActiveSubtaskID() string                { return a.activeSubtaskID }
func (a *Agent) SetOutputDir(dir string)       { a.outputDir = dir }
func (a *Agent) OutputDir() string             { return a.outputDir }
func (a *Agent) SetProjectLayout(l ProjectLayout) { a.projectLayout = l }
func (a *Agent) ProjectLayout() ProjectLayout  { return a.projectLayout }
func (a *Agent) SetRetryConfig(cfg stream.RetryConfig) { a.retryConfig = cfg }
func (a *Agent) RetryConfig() stream.RetryConfig       { return a.retryConfig }
func (a *Agent) SetMaxConsecutiveDenials(n int)        { a.maxConsecutiveDenials = n }
func (a *Agent) SetPipelineOptions(o PipelineOptions)  { a.pipelineOpts = o }
func (a *Agent) SetContextSlimOptions(o ContextSlimOptions) { a.contextSlim = o }
func (a *Agent) SetSkillCatalog(c *skills.Catalog)         { a.skillCatalog = c }
func (a *Agent) SkillCatalog() *skills.Catalog             { return a.skillCatalog }
func (a *Agent) SetMCPManager(m *mcp.Manager)              { a.mcpManager = m }
func (a *Agent) MCPManager() *mcp.Manager                  { return a.mcpManager }

func (a *Agent) SetRunLog(l *runlog.Logger) { a.runLog = l }

func (a *Agent) RunLogPath() string {
	if a.runLog == nil {
		return ""
	}
	return a.runLog.Path()
}

func (a *Agent) logRun(category, format string, args ...interface{}) {
	if a.runLog != nil {
		a.runLog.Line(category, format, args...)
	}
}

func (a *Agent) flushTurnLog(startIdx int) {
	if a.turnLogStore == nil || a.subAgent {
		return
	}
	entries := a.session.EntriesFrom(startIdx)
	if len(entries) == 0 {
		return
	}
	turnID := fmt.Sprintf("%s-%d", a.session.ID, startIdx)
	if err := a.turnLogStore.AppendTurnLog(turnID, entries); err != nil {
		log.Printf("turn log append failed: %v", err)
	}
}

func (a *Agent) State() State                       { return a.state }
func (a *Agent) Cancel()                            { a.state = StateIdle }
func (a *Agent) Toolbox() *tools.Registry           { return a.toolbox }
func (a *Agent) Permissions() *permissions.Checker  { return a.permChecker }
func (a *Agent) Session() *session.Session          { return a.session }

// ContextUsagePct returns estimated context window usage (0–100), aligned with compaction logic.
func (a *Agent) ContextUsagePct() float64 {
	if a.ctxMgr == nil || a.session == nil {
		return 0
	}
	return a.ctxMgr.UsagePercent(a.session.EntriesCopy())
}

func (a *Agent) setState(s State) {
	a.mu.Lock()
	a.state = s
	a.mu.Unlock()
}

func (a *Agent) buildMessages() []llm.Message {
	entries := a.session.EntriesCopy()
	minKeep := a.contextSlim.MinKeepEntries
	if minKeep <= 0 {
		minKeep = defaultContextMinKeep
	}
	if a.ctxMgr != nil && len(entries) > minKeep && a.ctxMgr.ShouldSummarize(entries) {
		compacted, err := a.ctxMgr.Compact(entries, minKeep)
		if err == nil && len(compacted) > 0 && len(compacted) < len(entries) {
			entries = compacted
			a.session.ReplaceEntries(compacted)
		}
	}

	sys := "You are a helpful coding assistant. You have access to tools. Use them when needed. Always respond in English."
	sys += codeExplorationGuidance
	sys += "\n\nVerification: When checking your work, use `go build ./...` first. " +
		"Run `go test ./...` only when the user asked for tests or when fixing existing test failures. " +
		"Never run long-lived processes (`go run`, dev servers, background servers) — they block the agent and are rejected. " +
		"If build or tests fail, analyze errors, fix code, and install dependencies with shell_exec (go mod tidy, npm install, etc.; may require approval). " +
		"A final verify step runs automatically until build (and tests when requested) pass."
	if a.outputDir != "" {
		sys += fmt.Sprintf("\n\nIMPORTANT: All generated code MUST be written under the directory: %s\nUse paths relative to that directory (e.g. main.go). NEVER create new project files in the repository root, internal/, cmd/, or tests/.", displayOutputDir(a.outputDir))
		if a.projectLayout != "" {
			sys += "\n\n" + compactLayoutGuidance(a.projectLayout)
		}
	}
	if a.skillCatalog != nil && a.skillCatalog.Count() > 0 {
		sys += fmt.Sprintf("\n\nSkills: %d SKILL.md loaded — call list_skills then load_skill before specialized workflows.", a.skillCatalog.Count())
	}
	if a.mcpManager != nil && a.mcpManager.ToolCount() > 0 {
		sys += fmt.Sprintf("\n\nMCP: %d external tool(s) connected (names prefixed mcp_).", a.mcpManager.ToolCount())
	}
	if addendum := reviewSystemAddendum(latestUserInstruction(a.session)); addendum != "" {
		sys += addendum
	}
	if addendum := exploredFilesAddendum(entries); addendum != "" {
		sys += addendum
	}
	msgs := []llm.Message{
		llm.NewMessage("system", sys),
	}

	keepFull := a.contextSlim.ToolResultsKeepFull
	if keepFull <= 0 {
		keepFull = defaultToolResultsKeepFull
	}
	toolRecency := toolEntryRecency(entries)

	pendingToolCallIDs := 0
	for i, e := range entries {
		switch e.Role {
		case "tool":
			if pendingToolCallIDs == 0 {
				content := e.Content
				if content == "" && len(e.ToolCalls) > 0 {
					content = e.ToolCalls[0].Result
				}
				msgs = append(msgs, llm.NewMessage("system", "[tool result] "+content))
				continue
			}
			if len(e.ToolCalls) > 0 {
				for _, tc := range e.ToolCalls {
					tcID := tc.ID
					if tcID == "" {
						tcID = tc.Name
					}
					content := tc.Result
					if tc.Error != "" {
						content = tc.Error
					}
					if r, ok := toolRecency[i]; ok && r >= keepFull {
						content = SlimToolResultForHistory(tc.Name, content)
					}
					msgs = append(msgs, llm.NewToolMessage(tcID, content))
					if pendingToolCallIDs > 0 {
						pendingToolCallIDs--
					}
				}
			} else {
				content := e.Content
				if r, ok := toolRecency[i]; ok && r >= keepFull {
					content = SlimToolResultForHistory("tool", content)
				}
				msgs = append(msgs, llm.NewToolMessage("tool", content))
				if pendingToolCallIDs > 0 {
					pendingToolCallIDs--
				}
			}
		case "assistant":
			if len(e.AssistantToolCalls) > 0 {
				msg := llm.Message{Role: "assistant"}
				if e.Content != "" {
					msg.Content = e.Content
				}
				for _, tc := range e.AssistantToolCalls {
					msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: SlimToolArguments(tc.Name, tc.Arguments),
					})
				}
				msgs = append(msgs, msg)
				pendingToolCallIDs = len(e.AssistantToolCalls)
			} else {
				msgs = append(msgs, llm.NewMessage(e.Role, e.Content))
				pendingToolCallIDs = 0
			}
		default:
			content := e.Content
			if e.Role == "system" && strings.Contains(content, "[PROJECT ANALYSIS]") {
				content = CompressGuardAnalysis(content, a.contextSlim.GuardAnalysisMax)
			}
			msgs = append(msgs, llm.NewMessage(e.Role, content))
			pendingToolCallIDs = 0
		}
	}
	return msgs
}

func (a *Agent) buildToolDefs() []llm.Tool {
	defs := make([]llm.Tool, 0, a.toolbox.Count())
	for _, t := range a.toolbox.List() {
		defs = append(defs, llm.Tool{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		})
	}
	return defs
}

// addAssistantWithToolCalls stores an assistant message that requested tool calls.
// The IDs are preserved so buildMessages can reconstruct the OpenAI conversation.
func (a *Agent) addAssistantWithToolCalls(content string, toolCalls []llm.ToolCall) {
	var calls []session.ToolCallData
	for _, tc := range toolCalls {
		calls = append(calls, session.ToolCallData{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: SlimToolArguments(tc.Name, tc.Arguments),
		})
	}
	a.session.Add(session.Entry{
		Role:                "assistant",
		Content:             content,
		State:               StateThinking.String(),
		AssistantToolCalls:  calls,
	})
}
