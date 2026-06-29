package agent

import (
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestLooksLikeSimpleChat_SnakeFollowUpNotChat(t *testing.T) {
	msg := "你再次检查一下 贪吃蛇的游戏完成了吗？最后编译成一个二进制"
	if looksLikeSimpleChat(msg) {
		t.Fatal("follow-up compile task should not be classified as simple chat")
	}
	if !looksLikeCodeMod(msg) {
		t.Fatal("should detect code mod intent")
	}
}

func TestLooksLikeSimpleChat_QuestionMarkAloneNotEnough(t *testing.T) {
	if !looksLikeSimpleChat("what is a goroutine?") {
		t.Fatal("short conceptual question should be chat")
	}
	if looksLikeSimpleChat("is the build finished? run go build ./...") {
		t.Fatal("build question should not be chat")
	}
}

func TestHasPriorCodeActivity_FollowUpTurn(t *testing.T) {
	a := New(nil, nil, nil, session.New())
	a.session.AddWithState("user", "build snake game", "Thinking", 0)
	a.session.AddWithState("user", "check again", "Thinking", 0)
	if !a.hasPriorCodeActivity() {
		t.Fatal("second user turn should count as prior activity context")
	}
}

func TestClassifyIntentFollowUpUsesCodeMod(t *testing.T) {
	a := New(nil, nil, nil, session.New())
	a.session.AddWithState("user", "build snake", "Thinking", 0)
	a.SetOutputDir("deliverables/snake-game")
	a.session.AddWithState("user", "你再次检查一下完成了吗？最后编译成一个二进制", "Thinking", 0)
	got := a.classifyIntent(t.Context(), "你再次检查一下完成了吗？最后编译成一个二进制", nil)
	if got != IntentCodeMod {
		t.Fatalf("intent=%q want code_modification", got)
	}
}
