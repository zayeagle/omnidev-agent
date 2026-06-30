package stream

import (
	"context"
	"fmt"
	"strings"

	"github.com/zayeagle/omnidev-agent/internal/llm"
)

// ChatWithRetry calls the LLM with retry. Network errors keep reconnecting by default
// (PersistNetworkRetry) until success or ctx cancellation; 4xx fail immediately.
func ChatWithRetry(ctx context.Context, provider llm.Provider, req *llm.Request, onChunk func(string), cfg RetryConfig) (*llm.Response, error) {
	if len(req.Tools) == 0 {
		if resp, err := collectStreamWithRetry(ctx, provider, req, onChunk, cfg); err == nil {
			return resp, nil
		}
	}
	resp, err := RetryChat(ctx, provider, req, cfg)
	if err != nil {
		return nil, err
	}
	if onChunk != nil && resp.Content != "" {
		EmitChunks(resp.Content, onChunk)
	}
	return resp, nil
}

func collectStreamWithRetry(ctx context.Context, provider llm.Provider, req *llm.Request, onChunk func(string), cfg RetryConfig) (*llm.Response, error) {
	return retryLLM(ctx, cfg, func() (*llm.Response, error) {
		return collectStream(ctx, provider, req, onChunk)
	})
}

func collectStream(ctx context.Context, provider llm.Provider, req *llm.Request, onChunk func(string)) (*llm.Response, error) {
	ch, err := provider.Stream(ctx, req)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	for chunk := range ch {
		if chunk.Error != "" {
			return nil, fmt.Errorf("%s", chunk.Error)
		}
		if chunk.Content != "" {
			b.WriteString(chunk.Content)
			if onChunk != nil {
				onChunk(chunk.Content)
			}
		}
		if chunk.Done {
			break
		}
	}
	if b.Len() == 0 {
		return nil, fmt.Errorf("stream returned no content")
	}
	return &llm.Response{Content: b.String()}, nil
}

// EmitChunks splits batched text into smaller pieces for progressive TUI rendering.
func EmitChunks(content string, onChunk func(string)) {
	if onChunk == nil || content == "" {
		return
	}
	runes := []rune(content)
	const step = 32
	for i := 0; i < len(runes); i += step {
		end := i + step
		if end > len(runes) {
			end = len(runes)
		}
		onChunk(string(runes[i:end]))
	}
}
