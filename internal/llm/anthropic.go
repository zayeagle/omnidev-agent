package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const anthropicAPIVersion = "2023-06-01"

// AnthropicClient implements Provider using Anthropic Messages API.
type AnthropicClient struct {
	baseURL    string
	apiKey     string
	model      string
	opts       Options
	httpClient *http.Client
}

// NewAnthropic creates an Anthropic Messages API provider.
func NewAnthropic(baseURL, apiKey, model string, opts Options) *AnthropicClient {
	opts = opts.Resolved()
	return &AnthropicClient{
		baseURL: NormalizeBaseURL(baseURL),
		apiKey:  apiKey,
		model:   model,
		opts:    opts,
		httpClient: &http.Client{
			Timeout: time.Duration(opts.TimeoutSec) * time.Second,
		},
	}
}

func (c *AnthropicClient) endpoint() string {
	return c.baseURL + "/messages"
}

// Chat sends a non-streaming request.
func (c *AnthropicClient) Chat(ctx context.Context, req *Request) (*Response, error) {
	body := c.buildRequest(req, false)
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode >= 400 {
		return nil, fmt.Errorf("anthropic: %d %s", httpResp.StatusCode, string(respBody))
	}

	var result anthropicResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return convertAnthropicResponse(&result), nil
}

// Stream sends a streaming request. Tool calls fall back to non-streaming Chat.
func (c *AnthropicClient) Stream(ctx context.Context, req *Request) (<-chan *Chunk, error) {
	if len(req.Tools) > 0 {
		resp, err := c.Chat(ctx, req)
		if err != nil {
			return nil, err
		}
		ch := make(chan *Chunk, 1)
		go func() {
			defer close(ch)
			if resp.Content != "" {
				ch <- &Chunk{Content: resp.Content}
			}
			ch <- &Chunk{Done: true}
		}()
		return ch, nil
	}

	body := c.buildRequest(req, true)
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	if httpResp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		return nil, fmt.Errorf("anthropic: %d %s", httpResp.StatusCode, string(respBody))
	}

	ch := make(chan *Chunk, 64)
	go c.readSSE(ctx, httpResp.Body, ch)
	return ch, nil
}

func (c *AnthropicClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", anthropicAPIVersion)
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
}

func (c *AnthropicClient) buildRequest(req *Request, stream bool) anthropicRequest {
	system, messages := convertAnthropicMessages(req.Messages)
	out := anthropicRequest{
		Model:       c.model,
		MaxTokens:   c.opts.MaxTokens,
		System:      system,
		Messages:    messages,
		Temperature: c.opts.Temperature,
	}
	if stream {
		out.Stream = true
	}
	if len(req.Tools) > 0 {
		out.Tools = convertAnthropicTools(req.Tools)
	}
	return out
}

func (c *AnthropicClient) readSSE(ctx context.Context, body io.ReadCloser, ch chan<- *Chunk) {
	defer close(ch)
	defer body.Close()

	parser := &sseLineParser{}
	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		n, err := body.Read(buf)
		if n > 0 {
			for _, payload := range parser.feed(buf[:n]) {
				if payload == nil || bytes.Equal(payload, []byte("__DONE__")) {
					continue
				}
				var event anthropicStreamEvent
				if json.Unmarshal(payload, &event) != nil {
					continue
				}
				switch event.Type {
				case "content_block_delta":
					if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
						ch <- &Chunk{Content: event.Delta.Text}
					}
				case "message_stop":
					ch <- &Chunk{Done: true}
					return
				}
			}
		}
		if err != nil {
			ch <- &Chunk{Done: true}
			return
		}
	}
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []anthropicMessage `json:"messages"`
	Tools       []anthropicTool    `json:"tools,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	Content []anthropicContentBlock `json:"content"`
	StopReason string               `json:"stop_reason"`
}

type anthropicContentBlock struct {
	Type  string                 `json:"type"`
	Text  string                 `json:"text,omitempty"`
	ID    string                 `json:"id,omitempty"`
	Name  string                 `json:"name,omitempty"`
	Input map[string]interface{} `json:"input,omitempty"`
}

type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

func convertAnthropicTools(tools []Tool) []anthropicTool {
	out := make([]anthropicTool, len(tools))
	for i, t := range tools {
		out[i] = anthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: toolParametersSchema(t.Parameters),
		}
	}
	return out
}

func convertAnthropicMessages(msgs []Message) (string, []anthropicMessage) {
	var systemParts []string
	var out []anthropicMessage

	for _, m := range msgs {
		switch m.Role {
		case "system":
			if m.Content != "" {
				systemParts = append(systemParts, m.Content)
			}
		case "tool":
			out = append(out, anthropicMessage{
				Role: "user",
				Content: []map[string]interface{}{
					{
						"type":        "tool_result",
						"tool_use_id": m.ToolCallID,
						"content":     m.Content,
					},
				},
			})
		case "assistant":
			var blocks []map[string]interface{}
			if m.Content != "" {
				blocks = append(blocks, map[string]interface{}{
					"type": "text",
					"text": m.Content,
				})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, map[string]interface{}{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Name,
					"input": tc.Arguments,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, map[string]interface{}{"type": "text", "text": ""})
			}
			out = append(out, anthropicMessage{Role: "assistant", Content: blocks})
		default:
			out = append(out, anthropicMessage{Role: m.Role, Content: m.Content})
		}
	}
	return joinStrings(systemParts, "\n\n"), out
}

func convertAnthropicResponse(resp *anthropicResponse) *Response {
	r := &Response{}
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			if block.Text != "" {
				if r.Content != "" {
					r.Content += "\n"
				}
				r.Content += block.Text
			}
		case "tool_use":
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: block.Input,
			})
		}
	}
	return r
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += sep + parts[i]
	}
	return out
}
