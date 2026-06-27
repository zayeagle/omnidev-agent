package components

import (
	"fmt"
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
	full := tl.View(viewH, nil)
	if full == "" {
		t.Fatal("expected content")
	}

	tl.ScrollUp(5, viewH)
	if tl.AtBottom() {
		t.Fatal("expected scrolled up")
	}
	up := tl.View(viewH, nil)
	if up == full {
		t.Fatal("expected different view after scroll up")
	}

	tl.ScrollToBottom()
	if !tl.AtBottom() {
		t.Fatal("expected at bottom after ScrollToBottom")
	}
}
