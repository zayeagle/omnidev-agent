package session

// DefaultTurnLogDir is where external dev-assistant collaboration logs are appended
// (Cursor, Codex, etc. while building omnidev-agent) — YYYYMMDD-session.jsonl / .md.
// omnidev-agent runtime TUI sessions do NOT write here; see DefaultSessionDir.
const DefaultTurnLogDir = ".ai_history/logs"

// DefaultSessionDir holds omnidev-agent runtime session snapshots (per-run JSON/MD).
const DefaultSessionDir = ".ai_history/sessions"
