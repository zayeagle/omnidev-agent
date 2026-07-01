package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/zayeagle/omnidev-agent/internal/commands"
	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/permissions"
	"github.com/zayeagle/omnidev-agent/internal/session"
	"github.com/zayeagle/omnidev-agent/internal/stream"
	"github.com/zayeagle/omnidev-agent/internal/tools"
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

	if handled, err := a.tryBuiltinCommand(ctx, instruction, msgCh); handled || err != nil {
		return err
	}

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
				a.setupProjectWorkspace(ctx, instruction, msgCh,
					"Standalone project workspace: %s (all new code goes here)")
			} else if a.shouldReuseProjectWorkspace(instruction) {
				msgCh <- StreamChunkMsg{
					Content: fmt.Sprintf("Reusing project workspace: %s", a.outputDir),
					Done:    true,
				}
			} else {
				// Legacy project: understand before touching anything
				msgCh <- StreamChunkMsg{
					Content: "Legacy project detected. Learning project structure before making changes...",
					Done:    true,
				}
				a.guard.SetMsgCh(msgCh)
				a.setState(StateExecuting)
				msgCh <- AgentStateMsg{State: StateExecuting}
				emitActivity(msgCh, "Working · scanning project…")
				a.guard.RunScan(ctx)
			}

		case ProjectGreenfield:
			msgCh <- StreamChunkMsg{
				Content: "New project detected. Creating workspace...",
				Done:    true,
			}
			a.setupProjectWorkspace(ctx, instruction, msgCh, "Project workspace ready: %s")
		}
	}

	// 4. Acceptance criteria (structured checklist for completion gate)
	plan := a.ensureAcceptancePlan(ctx, instruction)
	a.session.AddWithState("system", formatAcceptancePlanSystem(plan), StateThinking.String(), 0)

	// 5. Requirements analysis (optional LLM — off by default to save tokens)
	if a.pipelineOpts.UseLLMRequirements {
		if analysis := a.analyzeRequirements(ctx, instruction); analysis != "" {
			msgCh <- StreamChunkMsg{Content: analysis, Done: true}
			a.session.AddWithState("system", analysis, StateThinking.String(), 0)
		}
	}

	// 6. LLM Decomposition + parallel dispatch (all code_mod paths)
	if a.dispatcher != nil {
		outcome, err := a.dispatcher.Dispatch(ctx, instruction, msgCh)
		if err != nil {
			msgCh <- StreamChunkMsg{
				Content: fmt.Sprintf("Task planning failed (%v), falling back to sequential execution.", err),
				Done:    true,
			}
		}
		switch outcome {
		case OutcomeSuccess:
			if a.store != nil {
				a.store.SaveActive(a.session)
			}
			return nil
		case OutcomeFailed:
			if a.store != nil {
				a.store.SaveActive(a.session)
			}
			return nil
		case OutcomeCancelled:
			if a.store != nil {
				a.store.SaveActive(a.session)
			}
			return nil
		}
	}

	// 7. Fallback: main agent loop
	if err := a.standardLoop(ctx, msgCh, true); err != nil {
		return err
	}
	return nil
}

// classifyIntent uses strict chat heuristics first; optional LLM when configured.
func (a *Agent) classifyIntent(ctx context.Context, instruction string, msgCh chan<- tea.Msg) IntentClass {
	if commands.IsBuiltin(instruction) {
		return IntentChat
	}
	// Follow-ups after code work always enter the task pipeline (unless pure greeting).
	if a.hasPriorCodeActivity() && !isPureGreeting(instruction) {
		return IntentCodeMod
	}
	if looksLikeCodeMod(instruction) {
		return IntentCodeMod
	}
	if looksLikeSimpleChat(instruction) {
		return IntentChat
	}
	if a.classifier != nil && a.pipelineOpts.UseLLMClassifier {
		return a.classifier.Classify(ctx, instruction)
	}
	return IntentCodeMod
}

// assessProjectLayout decides minimal vs DDD structure for a new workspace.
func (a *Agent) assessProjectLayout(ctx context.Context, instruction string) ProjectLayout {
	if a.complexityClassifier != nil && a.pipelineOpts.UseLLMComplexity {
		return a.complexityClassifier.Classify(ctx, instruction)
	}
	return layoutFromHeuristic(instruction)
}

// setupProjectWorkspace creates or reuses deliverables/<name>/ from user instruction(s).
func (a *Agent) setupProjectWorkspace(ctx context.Context, instruction string, msgCh chan<- tea.Msg, readyFmt string) {
	if a.shouldReuseProjectWorkspace(instruction) {
		msgCh <- StreamChunkMsg{
			Content: fmt.Sprintf("Reusing project workspace: %s", a.outputDir),
			Done:    true,
		}
		return
	}
	cwd, _ := os.Getwd()
	texts := append([]string{instruction}, a.userInstructionsForNaming()...)
	projDir, err := EnsureProjectWorkspace(cwd, texts...)
	if err != nil {
		msgCh <- StreamChunkMsg{
			Content: fmt.Sprintf("Workspace warning: %v — continuing.", err),
			Done:    true,
		}
		if a.guard != nil && a.guard.ProjectType() == ProjectLegacy {
			a.guard.SetMsgCh(msgCh)
			a.setState(StateExecuting)
			msgCh <- AgentStateMsg{State: StateExecuting}
			emitActivity(msgCh, "Working · scanning project…")
			a.guard.RunScan(ctx)
		}
		return
	}
	a.finalizeNewProjectWorkspace(ctx, projDir, instruction, msgCh, fmt.Sprintf(readyFmt, projDir))
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

func formatAcceptancePlanSystem(plan AcceptancePlan) string {
	var b strings.Builder
	b.WriteString("[ACCEPTANCE CRITERIA]\n")
	for i, c := range plan.Criteria {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, c))
	}
	return strings.TrimRight(b.String(), "\n")
}

// standardLoop is the sequential LLM reasoning loop — used by sub-agents
// and as a fallback when decomposition fails.
// includeTools=false for conversation-only turns (some gateways reject tool schemas).
func (a *Agent) standardLoop(ctx context.Context, msgCh chan<- tea.Msg, includeTools bool) error {
	if err := a.agentLoop(ctx, msgCh, includeTools); err != nil {
		return err
	}
	if !a.subAgent && a.state != StateError {
		a.finishParentTask(ctx, msgCh, includeTools, 1)
	}
	if a.store != nil {
		if err := a.store.SaveActive(a.session); err != nil {
			msgCh <- ErrorMsg{Error: "session save failed: " + err.Error()}
			return err
		}
	}
	return nil
}

func (a *Agent) agentLoop(ctx context.Context, msgCh chan<- tea.Msg, includeTools bool) error {
	consecutiveRejects := 0
	reviewNudges := 0
	exitGateNudges := a.restoreAcceptanceFromCheckpoint()
	instruction := latestUserInstruction(a.session)
	exitedClean := false

	for turn := 0; a.turnsUnlimited() || turn < a.maxTurns; turn++ {
		select {
		case <-ctx.Done():
			a.SaveInterruptCheckpoint(turn)
			a.setState(StateError)
			a.session.AddWithState("system", "agent interrupted", StateError.String(), 0)
			msgCh <- ErrorMsg{Error: "interrupted"}
			return ctx.Err()
		default:
		}

		if includeTools {
			a.setState(StateExecuting)
			msgCh <- AgentStateMsg{State: StateExecuting}
		} else {
			a.setState(StateThinking)
			msgCh <- AgentStateMsg{State: StateThinking}
		}
		messages := a.buildMessages()
		a.logRun("llm", "turn %s request msgs=%d", formatTurnCounter(turn+1, a.maxTurns), len(messages))
		turnStart := time.Now()
		req := &llm.Request{Messages: messages}
		if includeTools {
			req = llm.AdaptToolsForGateway(req, a.buildToolDefs(), llm.ProviderGatewayMode(a.provider))
		}

		resp, err := stream.ChatWithRetry(ctx, a.provider, req, func(part string) {
			msgCh <- StreamChunkMsg{Content: part, Done: false}
		}, a.llmRetryConfig(msgCh))
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
		a.logRun("llm", "turn %s response duration=%s content_chars=%d tool_calls=%d",
			formatTurnCounter(turn+1, a.maxTurns), time.Since(turnStart).Round(time.Millisecond), len(assistantContent), len(toolCalls))

		if len(toolCalls) > 0 {
			ensureToolCallIDs(toolCalls, turn)
			a.addAssistantWithToolCalls(assistantContent, toolCalls)
		} else if assistantContent != "" {
			a.session.AddWithState("assistant", assistantContent, StateThinking.String(), 0)
		}

		// No tool calls → enter exit gate for code tasks, or finish for chat/review
		if len(toolCalls) == 0 {
			if reviewNudges < 2 && needsMoreReview(instruction, a.session) {
				reviewNudges++
				a.session.AddWithState("system", reviewNudgeText, StateThinking.String(), 0)
				msgCh <- StreamChunkMsg{Content: "Review incomplete — exploring more of the codebase…", Done: true}
				continue
			}
			if includeTools && a.acceptanceStrict {
				passed, cont := a.runExitGate(ctx, msgCh, instruction, nil, &exitGateNudges)
				if cont {
					continue
				}
				if passed {
					exitedClean = true
					break
				}
				continue
			}
			exitedClean = true
			break
		}

		// Handle tool calls
		a.setState(StateExecuting)
		msgCh <- AgentStateMsg{State: StateExecuting}
		if len(toolCalls) == 1 {
			emitActivity(msgCh, "Working · "+toolCalls[0].Name+"…")
		} else {
			emitActivity(msgCh, fmt.Sprintf("Working · running %d tools…", len(toolCalls)))
		}

		allApproved := true
		for _, tc := range toolCalls {
			msgCh <- ToolCallMsg{
				Name:      tc.Name,
				Args:      tc.Arguments,
				Status:    "executing",
				SubtaskID: a.ActiveSubtaskID(),
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

			if blockMsg, ok := a.validateTestFileWrite(tc.Name, tc.Arguments); !ok {
				a.logRun("tool_guard", "blocked write_file _test.go %s", SummarizeToolArgsForLog(tc.Name, tc.Arguments))
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
					Preview:     buildConfirmPreview(tc.Name, tc.Arguments),
					Reply:       reply,
				}

				select {
				case userResp := <-reply:
					if userResp.AllowAll {
						a.permChecker.SetInteractive(false)
					}
					if !userResp.Granted {
						rejection := "user denied " + tc.Name
						if userResp.Reason != "" {
							rejection += ": " + userResp.Reason
						}
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
								Error:     rejection,
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

			// Execute the tool (dedupe identical read_file calls within the session)
			var result *tools.Result
			if tc.Name == "read_file" {
				if cached, ok, prefix := a.readCache.Get(tc.Arguments); ok {
					result = tools.OkResult(prefix + cached)
					kind := "CACHED"
					if strings.Contains(prefix, "THROTTLED") {
						kind = "THROTTLED"
					}
					a.logRun("read_cache", "%s path=%v", kind, tc.Arguments["path"])
				} else {
					result = tool.Execute(ctx, tc.Arguments)
					if result.Success {
						a.readCache.Put(tc.Arguments, result.Data)
					}
				}
			} else {
				result = tool.Execute(ctx, tc.Arguments)
			}
			if result.Success && (tc.Name == "write_file" || tc.Name == "edit_file" || tc.Name == "delete_file") {
				if p := strings.TrimSpace(fmt.Sprint(tc.Arguments["path"])); p != "" {
					a.readCache.InvalidatePath(p)
					a.logRun("read_cache", "INVALIDATE path=%s via=%s", p, tc.Name)
				}
			}
			if !result.Success && tc.Name == "edit_file" {
				if hint := editFileFailureHint(tc.Arguments); hint != "" {
					result.Error = strings.TrimSpace(result.Error) + "\n" + hint
				}
			}
			if result.Success {
				a.logRun("tool", "%s ok %s", tc.Name, SummarizeToolArgsForLog(tc.Name, tc.Arguments))
			} else {
				a.logRun("tool", "%s FAIL %s err=%s", tc.Name, SummarizeToolArgsForLog(tc.Name, tc.Arguments), result.Error)
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
				Content:   toolSummaryLine(tc.Name, result.Success),
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
		if a.maxConsecutiveDenials > 0 && consecutiveRejects >= a.maxConsecutiveDenials {
			errMsg := fmt.Sprintf("Aborting: %d consecutive turns with denied operations. Please review your request.", consecutiveRejects)
			a.setState(StateError)
			a.session.AddWithState("system", errMsg, StateError.String(), 0)
			msgCh <- ErrorMsg{Error: errMsg}
			msgCh <- AgentStateMsg{State: StateError}
			break
		}
	}

	if !exitedClean && a.state != StateError && !a.turnsUnlimited() {
		a.emitLoopExhaustedSummary(ctx, msgCh)
	}

	return nil
}

func (a *Agent) emitLoopExhaustedSummary(ctx context.Context, msgCh chan<- tea.Msg) {
	reason := a.loopExhaustedReason()
	summary := a.BuildSessionSummary(ctx, SessionOutcomePartial, reason, nil, nil)
	if summary == "" {
		summary = fmt.Sprintf("Stopped: reached %s.", reason)
	}
	a.session.AddWithState("system", "[ITERATION LIMIT]\n"+summary, StateExecuting.String(), 0)
	msgCh <- StreamChunkMsg{Content: "Iteration limit — summary:\n\n" + summary, Done: true}
	a.logRun("llm", "iteration limit: %s", reason)
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

// buildConfirmPreview returns a short diff/snippet for the permission dialog.
func buildConfirmPreview(name string, args map[string]interface{}) string {
	const maxLines = 8
	const maxLineLen = 72

	truncLine := func(s string) string {
		s = strings.TrimSpace(s)
		if len(s) > maxLineLen {
			return s[:maxLineLen] + "…"
		}
		return s
	}

	switch name {
	case "write_file":
		path, _ := args["path"].(string)
		content, _ := args["content"].(string)
		if path == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString("write " + path + "\n")
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if i >= maxLines {
				b.WriteString(fmt.Sprintf("… +%d lines\n", len(lines)-maxLines))
				break
			}
			b.WriteString("+ " + truncLine(line) + "\n")
		}
		return strings.TrimRight(b.String(), "\n")
	case "edit_file":
		path, _ := args["path"].(string)
		oldS, _ := args["old_snippet"].(string)
		newS, _ := args["new_snippet"].(string)
		if path == "" {
			return ""
		}
		var b strings.Builder
		b.WriteString("edit " + path + "\n")
		for i, line := range strings.Split(oldS, "\n") {
			if i >= maxLines {
				b.WriteString("…\n")
				break
			}
			b.WriteString("- " + truncLine(line) + "\n")
		}
		for i, line := range strings.Split(newS, "\n") {
			if i >= maxLines {
				b.WriteString("…\n")
				break
			}
			b.WriteString("+ " + truncLine(line) + "\n")
		}
		return strings.TrimRight(b.String(), "\n")
	case "delete_file":
		if path, ok := args["path"].(string); ok && path != "" {
			return "delete " + path
		}
	}
	return ""
}

// finishParentTask runs verify-fix until pass, then signals completion to the TUI.
func (a *Agent) finishParentTask(ctx context.Context, msgCh chan<- tea.Msg, includeTools bool, taskCount int) {
	if a.state == StateError {
		a.signalTaskFailed(ctx, msgCh, "agent stopped before completion", nil, nil)
		return
	}

	projectDir := a.resolveVerifyDir()

	verifyOK := true
	if includeTools && projectDir != "" {
		verifyOK = a.runVerifyFixUntilPass(ctx, msgCh, projectDir)
	}

	if !verifyOK {
		a.signalTaskFailed(ctx, msgCh, "build/test verification did not pass after retries", nil, nil)
		return
	}

	instruction := latestUserInstruction(a.session)
	statuses, accepted := a.driveUntilAccepted(ctx, msgCh, instruction, includeTools, nil)
	if includeTools && a.acceptanceStrict && !accepted {
		a.signalTaskFailed(ctx, msgCh, "could not complete task after autonomous recovery", statuses, nil)
		return
	}

	a.signalTaskComplete(ctx, msgCh, includeTools, taskCount, projectDir, nil, statuses)
}

func (a *Agent) signalTaskComplete(ctx context.Context, msgCh chan<- tea.Msg, includeTools bool, taskCount int, projectDir string, results []TaskResult, criteria []CriterionStatus) {
	if a.acceptanceStrict && len(criteria) > 0 && !allCriteriaMet(criteria) {
		a.signalTaskFailed(ctx, msgCh, "acceptance criteria not fully met", criteria, results)
		return
	}
	conclusion := a.BuildFinalConclusion(ctx, results, criteria)
	passedN, totalN := countCriteriaMet(criteria), len(criteria)
	allPassed := len(criteria) == 0 || allCriteriaMet(criteria)
	if projectDir != "" {
		if len(criteria) > 0 && a.acceptanceStrict {
			mech := a.pendingMech
			if !mech.allOK() && mech.Summary == "" {
				mech = mechanicalVerifyResult{WorkspaceOK: allCriteriaMet(criteria), CustomOK: true}
			}
			msgCh <- NewAllCompleteWithAcceptance(conclusion, projectDir, formatAcceptanceReport(criteria, mech), allPassed, passedN, totalN)
		} else {
			msgCh <- NewAllComplete(conclusion, projectDir)
		}
	} else if includeTools {
		msgCh <- AllCompleteMsg{Summary: conclusion}
	}
	a.setState(StateDone)
	msgCh <- AgentStateMsg{State: StateDone}
	msgCh <- DoneMsg{}
}

func (a *Agent) signalTaskFailed(ctx context.Context, msgCh chan<- tea.Msg, reason string, criteria []CriterionStatus, results []TaskResult) {
	projectDir := a.resolveVerifyDir()
	if len(criteria) > 0 {
		report := formatAcceptanceReport(criteria, a.pendingMech)
		msgCh <- StreamChunkMsg{Content: report, Done: true}
		a.logRun("acceptance", "task failed: %s\n%s", reason, report)
	} else {
		a.logRun("acceptance", "task failed: %s", reason)
	}
	conclusion := a.BuildSessionSummary(ctx, SessionOutcomeFailed, reason, results, criteria)
	if conclusion == "" {
		conclusion = reason
		if len(criteria) > 0 {
			conclusion += " — " + formatVerificationSummary(criteria)
		}
	}
	resumable := a.acceptanceStrict && len(criteria) > 0 && !allCriteriaMet(criteria)
	if resumable {
		plan := a.ensureAcceptancePlan(ctx, latestUserInstruction(a.session))
		a.persistAcceptanceCheckpoint(plan, criteria, 0)
	}
	detail := ""
	passedN, totalN := 0, len(criteria)
	if len(criteria) > 0 {
		detail = formatAcceptanceReport(criteria, a.pendingMech)
		for _, c := range criteria {
			if c.Met {
				passedN++
			}
		}
	}
	msgCh <- PartialCompleteMsg{
		Summary:    conclusion,
		ProjectDir: projectDir,
		Criteria:   criteria,
		Reason:     reason,
		Resumable:  resumable,
		AcceptanceDetail: detail,
		AcceptancePassed: passedN == totalN && totalN > 0,
		AcceptancePassedN: passedN,
		AcceptanceTotalN:  totalN,
	}
	msgCh <- StreamChunkMsg{Content: conclusion, Done: true}
	msgCh <- ErrorMsg{Error: reason}
	a.setState(StateError)
	msgCh <- AgentStateMsg{State: StateError}
}

func formatVerificationSummary(statuses []CriterionStatus) string {
	passed := 0
	for _, s := range statuses {
		if s.Met {
			passed++
		}
	}
	return fmt.Sprintf("%d/%d acceptance criteria met", passed, len(statuses))
}

func conclusionWithCriteria(conclusion string, criteria []CriterionStatus) string {
	if len(criteria) == 0 {
		return conclusion
	}
	passed := 0
	for _, c := range criteria {
		if c.Met {
			passed++
		}
	}
	note := fmt.Sprintf("Acceptance: %d/%d criteria verified.", passed, len(criteria))
	if strings.TrimSpace(conclusion) == "" {
		return note
	}
	return conclusion + "\n\n" + note
}
