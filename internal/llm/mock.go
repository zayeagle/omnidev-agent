package llm

import (
	"context"
	"fmt"
	"time"
)

// MockProvider returns canned responses for testing.
// Useful for unit tests of Agent Loop without a real LLM backend.
type MockProvider struct {
	responses []*Response
	index     int
	chatDelay time.Duration
	streamDelay time.Duration
}

// NewMockProvider creates a MockProvider that cycles through the given responses.
func NewMockProvider(responses []*Response) *MockProvider {
	return &MockProvider{
		responses:    responses,
		chatDelay:    10 * time.Millisecond,
		streamDelay:  5 * time.Millisecond,
	}
}

// Chat returns the next canned response.
func (m *MockProvider) Chat(ctx context.Context, req *Request) (*Response, error) {
	if m.index >= len(m.responses) {
		return nil, fmt.Errorf("mock: no more responses")
	}
	time.Sleep(m.chatDelay)
	resp := m.responses[m.index]
	m.index++
	return resp, nil
}

// Stream returns the content of the next canned response as a single chunk.
func (m *MockProvider) Stream(ctx context.Context, req *Request) (<-chan *Chunk, error) {
	ch := make(chan *Chunk, 8)
	go func() {
		defer close(ch)
		if m.index >= len(m.responses) {
			ch <- &Chunk{Done: true}
			return
		}
		resp := m.responses[m.index]
		m.index++

		// Emit content chunk-by-chunk (one character at a time for realism)
		for _, r := range resp.Content {
			select {
			case ch <- &Chunk{Content: string(r)}:
				time.Sleep(m.streamDelay)
			case <-ctx.Done():
				return
			}
		}
		ch <- &Chunk{Done: true}
	}()
	return ch, nil
}

// AddResponse appends a canned response for subsequent calls.
func (m *MockProvider) AddResponse(resp *Response) {
	m.responses = append(m.responses, resp)
}

// Reset rewinds the response index to 0.
func (m *MockProvider) Reset() {
	m.index = 0
}
