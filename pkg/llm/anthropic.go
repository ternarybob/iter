package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	anthropicAPIURL     = "https://api.anthropic.com/v1/messages"
	anthropicAPIVersion = "2023-06-01"
)

// AnthropicProvider implements the Provider interface for Claude.
type AnthropicProvider struct {
	apiKey     string
	httpClient *http.Client
	models     []string
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
		models: []string{
			"claude-sonnet-4-20250514",
			"claude-opus-4-20250514",
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
			"claude-3-opus-20240229",
		},
	}
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// Models returns available model identifiers.
func (p *AnthropicProvider) Models() []string {
	return p.models
}

// Complete generates a completion.
func (p *AnthropicProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	anthropicReq := p.toAnthropicRequest(req)

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, p.parseError(resp.StatusCode, respBody)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return p.fromAnthropicResponse(&anthropicResp), nil
}

// Stream generates a streaming completion.
func (p *AnthropicProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	anthropicReq := p.toAnthropicRequest(req)
	anthropicReq.Stream = true

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", anthropicAPIURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, p.parseError(resp.StatusCode, respBody)
	}

	ch := make(chan StreamChunk)
	go p.streamResponse(ctx, resp.Body, ch)

	return ch, nil
}

// CountTokens estimates token count.
func (p *AnthropicProvider) CountTokens(content string) (int, error) {
	// Rough estimate: ~4 characters per token
	return EstimateTokens(content), nil
}

// setHeaders sets the required HTTP headers.
func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
}

// anthropicRequest is the Anthropic API request format.
type anthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []anthropicMessage  `json:"messages"`
	System      string              `json:"system,omitempty"`
	MaxTokens   int                 `json:"max_tokens"`
	Temperature float64             `json:"temperature,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
	Stop        []string            `json:"stop_sequences,omitempty"`
	Tools       []anthropicTool     `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
}

type anthropicContentBlock struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Input     any    `json:"input,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// anthropicResponse is the Anthropic API response format.
type anthropicResponse struct {
	ID           string                  `json:"id"`
	Type         string                  `json:"type"`
	Role         string                  `json:"role"`
	Content      []anthropicContentBlock `json:"content"`
	Model        string                  `json:"model"`
	StopReason   string                  `json:"stop_reason"`
	StopSequence string                  `json:"stop_sequence,omitempty"`
	Usage        anthropicUsage          `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type anthropicErrorResponse struct {
	Type  string         `json:"type"`
	Error anthropicError `json:"error"`
}

// toAnthropicRequest converts our request to Anthropic format.
func (p *AnthropicProvider) toAnthropicRequest(req *CompletionRequest) *anthropicRequest {
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			continue // System is handled separately
		}

		anthropicMsg := anthropicMessage{
			Role:    msg.Role,
			Content: []anthropicContentBlock{},
		}

		if msg.Content != "" {
			anthropicMsg.Content = append(anthropicMsg.Content, anthropicContentBlock{
				Type: "text",
				Text: msg.Content,
			})
		}

		// Handle tool calls in assistant messages
		for _, tc := range msg.ToolCalls {
			var input any
			if err := json.Unmarshal([]byte(tc.Arguments), &input); err != nil {
				input = tc.Arguments
			}
			anthropicMsg.Content = append(anthropicMsg.Content, anthropicContentBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Name,
				Input: input,
			})
		}

		// Handle tool results
		if msg.Role == "tool" {
			anthropicMsg.Role = "user"
			anthropicMsg.Content = []anthropicContentBlock{{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
				IsError:   msg.IsError,
			}}
		}

		messages = append(messages, anthropicMsg)
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	anthropicReq := &anthropicRequest{
		Model:       req.Model,
		Messages:    messages,
		System:      req.System,
		MaxTokens:   maxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		Stop:        req.StopSequences,
	}

	// Convert tools
	if len(req.Tools) > 0 {
		anthropicReq.Tools = make([]anthropicTool, len(req.Tools))
		for i, tool := range req.Tools {
			schema := tool.Parameters
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			anthropicReq.Tools[i] = anthropicTool{
				Name:        tool.Name,
				Description: tool.Description,
				InputSchema: schema,
			}
		}
	}

	// Handle tool choice
	if req.ToolChoice != "" {
		switch req.ToolChoice {
		case "auto":
			anthropicReq.ToolChoice = &anthropicToolChoice{Type: "auto"}
		case "none":
			// Don't send tools if none
			anthropicReq.Tools = nil
		default:
			anthropicReq.ToolChoice = &anthropicToolChoice{
				Type: "tool",
				Name: req.ToolChoice,
			}
		}
	}

	return anthropicReq
}

// fromAnthropicResponse converts Anthropic response to our format.
func (p *AnthropicProvider) fromAnthropicResponse(resp *anthropicResponse) *CompletionResponse {
	result := &CompletionResponse{
		ID:           resp.ID,
		Model:        resp.Model,
		FinishReason: p.mapStopReason(resp.StopReason),
		Usage: TokenUsage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}

	var contentParts []string
	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			contentParts = append(contentParts, block.Text)
		case "tool_use":
			argsJSON, _ := json.Marshal(block.Input)
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(argsJSON),
			})
		}
	}

	result.Content = strings.Join(contentParts, "")

	return result
}

// mapStopReason converts Anthropic stop reason to our format.
func (p *AnthropicProvider) mapStopReason(reason string) string {
	switch reason {
	case "end_turn":
		return "stop"
	case "max_tokens":
		return "max_tokens"
	case "stop_sequence":
		return "stop"
	case "tool_use":
		return "tool_use"
	default:
		return reason
	}
}

// parseError parses an error response.
func (p *AnthropicProvider) parseError(statusCode int, body []byte) error {
	var errResp anthropicErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return &ProviderError{
			Provider: "anthropic",
			Code:     fmt.Sprintf("http_%d", statusCode),
			Message:  string(body),
		}
	}

	code := errResp.Error.Type
	switch statusCode {
	case 429:
		code = "rate_limit"
	case 401:
		code = "authentication_error"
	}

	return &ProviderError{
		Provider: "anthropic",
		Code:     code,
		Message:  errResp.Error.Message,
	}
}

// streamResponse handles streaming response.
func (p *AnthropicProvider) streamResponse(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer body.Close()
	defer close(ch)

	decoder := newSSEDecoder(body)
	var usage *TokenUsage

	for {
		select {
		case <-ctx.Done():
			ch <- StreamChunk{Error: ctx.Err()}
			return
		default:
		}

		event, err := decoder.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			ch <- StreamChunk{Error: err}
			return
		}

		chunk := p.parseStreamEvent(event)
		if chunk.Usage != nil {
			usage = chunk.Usage
		}
		if chunk.Content != "" || chunk.ToolCall != nil || chunk.Done {
			ch <- chunk
		}

		if chunk.Done {
			return
		}
	}

	// Final chunk
	ch <- StreamChunk{Done: true, Usage: usage}
}

// SSE event types
type sseEvent struct {
	Event string
	Data  []byte
}

type sseDecoder struct {
	reader *bytes.Reader
	data   []byte
}

func newSSEDecoder(r io.Reader) *sseDecoder {
	data, _ := io.ReadAll(r)
	return &sseDecoder{
		reader: bytes.NewReader(data),
		data:   data,
	}
}

func (d *sseDecoder) Next() (*sseEvent, error) {
	// Simple SSE parsing
	line, err := d.readLine()
	if err != nil {
		return nil, err
	}

	event := &sseEvent{}

	for line != "" {
		if strings.HasPrefix(line, "event: ") {
			event.Event = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			event.Data = []byte(strings.TrimPrefix(line, "data: "))
		}

		line, err = d.readLine()
		if err != nil && err != io.EOF {
			return nil, err
		}
		if line == "" || err == io.EOF {
			break
		}
	}

	return event, nil
}

func (d *sseDecoder) readLine() (string, error) {
	var line []byte
	for {
		b, err := d.reader.ReadByte()
		if err != nil {
			return string(line), err
		}
		if b == '\n' {
			return string(line), nil
		}
		if b != '\r' {
			line = append(line, b)
		}
	}
}

// parseStreamEvent parses a streaming event.
func (p *AnthropicProvider) parseStreamEvent(event *sseEvent) StreamChunk {
	chunk := StreamChunk{}

	switch event.Event {
	case "message_stop":
		chunk.Done = true
	case "content_block_delta":
		var delta struct {
			Type  string `json:"type"`
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
			} `json:"delta"`
		}
		if err := json.Unmarshal(event.Data, &delta); err == nil {
			if delta.Delta.Type == "text_delta" {
				chunk.Content = delta.Delta.Text
			}
		}
	case "message_delta":
		var delta struct {
			Usage anthropicUsage `json:"usage"`
		}
		if err := json.Unmarshal(event.Data, &delta); err == nil {
			chunk.Usage = &TokenUsage{
				PromptTokens:     delta.Usage.InputTokens,
				CompletionTokens: delta.Usage.OutputTokens,
				TotalTokens:      delta.Usage.InputTokens + delta.Usage.OutputTokens,
			}
		}
	}

	return chunk
}
