package tests

import (
	"context"
	"testing"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/llm"
)

// TestMockProviderChat verifies canned response delivery.
func TestMockProviderChat(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{Content: "Hello, world!"},
		{Content: "Goodbye."},
	})

	resp1, err := mock.Chat(context.Background(), &llm.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp1.Content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", resp1.Content)
	}

	resp2, err := mock.Chat(context.Background(), &llm.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp2.Content != "Goodbye." {
		t.Errorf("expected 'Goodbye.', got %q", resp2.Content)
	}

	// Third call should error (no more responses)
	_, err = mock.Chat(context.Background(), &llm.Request{})
	if err == nil {
		t.Error("expected error for exhausted responses")
	}
}

// TestMockProviderStream verifies streaming output.
func TestMockProviderStream(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{Content: "ABC"},
	})

	ch, err := mock.Stream(context.Background(), &llm.Request{})
	if err != nil {
		t.Fatal(err)
	}

	var content string
	timeout := time.After(2 * time.Second)
	for {
		select {
		case chunk, ok := <-ch:
			if !ok {
				t.Fatal("channel closed unexpectedly")
			}
			if chunk.Done {
				goto verify
			}
			content += chunk.Content
		case <-timeout:
			t.Fatal("stream timed out")
		}
	}
verify:
	if content != "ABC" {
		t.Errorf("expected 'ABC', got %q", content)
	}
}

// TestMockProviderReset verifies the index reset.
func TestMockProviderReset(t *testing.T) {
	mock := llm.NewMockProvider([]*llm.Response{
		{Content: "first"},
	})

	mock.Chat(context.Background(), &llm.Request{})
	// Exhausted — next call should error
	_, err := mock.Chat(context.Background(), &llm.Request{})
	if err == nil {
		t.Error("expected error after exhaustion")
	}

	mock.Reset()
	resp, err := mock.Chat(context.Background(), &llm.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "first" {
		t.Errorf("expected 'first' after reset, got %q", resp.Content)
	}
}
