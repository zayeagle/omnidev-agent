package components

import (
	"fmt"
	"strings"
	"testing"
)

func TestTurnList_Scroll(t *testing.T) {
	tl := NewTurnList(10)
	tl.SetWidth(80)

	turn := tl.AddTurn(1, "hello")
	for i := 0; i < 40; i++ {
		turn.AppendReply(fmt.Sprintf("output line %d with some padding", i))
	}
	turn.FlushReply("")

	viewH := 10
	full, _ := tl.ViewScroll(viewH, nil, nil)
	if full == "" {
		t.Fatal("expected content")
	}

	tl.ScrollUp(5, viewH, nil, nil)
	if tl.AtBottom() {
		t.Fatal("expected scrolled up")
	}
	up, _ := tl.ViewScroll(viewH, nil, nil)
	if up == full {
		t.Fatal("expected different view after scroll up")
	}

	tl.ScrollToBottom()
	if !tl.AtBottom() {
		t.Fatal("expected at bottom after ScrollToBottom")
	}
}

func TestTurnList_ViewScroll_IncludesSuffix(t *testing.T) {
	tl := NewTurnList(10)
	tl.SetWidth(80)
	turn := tl.AddTurn(1, "hello")
	for i := 0; i < 20; i++ {
		turn.AppendReply(fmt.Sprintf("line %d", i))
	}
	turn.FlushReply("")

	suffix := []string{"", "TAIL MARKER"}
	view, _ := tl.ViewScroll(8, nil, suffix)
	if !strings.Contains(view, "TAIL MARKER") {
		t.Fatalf("expected suffix visible at bottom, got:\n%s", view)
	}
}
