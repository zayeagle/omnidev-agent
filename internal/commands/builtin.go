package commands

import (
	"strings"
	"unicode"
)

// HelpText is the built-in /help output (no LLM).
func HelpText() string {
	return strings.TrimSpace(`Built-in commands:
  /help       — show this help
  /clear      — clear transcript (keeps agent context)
  /archive    — archive current session and start fresh
  /sessions   — list archived sessions (first prompt, stats)
  /session <id> — preview a saved session
  /model      — show current model
  /status     — show agent status
  /skills     — list loaded agent skills
  /skill <n>  — load a skill into session context
  /yolo       — toggle permission mode (confirm ↔ yolo)
  Ctrl+Y      — toggle permission mode anytime (even while agent runs)
  [A]         — during a permission prompt: allow all remaining ops
  /checkpoint — show in-progress checkpoint
  /checkpoint rollback <task_id> — rollback and re-run from task
  /copy       — copy full transcript to clipboard (also saves last-screen.txt)
  quit, exit — exit agent
  Ctrl+C      — interrupt session while working; exit when idle

  Keyboard: ↑↓ history · PgUp/PgDn scroll · Home/End jump (empty input) · Tab expand Thinking · Ctrl+Y yolo · Esc interrupt · Y/N/A confirm`)
}

// Normalize trims input and maps fullwidth slash to ASCII.
func Normalize(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if strings.HasPrefix(input, "／") {
		input = "/" + strings.TrimPrefix(input, "／")
	}
	return input
}

// Parse classifies built-in session commands. cmd is lowercase; args holds the remainder.
func Parse(input string) (cmd, args string, ok bool) {
	input = Normalize(input)
	if input == "" {
		return "", "", false
	}

	body := input
	if strings.HasPrefix(body, "/") {
		body = strings.TrimSpace(strings.TrimPrefix(body, "/"))
	} else if isBareAlias(body) {
		return bareAliasCmd(body), "", true
	} else {
		return "", "", false
	}

	if body == "" {
		return "", "", false
	}

	lower := strings.ToLower(body)
	switch {
	case lower == "help", lower == "status", lower == "model", lower == "yolo",
		lower == "skills", lower == "checkpoint", lower == "clear",
		lower == "sessions", lower == "archive", lower == "copy":
		return lower, "", true
	case strings.HasPrefix(lower, "skill"):
		return "skill", argAfterPrefix(body, "skill"), true
	case strings.HasPrefix(lower, "session"):
		return "session", argAfterPrefix(body, "session"), true
	case strings.HasPrefix(lower, "checkpoint"):
		return "checkpoint", argAfterPrefix(body, "checkpoint"), true
	}
	return "", "", false
}

// IsBuiltin reports whether input should be handled locally (never sent to the LLM pipeline).
func IsBuiltin(input string) bool {
	_, _, ok := Parse(input)
	return ok
}

func isBareAlias(input string) bool {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "help", "?", "h":
		return true
	default:
		return false
	}
}

func bareAliasCmd(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "help", "?", "h":
		return "help"
	default:
		return ""
	}
}

func argAfterPrefix(body, prefix string) string {
	body = strings.TrimSpace(body)
	if len(body) <= len(prefix) {
		return ""
	}
	rest := strings.TrimSpace(body[len(prefix):])
	rest = strings.TrimLeftFunc(rest, func(r rune) bool {
		return unicode.IsSpace(r) || r == ':'
	})
	return rest
}
