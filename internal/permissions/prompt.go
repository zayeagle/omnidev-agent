package permissions

import "time"

// PromptChecker wraps a Checker with TUI prompt capability.
// When a dangerous operation requires approval, PromptChecker
// sends a confirm request through a channel and waits for the response.
type PromptChecker struct {
	*Checker
	promptCh  chan<- ConfirmRequest
	responseCh <-chan ConfirmResponse
	timeout   time.Duration
}

// ConfirmRequest is sent to the TUI when an operation needs user approval.
type ConfirmRequest struct {
	Level       Level
	Description string
}

// ConfirmResponse is received from the TUI after user decides.
type ConfirmResponse struct {
	Granted  bool
	Reason   string
	AllowAll bool // when true, auto-approve all remaining dangerous ops this session
}

// NewPromptChecker creates a PromptChecker wired to the given channels.
// promptCh sends requests to TUI; responseCh receives user decisions.
func NewPromptChecker(interactive bool, promptCh chan<- ConfirmRequest, responseCh <-chan ConfirmResponse) *PromptChecker {
	return &PromptChecker{
		Checker:    NewChecker(interactive),
		promptCh:   promptCh,
		responseCh: responseCh,
		timeout:    30 * time.Second,
	}
}

// SetTimeout sets the approval wait timeout.
func (pc *PromptChecker) SetTimeout(d time.Duration) {
	pc.timeout = d
}

// Request sends a confirmation prompt to the TUI and blocks until
// the user responds or the timeout expires (default deny).
func (pc *PromptChecker) Request(level Level, description string) *Approval {
	if !pc.RequiresApproval(level) {
		return &Approval{Granted: true}
	}
	if pc.promptCh == nil {
		return &Approval{Granted: false, Reason: "prompt channel not configured"}
	}

	// Non-blocking send to TUI
	select {
	case pc.promptCh <- ConfirmRequest{Level: level, Description: description}:
	default:
		return &Approval{Granted: false, Reason: "unable to reach TUI prompt"}
	}

	// Wait for user response with timeout
	select {
	case resp := <-pc.responseCh:
		return &Approval{Granted: resp.Granted, Reason: resp.Reason}
	case <-time.After(pc.timeout):
		return &Approval{Granted: false, Reason: "approval timeout — default deny"}
	}
}
