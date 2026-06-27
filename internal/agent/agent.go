package agent

import (
	"fmt"
	"log"
	"sync"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
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
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "Idle"
	case StateThinking:
		return "Thinking"
	case StateExecuting:
		return "Executing"
	case StateWaitingApproval:
		return "WaitingApproval"
	case StateDone:
		return "Done"
	case StateError:
		return "Error"
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
}

func New(provider llm.Provider, permChecker *permissions.Checker, toolbox *tools.Registry, sess *session.Session) *Agent {
	return &Agent{
		state:       StateIdle,
		provider:    provider,
		permChecker: permChecker,
		toolbox:     toolbox,
		session:     sess,
		maxTurns:    20,
	}
}

func (a *Agent) SetMaxTurns(n int)                     { a.maxTurns = n }
func (a *Agent) SetContextManager(cm *ContextManager)   { a.ctxMgr = cm }
func (a *Agent) SetGuard(g *ProjectAwarenessGuard)      { a.guard = g }
func (a *Agent) SetClassifier(c *Classifier)                   { a.classifier = c }
func (a *Agent) SetComplexityClassifier(c *ComplexityClassifier) { a.complexityClassifier = c }
func (a *Agent) SetDispatcher(d *TaskDispatcher)                 { a.dispatcher = d }
func (a *Agent) SetCheckpointStore(cs *CheckpointStore) { a.cpStore = cs }
func (a *Agent) SetStore(s *session.Store)                 { a.store = s }
func (a *Agent) SetSubAgent(v bool)                     { a.subAgent = v }
func (a *Agent) SetActiveSubtaskID(id string)           { a.activeSubtaskID = id }
func (a *Agent) ActiveSubtaskID() string                { return a.activeSubtaskID }
func (a *Agent) SetOutputDir(dir string)       { a.outputDir = dir }
func (a *Agent) OutputDir() string             { return a.outputDir }
func (a *Agent) SetProjectLayout(l ProjectLayout) { a.projectLayout = l }
func (a *Agent) ProjectLayout() ProjectLayout  { return a.projectLayout }

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

func (a *Agent) setState(s State) {
	a.mu.Lock()
	a.state = s
	a.mu.Unlock()
}

func (a *Agent) buildMessages() []llm.Message {
	sys := "You are a helpful coding assistant. You have access to tools. Use them when needed. Always respond in English."
	if a.outputDir != "" {
		sys += fmt.Sprintf("\n\nIMPORTANT: All generated code MUST be written under the directory: %s\nUse paths relative to that directory (e.g. main.go). NEVER create new project files in the repository root, internal/, cmd/, or tests/.", displayOutputDir(a.outputDir))
		if a.projectLayout != "" {
			sys += "\n\n" + layoutGuidance(a.projectLayout)
		}
	}
	msgs := []llm.Message{
		llm.NewMessage("system", sys),
	}

	const minKeep = 10
	entries := a.session.Entries
	if a.ctxMgr != nil && len(entries) > minKeep && a.ctxMgr.ShouldSummarize(entries) {
		compacted, err := a.ctxMgr.Compact(entries, minKeep)
		if err == nil && len(compacted) > 0 {
			entries = compacted
		}
	}

	pendingToolCallIDs := 0
	for _, e := range entries {
		switch e.Role {
		case "tool":
			// Each tool result must follow an assistant message with tool_calls.
			if pendingToolCallIDs == 0 {
				msgs = append(msgs, llm.NewMessage("system", "[tool result] "+e.Content))
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
					msgs = append(msgs, llm.NewToolMessage(tcID, content))
					if pendingToolCallIDs > 0 {
						pendingToolCallIDs--
					}
				}
			} else {
				msgs = append(msgs, llm.NewToolMessage("tool", e.Content))
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
					msg.ToolCalls = append(msg.ToolCalls, llm.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
				}
				msgs = append(msgs, msg)
				pendingToolCallIDs = len(e.AssistantToolCalls)
			} else {
				msgs = append(msgs, llm.NewMessage(e.Role, e.Content))
				pendingToolCallIDs = 0
			}
		default:
			msgs = append(msgs, llm.NewMessage(e.Role, e.Content))
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
		calls = append(calls, session.ToolCallData{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments})
	}
	a.session.Add(session.Entry{
		Role:                "assistant",
		Content:             content,
		State:               StateThinking.String(),
		AssistantToolCalls:  calls,
	})
}
