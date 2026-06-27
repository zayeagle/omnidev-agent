package permissions

import "fmt"

// Level represents the safety classification of an operation.
// v1.2: file read/search are LevelSafe (auto-execute);
//
//	file write/edit/delete, shell exec, and system commands are LevelDangerous (confirm).
type Level int

const (
	LevelSafe      Level = 0
	LevelDangerous Level = 1
)

func (l Level) String() string {
	switch l {
	case LevelSafe:
		return "safe"
	case LevelDangerous:
		return "dangerous"
	default:
		return "unknown"
	}
}

type Checker struct {
	interactive   bool
	denyDangerous bool // headless safe mode: block dangerous ops unless --yolo
}

func NewChecker(interactive bool) *Checker {
	return &Checker{interactive: interactive}
}

// NewForRun selects the permission policy for TUI, headless (-p), or --yolo.
func NewForRun(headless, yolo bool) *Checker {
	if yolo {
		return &Checker{} // auto-approve all dangerous ops
	}
	if headless {
		return &Checker{denyDangerous: true}
	}
	return &Checker{interactive: true}
}

// RequiresApproval reports whether an operation at the given level
// must go through user confirmation. LevelSafe operations skip the prompt;
// LevelDangerous operations require interactive approval.
func (c *Checker) RequiresApproval(level Level) bool {
	return level == LevelDangerous && c.interactive
}

// DenyDangerous reports headless safe mode (dangerous ops blocked without --yolo).
func (c *Checker) DenyDangerous() bool { return c.denyDangerous }

type Approval struct {
	Granted bool   `json:"granted"`
	Reason  string `json:"reason,omitempty"`
}

// Request prompts the user for approval of a dangerous operation.
// For LevelSafe operations this returns Granted:true immediately.
func (c *Checker) Request(level Level, description string) *Approval {
	return &Approval{Granted: !c.RequiresApproval(level)}
}

func (c *Checker) Interactive() bool { return c.interactive }
func (c *Checker) SetInteractive(v bool) { c.interactive = v }

func RequiresApprovalError(level Level, description string) error {
	return fmt.Errorf("permission denied: %s operation '%s' requires user approval", level, description)
}
