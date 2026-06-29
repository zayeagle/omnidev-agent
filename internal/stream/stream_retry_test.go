package stream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zayeagle/omnidev-agent/internal/llm"
)

type failThenOKProvider struct {
	calls int
}

func (p *failThenOKProvider) Chat(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	p.calls++
	if p.calls == 1 {
		return nil, errors.New("temporary network error")
	}
	return &llm.Response{Content: "ok"}, nil
}

func (p *failThenOKProvider) Stream(ctx context.Context, req *llm.Request) (<-chan *llm.Chunk, error) {
	return nil, errors.New("not implemented")
}

func TestRetryChat_RespectsConfig(t *testing.T) {
	p := &failThenOKProvider{}
	cfg := RetryConfig{
		MaxRetries: 2,
		Backoffs:   []time.Duration{time.Millisecond, 2 * time.Millisecond},
	}
	resp, err := RetryChat(context.Background(), p, &llm.Request{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, cfg)
	if err != nil {
		t.Fatalf("expected success on second attempt: %v", err)
	}
	if resp.Content != "ok" || p.calls != 2 {
		t.Fatalf("calls=%d content=%q", p.calls, resp.Content)
	}
}

func TestRetryChat_ZeroRetries(t *testing.T) {
	p := &failThenOKProvider{}
	_, err := RetryChat(context.Background(), p, &llm.Request{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, RetryConfig{MaxRetries: 0, Backoffs: []time.Duration{time.Millisecond}})
	if err == nil {
		t.Fatal("expected error with zero retries")
	}
	if p.calls != 1 {
		t.Fatalf("expected 1 attempt, got %d", p.calls)
	}
}
