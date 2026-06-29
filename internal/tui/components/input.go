package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

var (
	inputPromptStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))
	inputPlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563")).Italic(true)
	inputCursorStyle      = lipgloss.NewStyle().Background(lipgloss.Color("#374151")).Foreground(lipgloss.Color("#FFFFFF"))
)

type InputLine struct {
	text    []rune
	cursor  int
	history []string
	histIdx int
}

func NewInputLine() *InputLine {
	return &InputLine{
		text:    make([]rune, 0),
		histIdx: -1,
	}
}

func (il *InputLine) SetText(s string) {
	il.text = []rune(s)
	il.cursor = len(il.text)
}

func (il *InputLine) Text() string   { return string(il.text) }
func (il *InputLine) CursorPos() int { return il.cursor }

func (il *InputLine) Insert(r rune) {
	il.text = append(il.text, 0)
	copy(il.text[il.cursor+1:], il.text[il.cursor:])
	il.text[il.cursor] = r
	il.cursor++
}

func (il *InputLine) DeleteBefore() {
	if il.cursor > 0 {
		il.text = append(il.text[:il.cursor-1], il.text[il.cursor:]...)
		il.cursor--
	}
}

func (il *InputLine) DeleteAfter() {
	if il.cursor < len(il.text) {
		il.text = append(il.text[:il.cursor], il.text[il.cursor+1:]...)
	}
}

func (il *InputLine) MoveLeft()  { if il.cursor > 0 { il.cursor-- } }
func (il *InputLine) MoveRight() { if il.cursor < len(il.text) { il.cursor++ } }
func (il *InputLine) MoveHome()  { il.cursor = 0 }
func (il *InputLine) MoveEnd()   { il.cursor = len(il.text) }

func (il *InputLine) Submit() string {
	t := il.Text()
	if t != "" {
		il.history = append(il.history, t)
	}
	il.histIdx = -1
	il.text = make([]rune, 0)
	il.cursor = 0
	return t
}

func (il *InputLine) HistPrev() {
	if len(il.history) == 0 {
		return
	}
	if il.histIdx == -1 {
		il.histIdx = len(il.history) - 1
	} else if il.histIdx > 0 {
		il.histIdx--
	}
	il.text = []rune(il.history[il.histIdx])
	il.cursor = len(il.text)
}

func (il *InputLine) HistNext() {
	if il.histIdx == -1 || len(il.history) == 0 {
		return
	}
	if il.histIdx < len(il.history)-1 {
		il.histIdx++
		il.text = []rune(il.history[il.histIdx])
		il.cursor = len(il.text)
	} else {
		il.histIdx = -1
		il.text = make([]rune, 0)
		il.cursor = 0
	}
}

const inputMinLines = 2

func (il *InputLine) View(disabled, hasTurns bool, width int) string {
	if width < 20 {
		width = 80
	}
	prompt := inputPromptStyle.Render("\u2192 ")
	promptWidth := runewidth.StringWidth("→ ")
	contentWidth := width - promptWidth
	if contentWidth < 10 {
		contentWidth = 10
	}

	if len(il.text) == 0 {
		placeholder := "Type a message and press Enter"
		if hasTurns {
			placeholder = "Add a follow-up"
		}
		if disabled {
			placeholder = "Agent working… (/yolo or Ctrl+Y to toggle mode)"
		}
		return padInputMinLines(renderInputLines(prompt, promptWidth, WrapDisplayWidth(placeholder, contentWidth), func(line string) string {
			return inputPlaceholderStyle.Render(line)
		}), inputMinLines, promptWidth)
	}

	full := string(il.text)
	if disabled {
		return padInputMinLines(renderInputLines(prompt, promptWidth, WrapDisplayWidth(full, contentWidth), func(line string) string {
			return inputPlaceholderStyle.Render(line)
		}), inputMinLines, promptWidth)
	}

	before := string(il.text[:il.cursor])
	after := ""
	if il.cursor < len(il.text) {
		after = string(il.text[il.cursor+1:])
	}
	cur := " "
	marker := "\x00"
	var plain string
	if il.cursor < len(il.text) {
		cur = string(il.text[il.cursor])
		plain = before + marker + after
	} else {
		plain = before + marker
	}
	lines := WrapDisplayWidth(plain, contentWidth)
	return padInputMinLines(renderInputLines(prompt, promptWidth, lines, func(line string) string {
		if !strings.Contains(line, marker) {
			return line
		}
		parts := strings.SplitN(line, marker, 2)
		if len(parts) == 2 {
			return parts[0] + inputCursorStyle.Render(cur) + parts[1]
		}
		return parts[0] + inputCursorStyle.Render(cur)
	}), inputMinLines, promptWidth)
}

func padInputMinLines(block string, minLines, promptWidth int) string {
	lines := strings.Split(block, "\n")
	for len(lines) < minLines {
		lines = append(lines, strings.Repeat(" ", promptWidth))
	}
	return strings.Join(lines, "\n")
}

func renderInputLines(prompt string, promptWidth int, lines []string, styleFn func(string) string) string {
	if len(lines) == 0 {
		return prompt
	}
	out := make([]string, len(lines))
	out[0] = prompt + styleFn(lines[0])
	for i := 1; i < len(lines); i++ {
		out[i] = strings.Repeat(" ", promptWidth) + styleFn(lines[i])
	}
	return strings.Join(out, "\n")
}
