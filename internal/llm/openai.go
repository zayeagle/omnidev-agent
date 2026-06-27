package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// OpenAIClient implements Provider using the OpenAI-compatible Chat Completions API.
type OpenAIClient struct {
	baseURL    string
	apiKey     string
	model      string
	opts       Options
	httpClient *http.Client
}

// NewOpenAI creates an OpenAI-compatible Provider.
func NewOpenAI(baseURL, apiKey, model string, opts Options) *OpenAIClient {
	opts = opts.Resolved()
	return &OpenAIClient{
		baseURL: NormalizeBaseURL(baseURL),
		apiKey:  apiKey,
		model:   model,
		opts:    opts,
		httpClient: &http.Client{
			Timeout: time.Duration(opts.TimeoutSec) * time.Second,
		},
	}
}

// Chat sends a request and accepts JSON or SSE responses (many gateways stream even without stream=true).
func (c *OpenAIClient) Chat(ctx context.Context, req *Request) (*Response, error) {
	body := c.buildRequest(req, false)
	resp, err := c.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	return convertResponse(resp), nil
}

// Stream sends a streaming request and returns a channel of chunks.
func (c *OpenAIClient) Stream(ctx context.Context, req *Request) (<-chan *Chunk, error) {
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
		if err := extractSSEError(respBody); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("openai: %d %s", httpResp.StatusCode, string(respBody))
	}

	ch := make(chan *Chunk, 64)
	go c.readSSE(ctx, httpResp.Body, ch)
	return ch, nil
}

func (c *OpenAIClient) endpoint() string {
	return c.baseURL + "/chat/completions"
}

// GatewayMode returns the configured gateway compatibility profile.
func (c *OpenAIClient) GatewayMode() string {
	return c.opts.GatewayMode
}

func (c *OpenAIClient) buildRequest(req *Request, stream bool) openAIRequest {
	msgs := req.Messages
	if c.opts.GatewayMode == GatewayStrict {
		msgs = NormalizeStrictGatewayMessages(msgs)
	}

	body := openAIRequest{
		Model:     c.model,
		Messages:  convertMessages(msgs),
		MaxTokens: effectiveMaxTokens(c.opts.MaxTokens, len(req.Tools)),
	}
	if !c.opts.OmitTemperature {
		t := c.opts.Temperature
		body.Temperature = &t
	}
	if stream {
		body.Stream = true
	}
	if len(req.Tools) > 0 {
		body.Tools = convertTools(req.Tools)
		body.ToolChoice = "auto"
	}
	return body
}

// effectiveMaxTokens caps completion budget on tool-call requests. Many enterprise
// gateways reject large max_tokens together with function tools (模型推理异常).
func effectiveMaxTokens(configured, toolCount int) int {
	if toolCount > 0 && configured > StrictGatewayMaxTokensCap {
		return StrictGatewayMaxTokensCap
	}
	return configured
}

func (c *OpenAIClient) doRequest(ctx context.Context, body openAIRequest) (*openAIResponse, error) {
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(rawBody))
	if err != nil {
		return nil, err
	}
	c.setHeaders(httpReq)

	if os.Getenv("OMNIDEV_LLM_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "llm → POST %s (%d bytes)\n%s\n", c.endpoint(), len(rawBody), rawBody)
	}

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}
	if os.Getenv("OMNIDEV_LLM_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "llm ← HTTP %d (%d bytes)\n%s\n", httpResp.StatusCode, len(respBody), truncateBody(respBody))
	}
	if httpResp.StatusCode >= 400 {
		if err := extractSSEError(respBody); err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("openai: %d %s", httpResp.StatusCode, string(respBody))
	}

	var result openAIResponse
	parsed, err := parseChatCompletionBody(respBody)
	if err != nil {
		return nil, err
	}
	result = *parsed
	return &result, nil
}

func (c *OpenAIClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
}

func (c *OpenAIClient) readSSE(ctx context.Context, body io.ReadCloser, ch chan<- *Chunk) {
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
				if payload == nil {
					continue
				}
				if bytes.Equal(payload, []byte("__DONE__")) {
					ch <- &Chunk{Done: true}
					return
				}
				if err := parseAPIErrorJSON(payload); err != nil {
					ch <- &Chunk{Error: err.Error(), Done: true}
					return
				}
				var chunk openAIStreamChunk
				if json.Unmarshal(payload, &chunk) != nil {
					continue
				}
				for _, choice := range chunk.Choices {
					if choice.Delta.Content != nil && *choice.Delta.Content != "" {
						ch <- &Chunk{Content: *choice.Delta.Content}
					}
					if choice.FinishReason != "" {
						ch <- &Chunk{Done: true}
						return
					}
				}
			}
		}
		if err != nil {
			ch <- &Chunk{Done: true}
			return
		}
	}
}

// sseLineParser buffers partial SSE lines across reads.
type sseLineParser struct {
	buf []byte
}

func (p *sseLineParser) feed(raw []byte) [][]byte {
	p.buf = append(p.buf, raw...)
	var payloads [][]byte
	for {
		idx := bytes.Index(p.buf, []byte("\n"))
		if idx < 0 {
			break
		}
		line := bytes.TrimSpace(p.buf[:idx])
		p.buf = p.buf[idx+1:]
		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data: ")))
		if len(data) == 0 {
			continue
		}
		if string(data) == "[DONE]" {
			payloads = append(payloads, []byte("__DONE__"))
			continue
		}
		payloads = append(payloads, data)
	}
	return payloads
}

type openAIRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Tools       []openAITool    `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    *string          `json:"-"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

func (m openAIMessage) MarshalJSON() ([]byte, error) {
	out := map[string]any{"role": m.Role}
	if m.Content != nil {
		out["content"] = *m.Content
	}
	if m.ToolCallID != "" {
		out["tool_call_id"] = m.ToolCallID
	}
	if len(m.ToolCalls) > 0 {
		out["tool_calls"] = m.ToolCalls
	}
	return json.Marshal(out)
}

func (m *openAIMessage) UnmarshalJSON(data []byte) error {
	var aux struct {
		Role       string           `json:"role"`
		Content    *string          `json:"content"`
		ToolCallID string           `json:"tool_call_id"`
		ToolCalls  []openAIToolCall `json:"tool_calls"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	m.Role = aux.Role
	m.Content = aux.Content
	m.ToolCallID = aux.ToolCallID
	m.ToolCalls = aux.ToolCalls
	return nil
}

type openAITool struct {
	Type     string     `json:"type"`
	Function openAIFunc `json:"function"`
}

type openAIFunc struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openAIToolCall struct {
	Index    *int           `json:"index,omitempty"`
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Function openAIFuncCall `json:"function"`
}

type openAIFuncCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
}

type openAIChoice struct {
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIStreamChunk struct {
	Choices []openAIStreamChoice `json:"choices"`
}

type openAIStreamChoice struct {
	Delta        openAIMessage `json:"delta"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

func convertMessages(msgs []Message) []openAIMessage {
	out := make([]openAIMessage, len(msgs))
	for i, m := range msgs {
		o := openAIMessage{
			Role:       m.Role,
			ToolCallID: m.ToolCallID,
		}
		if m.Content != "" {
			c := m.Content
			o.Content = &c
		} else if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			// Omit content field entirely for tool-only assistant messages.
			o.Content = nil
		} else if m.Role == "tool" {
			empty := ""
			o.Content = &empty
		} else {
			empty := ""
			o.Content = &empty
		}
		if len(m.ToolCalls) > 0 {
			for _, tc := range m.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Arguments)
				o.ToolCalls = append(o.ToolCalls, openAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openAIFuncCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
		}
		out[i] = o
	}
	return out
}

func convertTools(tools []Tool) []openAITool {
	out := make([]openAITool, len(tools))
	for i, t := range tools {
		out[i] = openAITool{
			Type: "function",
			Function: openAIFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  toolParametersSchema(t.Parameters),
			},
		}
	}
	return out
}

func convertResponse(resp *openAIResponse) *Response {
	if len(resp.Choices) == 0 {
		return &Response{}
	}
	msg := resp.Choices[0].Message
	finish := resp.Choices[0].FinishReason

	r := &Response{}
	if msg.Content != nil {
		r.Content = *msg.Content
	}
	if finish == "tool_calls" || len(msg.ToolCalls) > 0 {
		for _, tc := range msg.ToolCalls {
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				args = map[string]interface{}{}
			}
			r.ToolCalls = append(r.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: args,
			})
		}
	}
	return r
}
