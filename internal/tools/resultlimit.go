package tools

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ResultLimits controls inline tool output budgets (Claude Code / Cursor-style).
type ResultLimits struct {
	MaxChars            int    // inline char budget; default 8000
	SpoolDir            string // full payloads when not already a file; default .ai_history/tool_spool/
	SearchMaxLines      int    // max match lines for search_code; default 100
	ListDirMaxEntries   int    // max directory entries; default 200
	ReadFileDefaultLimit int   // default lines when read_file limit omitted; default 300
}

var globalLimits = DefaultResultLimits()

func DefaultResultLimits() ResultLimits {
	return ResultLimits{
		MaxChars:             8000,
		SpoolDir:             ".ai_history/tool_spool/",
		SearchMaxLines:       100,
		ListDirMaxEntries:    200,
		ReadFileDefaultLimit: 300,
	}
}

// SetResultLimits configures tool output budgets (call from main after config load).
func SetResultLimits(l ResultLimits) {
	if l.MaxChars > 0 {
		globalLimits.MaxChars = l.MaxChars
	}
	if l.SpoolDir != "" {
		globalLimits.SpoolDir = l.SpoolDir
	}
	if l.SearchMaxLines > 0 {
		globalLimits.SearchMaxLines = l.SearchMaxLines
	}
	if l.ListDirMaxEntries > 0 {
		globalLimits.ListDirMaxEntries = l.ListDirMaxEntries
	}
	if l.ReadFileDefaultLimit > 0 {
		globalLimits.ReadFileDefaultLimit = l.ReadFileDefaultLimit
	}
}

func effectiveLimits() ResultLimits {
	return globalLimits
}

// DeliverOpts describes a tool result before inline delivery to the LLM.
type DeliverOpts struct {
	ToolName   string
	Content    string
	SourcePath string // when set, full content stays on disk at this path (no duplicate spool)
	Hint       string // extra continuation guidance
}

// DeliverOutput returns content within budget, or a PARTIAL view with continuation handles.
// Full content is never discarded: large non-file outputs are spooled; file reads reference SourcePath.
func DeliverOutput(opts DeliverOpts) string {
	content := opts.Content
	max := effectiveLimits().MaxChars
	if max <= 0 || len(content) <= max {
		if opts.Hint != "" {
			return content + "\n\n" + opts.Hint
		}
		return content
	}

	original := len(content)
	spoolPath := opts.SourcePath
	if spoolPath == "" {
		var err error
		spoolPath, err = writeSpool(opts.ToolName, content)
		if err != nil {
			spoolPath = "(spool write failed: " + err.Error() + ")"
		}
	}

	hint := opts.Hint
	if hint == "" {
		if opts.SourcePath != "" {
			hint = fmt.Sprintf("Use read_file on %q with offset/limit to read more.", opts.SourcePath)
		} else if spoolPath != "" && !strings.HasPrefix(spoolPath, "(") {
			hint = fmt.Sprintf("Use read_file on %q with offset/limit to read the full output.", spoolPath)
		}
	}

	banner := fmt.Sprintf("[PARTIAL %s: %d chars → %d inline | full: %s]",
		opts.ToolName, original, max, spoolPath)
	if hint != "" {
		banner += "\nContinue: " + hint
	}
	banner += "\n---\n"

	bodyMax := max - len(banner)
	if bodyMax < 512 {
		bodyMax = max / 2
	}
	return banner + headTailTruncate(content, bodyMax)
}

func writeSpool(toolName, content string) (string, error) {
	dir := effectiveLimits().SpoolDir
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	id := time.Now().Format("20060102-150405")
	if b := randomHex(4); b != "" {
		id += "-" + b
	}
	safe := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return '_'
	}, toolName)
	path := filepath.Join(dir, id+"_"+safe+".txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}

// headTailTruncate keeps beginning and end so errors at the tail remain visible (Codex-style).
func headTailTruncate(content string, maxChars int) string {
	if maxChars <= 0 || len(content) <= maxChars {
		return content
	}
	const omitFmt = "\n\n... [omitted %d chars — not lost; see PARTIAL banner above] ...\n\n"
	omitReserve := 80
	if maxChars < omitReserve+200 {
		return content[:maxChars]
	}
	tailBudget := maxChars / 3
	headBudget := maxChars - tailBudget - omitReserve
	if headBudget < 100 {
		headBudget = maxChars / 2
		tailBudget = maxChars - headBudget - omitReserve
	}
	omitted := len(content) - headBudget - tailBudget
	if omitted < 0 {
		omitted = len(content) - headBudget
		tailBudget = 0
	}
	marker := fmt.Sprintf(omitFmt, omitted)
	head := content[:headBudget]
	var tail string
	if tailBudget > 0 && len(content) > tailBudget {
		tail = content[len(content)-tailBudget:]
	}
	return head + marker + tail
}

func okLimited(toolName, content string) *Result {
	return OkResult(DeliverOutput(DeliverOpts{ToolName: toolName, Content: content}))
}

func okLimitedFile(toolName, content, sourcePath string, offset, limit, totalLines int) *Result {
	hint := ""
	if totalLines > 0 {
		next := offset + limit
		if limit > 0 && next <= totalLines {
			hint = fmt.Sprintf("Lines %d–%d of %d shown. read_file path=%q offset=%d limit=%d for next page.",
				offset, offset+limit-1, totalLines, sourcePath, next, limit)
		} else if totalLines > 0 {
			hint = fmt.Sprintf("File has %d lines total. read_file path=%q offset=N limit=M to paginate.", totalLines, sourcePath)
		}
	}
	return OkResult(DeliverOutput(DeliverOpts{
		ToolName:   toolName,
		Content:    content,
		SourcePath: sourcePath,
		Hint:       hint,
	}))
}

// readFileSlice reads line offset (1-based) and limit (0 = rest). Returns text, total lines, ok.
func readFileSlice(data []byte, offset, limit int) (string, int) {
	text := string(data)
	if text == "" {
		return "", 0
	}
	lines := strings.Split(text, "\n")
	total := len(lines)
	if offset < 1 {
		offset = 1
	}
	if offset > total {
		return "", total
	}
	start := offset - 1
	end := total
	if limit > 0 && start+limit < end {
		end = start + limit
	}
	return strings.Join(lines[start:end], "\n"), total
}
