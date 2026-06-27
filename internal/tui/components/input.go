package components

import (
	"github.com/charmbracelet/lipgloss"
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

func (il *InputLine) View(disabled, hasTurns bool) string {
	prompt := inputPromptStyle.Render("\u2192 ")

	if len(il.text) == 0 {
		placeholder := "Type a message and press Enter"
		if hasTurns {
			placeholder = "Add a follow-up"
		}
		if disabled {
			placeholder = "Agent working…"
		}
		return prompt + inputPlaceholderStyle.Render(placeholder)
	}
	if disabled {
		return prompt + inputPlaceholderStyle.Render(string(il.text))
	}

	before := string(il.text[:il.cursor])

	at := ""
	after := ""
	if il.cursor < len(il.text) {
		at = inputCursorStyle.Render(string(il.text[il.cursor]))
		after = string(il.text[il.cursor+1:])
	} else {
		at = inputCursorStyle.Render(" ")
	}

	return prompt + before + at + after
}
