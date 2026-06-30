package components

import "testing"

func TestInputLine_PushHistoryDedupesConsecutive(t *testing.T) {
	il := NewInputLine()
	il.PushHistory("hello")
	il.PushHistory("hello")
	il.PushHistory("world")
	if len(il.history) != 2 {
		t.Fatalf("history len = %d, want 2", len(il.history))
	}
}

func TestInputLine_HistPrevNext(t *testing.T) {
	il := NewInputLine()
	il.PushHistory("first")
	il.PushHistory("second")

	il.HistPrev()
	if il.Text() != "second" {
		t.Fatalf("HistPrev = %q, want second", il.Text())
	}
	il.HistPrev()
	if il.Text() != "first" {
		t.Fatalf("HistPrev = %q, want first", il.Text())
	}
	il.HistNext()
	if il.Text() != "second" {
		t.Fatalf("HistNext = %q, want second", il.Text())
	}
	il.HistNext()
	if il.Text() != "" {
		t.Fatalf("HistNext past end = %q, want empty", il.Text())
	}
}

func TestInputLine_SubmitDoesNotRecordHistory(t *testing.T) {
	il := NewInputLine()
	il.SetText("draft")
	got := il.Submit()
	if got != "draft" {
		t.Fatalf("Submit = %q", got)
	}
	if len(il.history) != 0 {
		t.Fatal("Submit should not append to history")
	}
}

func TestInputLine_LoadHistory(t *testing.T) {
	il := NewInputLine()
	il.LoadHistory([]string{"a", "b", "b", "c"})
	if len(il.history) != 3 {
		t.Fatalf("LoadHistory len = %d, want 3", len(il.history))
	}
	il.HistPrev()
	if il.Text() != "c" {
		t.Fatalf("last history = %q, want c", il.Text())
	}
}
