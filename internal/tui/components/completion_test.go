package components

import "testing"

func TestCompletionPanelLines_ChatModeHidden(t *testing.T) {
	tn := NewTurn(1, "你好")
	tn.SetChatMode(true)
	tn.SetCompletion("Task completed.", "")
	if lines := CompletionPanelLines(tn, 80); len(lines) != 0 {
		t.Fatalf("chat mode should not show completion panel, got %v", lines)
	}
}

func TestTurnReplyVisibleWithCompletion(t *testing.T) {
	tn := NewTurn(1, "你好")
	tn.SetChatMode(true)
	tn.AppendReply("你好！有什么可以帮你的？")
	tn.SetCompletion("Task completed.", "")

	body := joinLines(tn.render(80, false))
	if !containsSubstring(body, "有什么可以帮") {
		t.Fatalf("reply should remain visible with completion set: %q", body)
	}
}
