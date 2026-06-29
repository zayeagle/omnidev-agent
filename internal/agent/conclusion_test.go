package agent

import (
	"strings"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/session"
)

func TestLooksLikeReviewInstruction(t *testing.T) {
	if !looksLikeReviewInstruction("检查一下贪吃蛇是否完善") {
		t.Fatal("expected review instruction")
	}
	if looksLikeReviewInstruction("hello") {
		t.Fatal("greeting should not be review")
	}
}

func TestNeedsMoreReview(t *testing.T) {
	s := session.New()
	s.AddWithState("user", "检查项目是否完善", "", 0)
	if !needsMoreReview("检查项目是否完善", s) {
		t.Fatal("empty exploration should need more review")
	}
	s.Add(session.Entry{
		Role: "assistant",
		AssistantToolCalls: []session.ToolCallData{
			{Name: "list_dir"},
			{Name: "read_file"},
			{Name: "read_file"},
			{Name: "read_file"},
			{Name: "read_file"},
		},
	})
	s.AddWithState("assistant", "结论：游戏逻辑已实现，缺少单元测试，可以编译。", "", 0)
	if needsMoreReview("检查项目是否完善", s) {
		t.Fatal("substantial conclusion should not need more review")
	}
}

func TestHasSubstantialConclusionText(t *testing.T) {
	if !hasSubstantialConclusionText("结论：核心玩法已实现，但缺少计分和边界检测。") {
		t.Fatal("expected substantial Chinese conclusion")
	}
	if hasSubstantialConclusionText("ok") {
		t.Fatal("short ok should not count")
	}
}

func TestExtractConclusionFromSession(t *testing.T) {
	s := session.New()
	s.AddWithState("assistant", "Final: snake logic complete.", "", 0)
	got := extractConclusionFromSession(s, []TaskResult{{TaskID: "1", Content: "Checked domain layer."}})
	if !strings.Contains(got, "Final:") || !strings.Contains(got, "Checked domain") {
		t.Fatalf("unexpected extract: %q", got)
	}
}
