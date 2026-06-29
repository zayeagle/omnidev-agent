package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/zayeagle/omnidev-agent/internal/llm"
	"github.com/zayeagle/omnidev-agent/internal/stream"
)

func TestChatWithRetry_StreamsWithoutTools(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{Content: "Hello from the model."},
	})

	var parts []string
	resp, err := stream.ChatWithRetry(context.Background(), mock, &llm.Request{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(s string) {
		parts = append(parts, s)
	}, stream.DefaultRetryConfig())
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content == "" {
		t.Fatal("expected content")
	}
	if len(parts) == 0 {
		t.Fatal("expected streamed chunks")
	}
	if strings.Join(parts, "") != resp.Content {
		t.Fatalf("chunks %q != content %q", strings.Join(parts, ""), resp.Content)
	}
}

func TestEmitChunks_SplitsBatch(t *testing.T) {
	var n int
	stream.EmitChunks("abcdefghijklmnopqrstuvwxyz", func(s string) {
		n += len(s)
	})
	if n != 26 {
		t.Fatalf("expected 26 runes emitted, got %d", n)
	}
}
