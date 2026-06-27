package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

// RunLoop starts the full agent reasoning loop on its own goroutine,
// communicating with the TUI through msgCh.
//
// Pipeline (parent agent):
//   1. LLM Classification: chat → direct conversation; code_modification → continue
//   2. Project Assessment: legacy → guard scan or new workspace; greenfield → workspace + layout
//   3. LLM Task Decomposition (always for code modification)
//   4. Dispatcher with checkpoint-aware parallel execution
//   5. Standard loop fallback (if decomposition fails)
//
// Sub-agents skip steps 1-4 and go directly to the standard loop.
func (a *Agent) RunLoop(ctx context.Context, instruction string, msgCh chan<- tea.Msg) error {
	a.setState(StateThinking)
	msgCh <- AgentStateMsg{State: StateThinking}

	defer func() {
		a.mu.Lock()
		if a.state != StateError {
			a.state = StateIdle
		}
		a.mu.Unlock()
	}()

	// 1. Add user message to session
	a.session.AddWithState("user", instruction, StateThinking.String(), 0)

	// Sub-agents: skip pipeline, go straight to standard loop
	if a.subAgent {
		return a.standardLoop(ctx, msgCh, true)
	}

	// ── PIPELINE (parent agent only) ──

	// 2. LLM Classification: is this simple chat or code modification?
	intent := a.classifyIntent(ctx, instruction, msgCh)

	if intent == IntentChat {
		msgCh <- StreamChunkMsg{
			Content: "Conversation mode — responding directly.",
			Done:    true,
		}
		return a.standardLoop(ctx, msgCh, false)
	}

	// 3. Project Assessment (code modification confirmed)
	msgCh <- StreamChunkMsg{
		Content: "Code modification mode — assessing project...",
		Done:    true,
	}

	if a.guard != nil {
		pt := a.guard.ProjectType()
		switch pt {
		case ProjectLegacy:
			if IsNewProjectRequest(instruction) {
				cwd, _ := os.Getwd()
				projDir, err := EnsureProjectWorkspace(cwd, instruction)
				if err != nil {
					msgCh <- StreamChunkMsg{
						Content: fmt.Sprintf("Workspace warning: %v — continuing with legacy scan.", err),
						Done:    true,
					}
					a.guard.SetMsgCh(msgCh)
					a.guard.RunScan(ctx)
				} else {
					a.finalizeNewProjectWorkspace(ctx, projDir, instruction, msgCh,
						fmt.Sprintf("Standalone project workspace: %s (all new code goes here)", projDir))
				}
			} else {
				// Legacy project: understand before touching anything
				msgCh <- StreamChunkMsg{
					Content: "Legacy project detected. Learning project structure before making changes...",
					Done:    true,
				}
				a.guard.SetMsgCh(msgCh)
				a.guard.RunScan(ctx)
			}

		case ProjectGreenfield:
			msgCh <- StreamChunkMsg{
				Content: "New project detected. Creating workspace...",
				Done:    true,
			}
			cwd, _ := os.Getwd()
			projDir, err := EnsureProjectWorkspace(cwd, instruction)
			if err != nil {
				msgCh <- StreamChunkMsg{
					Content: fmt.Sprintf("Workspace warning: %v — continuing.", err),
					Done:    true,
				}
			} else {
				a.finalizeNewProjectWorkspace(ctx, projDir, instruction, msgCh,
					fmt.Sprintf("Project workspace ready: %s", projDir))
			}
		}
	}

	// 4. LLM Decomposition (always for code modification)
	//    Simple tasks → 1 task; complex tasks → N tasks with DAG
	if a.dispatcher != nil {
		handled, err := a.dispatcher.Dispatch(ctx, instruction, msgCh)
		if err != nil {
			msgCh <- StreamChunkMsg{
				Content: fmt.Sprintf("Decomposition failed (%v), falling back to sequential execution.", err),
				Done:    true,
			}
		}
		if handled {
			a.setState(StateDone)
			msgCh <- AgentStateMsg{State: StateDone}
			msgCh <- DoneMsg{}
			if a.store != nil {
				a.store.Save(a.session)
				a.store.Export(a.session)
			}
			return nil
		}
	}

	// 5. Standard loop fallback
	a.session.AddWithState("system", "Falling back to sequential execution.", StateThinking.String(), 0)
	return a.standardLoop(ctx, msgCh, true)
}

// classifyIntent runs an inexpensive LLM call to determine whether the user
// wants a conversation or code changes. Falls back to IntentCodeMod on error.
func (a *Agent) classifyIntent(ctx context.Context, instruction string, msgCh chan<- tea.Msg) IntentClass {
	if a.classifier != nil {
		return a.classifier.Classify(ctx, instruction)
	}
	// No classifier configured: safe default — run full pipeline
	return IntentCodeMod
}

// assessProjectLayout decides minimal vs DDD structure for a new workspace.
func (a *Agent) assessProjectLayout(ctx context.Context, instruction string) ProjectLayout {
	if a.complexityClassifier != nil {
		return a.complexityClassifier.Classify(ctx, instruction)
	}
	return layoutFromHeuristic(instruction)
}

// finalizeNewProjectWorkspace sets output dir, classifies layout, and optionally scaffolds DDD.
func (a *Agent) finalizeNewProjectWorkspace(ctx context.Context, projDir, instruction string, msgCh chan<- tea.Msg, readyMsg string) {
	a.SetOutputDir(projDir)
	layout := a.assessProjectLayout(ctx, instruction)
	a.SetProjectLayout(layout)

	msgCh <- StreamChunkMsg{Content: readyMsg, Done: true}

	switch layout {
	case LayoutDDD:
		msgCh <- StreamChunkMsg{
			Content: "Architecture: DDD layout (multi-layer HTTP / full-stack).",
			Done:    true,
		}
		scaffolder := NewScaffolder(projDir)
		if _, err := scaffolder.InitDDD(ctx, msgCh); err != nil {
			msgCh <- StreamChunkMsg{
				Content: fmt.Sprintf("Scaffold warning: %v — continuing.", err),
				Done:    true,
			}
		}
	default:
		msgCh <- StreamChunkMsg{
			Content: "Architecture: minimal — prefer a single file or very few files; no DDD scaffold.",
			Done:    true,
		}
	}
}

// standardLoop is the sequential LLM reasoning loop — used by sub-agents
// and as a fallback when decomposition fails.
// includeTools=false for conversation-only turns (some gateways reject tool schemas).
func (a *Agent) standardLoop(ctx context.Context, msgCh chan<- tea.Msg, includeTools bool) error {
	consecutiveRejects := 0

	for turn := 0; turn < a.maxTurns; turn++ {
		select {
		case <-ctx.Done():
			a.setState(StateError)
			a.session.AddWithState("system", "agent cancelled", StateError.String(), 0)
			msgCh <- ErrorMsg{Error: "cancelled"}
			return ctx.Err()
		default:
		}

		a.setState(StateThinking)
		msgCh <- AgentStateMsg{State: StateThinking}

		messages := a.buildMessages()
		req := &llm.Request{Messages: messages}
		if includeTools {
			req.Tools = a.buildToolDefs()
		}

		resp, err := stream.ChatWithRetry(ctx, a.provider, req, func(part string) {
			msgCh <- StreamChunkMsg{Content: part, Done: false}
		})
		if err != nil {
			a.setState(StateError)
			errContent := "LLM error: " + err.Error()
			a.session.AddWithState("system", errContent, StateError.String(), 0)
			msgCh <- ErrorMsg{Error: errContent}
			return err
		}
		msgCh <- StreamChunkMsg{Done: true}

		// Determine tool calls: try structured plan JSON first, fall back to native tool_calls.
		// OpenAI requires every tool message to follow an assistant message with
		// matching tool_calls — record that assistant entry before executing tools.
		var toolCalls []llm.ToolCall
		var assistantContent string
		if plan := llm.ParseStructuredPlan(resp.Content); plan != nil {
			toolCalls = llm.ExtractToolCallsFromPlan(plan)
			assistantContent = llm.ExtractReasoningText(plan)
		} else {
			toolCalls = resp.ToolCalls
			assistantContent = resp.Content
		}

		if len(toolCalls) > 0 {
			ensureToolCallIDs(toolCalls, turn)
			a.addAssistantWithToolCalls(assistantContent, toolCalls)
		} else if assistantContent != "" {
			a.session.AddWithState("assistant", assistantContent, StateThinking.String(), 0)
		}

		// No tool calls → agent is finished
		if len(toolCalls) == 0 {
			break
		}

		// Handle tool calls
		a.setState(StateExecuting)
		msgCh <- AgentStateMsg{State: StateExecuting}

		allApproved := true
		for _, tc := range toolCalls {
			msgCh <- ToolCallMsg{
				Name:   tc.Name,
				Args:   tc.Arguments,
				Status: "executing",
			}

			tool, ok := a.toolbox.Get(tc.Name)
			if !ok {
				errMsg := "unknown tool: " + tc.Name
				a.session.Add(session.Entry{
					Timestamp: time.Now(),
					Role:      "tool",
					Content:   errMsg,
					State:     StateExecuting.String(),
					ToolCalls: []session.ToolCallEntry{{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Allowed:   false,
						Error:     errMsg,
					}},
				})
				msgCh <- ToolResultMsg{Success: false, Error: errMsg}
				continue
			}

			// Greenfield: block writes outside deliverables workspace
			if blockMsg, ok := a.validateWorkspacePath(tc.Name, tc.Arguments); !ok {
				a.session.Add(session.Entry{
					Timestamp: time.Now(),
					Role:      "tool",
					Content:   blockMsg,
					State:     StateExecuting.String(),
					ToolCalls: []session.ToolCallEntry{{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Allowed:   false,
						Error:     blockMsg,
					}},
				})
				msgCh <- ToolResultMsg{Success: false, Error: blockMsg}
				allApproved = false
				continue
			}

			// Legacy: block standalone-app pollution at repo root / new packages
			if blockMsg, ok := a.validateLegacyWrite(tc.Name, tc.Arguments); !ok {
				a.session.Add(session.Entry{
					Timestamp: time.Now(),
					Role:      "tool",
					Content:   blockMsg,
					State:     StateExecuting.String(),
					ToolCalls: []session.ToolCallEntry{{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Allowed:   false,
						Error:     blockMsg,
					}},
				})
				msgCh <- ToolResultMsg{Success: false, Error: blockMsg}
				allApproved = false
				continue
			}

			// Guard check: legacy project destructive write without awareness
			if a.guard != nil && IsDestructive(tc.Name, tool.Level()) && !a.guard.IsAwarenessComplete() {
				blockMsg := fmt.Sprintf("BLOCKED: %s requires project understanding first. Skipping.", tc.Name)
				a.session.Add(session.Entry{
					Timestamp: time.Now(),
					Role:      "system",
					Content:   blockMsg,
					State:     StateExecuting.String(),
					ToolCalls: []session.ToolCallEntry{{
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Allowed:   false,
					}},
				})
				msgCh <- ToolResultMsg{Success: false, Error: blockMsg}
				allApproved = false
				continue
			}

			// Headless safe mode: deny dangerous ops unless --yolo
			if tool.Level() == permissions.LevelDangerous && a.permChecker.DenyDangerous() {
				blockMsg := "BLOCKED: dangerous operation denied in headless mode (use --yolo to override)"
				a.session.Add(session.Entry{
					Timestamp: time.Now(),
					Role:      "tool",
					Content:   blockMsg,
					State:     StateExecuting.String(),
					ToolCalls: []session.ToolCallEntry{{
						ID:        tc.ID,
						Name:      tc.Name,
						Arguments: tc.Arguments,
						Allowed:   false,
						Error:     blockMsg,
					}},
				})
				msgCh <- ToolResultMsg{Success: false, Error: blockMsg}
				allApproved = false
				continue
			}

			// Permission check for dangerous operations
			if tool.Level() == permissions.LevelDangerous && a.permChecker.Interactive() {
				a.setState(StateWaitingApproval)
				msgCh <- AgentStateMsg{State: StateWaitingApproval}

				description := buildToolDescription(tc.Name, tc.Arguments)
				reply := make(chan permissions.ConfirmResponse, 1)
				msgCh <- ConfirmRequestMsg{
					Level:       tool.Level(),
					Description: description,
					Reply:       reply,
				}

				select {
				case userResp := <-reply:
					if userResp.AllowAll {
						a.permChecker.SetInteractive(false)
					}
					if !userResp.Granted {
						rejection := "user denied " + tc.Name
						a.session.Add(session.Entry{
							Timestamp: time.Now(),
							Role:      "tool",
							Content:   rejection,
							State:     StateWaitingApproval.String(),
							ToolCalls: []session.ToolCallEntry{{
								ID:        tc.ID,
								Name:      tc.Name,
								Arguments: tc.Arguments,
								Allowed:   false,
							}},
						})
						msgCh <- ToolResultMsg{Success: false, Error: rejection}
						allApproved = false
						continue
					}
				case <-ctx.Done():
					return ctx.Err()
				}

				a.setState(StateExecuting)
				msgCh <- AgentStateMsg{State: StateExecuting}
			}

			// Execute the tool
			result := tool.Execute(ctx, tc.Arguments)
			toolContent := result.Data
			if !result.Success {
				toolContent = result.Error
			}

			tcEntry := session.ToolCallEntry{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
				Allowed:   true,
			}
			if result.Success {
				tcEntry.Result = result.Data
			} else {
				tcEntry.Error = result.Error
			}

			a.session.Add(session.Entry{
				Timestamp: time.Now(),
				Role:      "tool",
				Content:   toolContent,
				State:     StateExecuting.String(),
				ToolCalls: []session.ToolCallEntry{tcEntry},
			})

			if result.Success {
				msgCh <- ToolResultMsg{Success: true, Data: result.Data}
			} else {
				msgCh <- ToolResultMsg{Success: false, Error: result.Error}
			}
		}

		// ── Reject limiter (v2.2): abort after 3 consecutive denied turns ──
		if !allApproved {
			a.session.AddWithState("system", "Some requested tool operations were denied by the user. Please adjust your approach.", StateThinking.String(), 0)
			consecutiveRejects++
		} else {
			consecutiveRejects = 0
		}
		if consecutiveRejects >= 3 {
			errMsg := fmt.Sprintf("Aborting: %d consecutive turns with denied operations. Please review your request.", consecutiveRejects)
			a.session.AddWithState("system", errMsg, StateError.String(), 0)
			msgCh <- ErrorMsg{Error: errMsg}
			break
		}
	}

	// Done — only the parent agent loop signals completion to the TUI.
	if !a.subAgent {
		a.setState(StateDone)
		msgCh <- AgentStateMsg{State: StateDone}
		msgCh <- DoneMsg{}
	}

	if a.store != nil {
		if err := a.store.Save(a.session); err != nil {
			msgCh <- ErrorMsg{Error: "session save failed: " + err.Error()}
			return err
		}
		if err := a.store.Export(a.session); err != nil {
			msgCh <- ErrorMsg{Error: "session export failed: " + err.Error()}
		}
	}
	return nil
}

// ensureToolCallIDs assigns stable IDs to tool calls that lack them.
func ensureToolCallIDs(calls []llm.ToolCall, turn int) {
	for i := range calls {
		if calls[i].ID == "" {
			calls[i].ID = fmt.Sprintf("call-%d-%d", turn, i)
		}
	}
}

// buildToolDescription creates a human-readable summary of a tool call.
func buildToolDescription(name string, args map[string]interface{}) string {
	switch name {
	case "shell_exec":
		if cmd, ok := args["cmd"].(string); ok {
			return fmt.Sprintf("shell_exec: %s", cmd)
		}
	case "delete_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("delete_file: %s", path)
		}
	case "write_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("write_file: %s", path)
		}
	case "edit_file":
		if path, ok := args["path"].(string); ok {
			return fmt.Sprintf("edit_file: %s", path)
		}
	}
	return fmt.Sprintf("%s %v", name, args)
}
