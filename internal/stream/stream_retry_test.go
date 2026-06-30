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

type persistentFailProvider struct {
	calls int
	max   int
}

func (p *persistentFailProvider) Chat(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	p.calls++
	if p.calls <= p.max {
		return nil, errors.New("dial tcp: connection refused")
	}
	return &llm.Response{Content: "ok"}, nil
}

func (p *persistentFailProvider) Stream(ctx context.Context, req *llm.Request) (<-chan *llm.Chunk, error) {
	return nil, errors.New("dial tcp: connection refused")
}

type genericFailProvider struct {
	calls int
}

func (p *genericFailProvider) Chat(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	p.calls++
	return nil, errors.New("llm: invalid response format")
}

func (p *genericFailProvider) Stream(ctx context.Context, req *llm.Request) (<-chan *llm.Chunk, error) {
	return nil, errors.New("not implemented")
}

func TestRetryChat_RespectsConfig(t *testing.T) {
	p := &failThenOKProvider{}
	cfg := RetryConfig{
		MaxRetries:          2,
		Backoffs:            []time.Duration{time.Millisecond, 2 * time.Millisecond},
		PersistNetworkRetry: true,
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
	p := &genericFailProvider{}
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

func TestRetryChat_PersistentNetworkBeyondMaxRetries(t *testing.T) {
	p := &persistentFailProvider{max: 5}
	cfg := RetryConfig{
		MaxRetries:          0,
		Backoffs:            []time.Duration{time.Millisecond},
		PersistNetworkRetry: true,
	}
	resp, err := RetryChat(context.Background(), p, &llm.Request{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, cfg)
	if err != nil {
		t.Fatalf("expected success after persistent retries: %v", err)
	}
	if resp.Content != "ok" || p.calls != 6 {
		t.Fatalf("calls=%d content=%q", p.calls, resp.Content)
	}
}

func TestRetryChat_NonNetworkRespectsMaxRetries(t *testing.T) {
	p := &genericFailProvider{}
	cfg := RetryConfig{
		MaxRetries:          0,
		Backoffs:            []time.Duration{time.Millisecond},
		PersistNetworkRetry: true,
	}
	_, err := RetryChat(context.Background(), p, &llm.Request{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, cfg)
	if err == nil {
		t.Fatal("expected failure for non-network error with zero retries")
	}
	if p.calls != 1 {
		t.Fatalf("expected 1 attempt, got %d", p.calls)
	}
}

func TestIsNetworkError(t *testing.T) {
	if !isNetworkError(errors.New("dial tcp 127.0.0.1:443: connect: connection refused")) {
		t.Fatal("expected connection refused as network error")
	}
	if isNetworkError(errors.New("llm: 401 unauthorized")) {
		t.Fatal("401 should not be network error")
	}
}
