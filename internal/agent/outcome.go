package agent

// DispatchOutcome reports how the dispatcher finished a code-modification request.
type DispatchOutcome int

const (
	OutcomeNotHandled DispatchOutcome = iota // fall back to standard loop
	OutcomeSuccess
	OutcomeFailed
	OutcomeCancelled
)

func (o DispatchOutcome) Handled() bool {
	return o != OutcomeNotHandled
}
