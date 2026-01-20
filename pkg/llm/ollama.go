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

const (
	ollamaDefaultURL = "http://localhost:11434"
)

// OllamaProvider implements the Provider interface for Ollama.
type OllamaProvider struct {
	baseURL    string
	httpClient *http.Client
	models     []string
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(baseURL string) *OllamaProvider {
	if baseURL == "" {
		baseURL = ollamaDefaultURL
	}
	return &OllamaProvider{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		models: []string{}, // Will be populated by list call
	}
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return "ollama"
}

// Models returns available model identifiers.
func (p *OllamaProvider) Models() []string {
	if len(p.models) == 0 {
		p.refreshModels()
	}
	return p.models
}

// refreshModels fetches available models from Ollama.
func (p *OllamaProvider) refreshModels() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
	if err != nil {
		return
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return
	}

	p.models = make([]string, len(result.Models))
	for i, m := range result.Models {
		p.models[i] = m.Name
	}
}

// Complete generates a completion.
func (p *OllamaProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	ollamaReq := p.toOllamaRequest(req)
	ollamaReq.Stream = false

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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
		return nil, &ProviderError{
			Provider: "ollama",
			Code:     fmt.Sprintf("http_%d", resp.StatusCode),
			Message:  string(respBody),
		}
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	return p.fromOllamaResponse(&ollamaResp), nil
}

// Stream generates a streaming completion.
func (p *OllamaProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	ollamaReq := p.toOllamaRequest(req)
	ollamaReq.Stream = true

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &ProviderError{
			Provider: "ollama",
			Code:     fmt.Sprintf("http_%d", resp.StatusCode),
			Message:  string(respBody),
		}
	}

	ch := make(chan StreamChunk)
	go p.streamResponse(ctx, resp.Body, ch)

	return ch, nil
}

// CountTokens estimates token count.
func (p *OllamaProvider) CountTokens(content string) (int, error) {
	// Rough estimate: ~4 characters per token
	return EstimateTokens(content), nil
}

// ollamaRequest is the Ollama API request format.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	Stop        []string `json:"stop,omitempty"`
}

// ollamaResponse is the Ollama API response format.
type ollamaResponse struct {
	Model     string        `json:"model"`
	Message   ollamaMessage `json:"message"`
	Done      bool          `json:"done"`
	DoneReason string       `json:"done_reason"`
	TotalDuration int64     `json:"total_duration"`
	PromptEvalCount int     `json:"prompt_eval_count"`
	EvalCount int           `json:"eval_count"`
}

// toOllamaRequest converts our request to Ollama format.
func (p *OllamaProvider) toOllamaRequest(req *CompletionRequest) *ollamaRequest {
	messages := make([]ollamaMessage, 0, len(req.Messages)+1)

	// Add system message first if present
	if req.System != "" {
		messages = append(messages, ollamaMessage{
			Role:    "system",
			Content: req.System,
		})
	}

	for _, msg := range req.Messages {
		if msg.Role == "system" {
			// Skip system messages as we handle them above
			continue
		}
		if msg.Role == "tool" {
			// Convert tool results to user messages
			messages = append(messages, ollamaMessage{
				Role:    "user",
				Content: fmt.Sprintf("[Tool Result]: %s", msg.Content),
			})
			continue
		}
		messages = append(messages, ollamaMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	ollamaReq := &ollamaRequest{
		Model:    req.Model,
		Messages: messages,
	}

	if req.Temperature > 0 || req.TopP > 0 || req.MaxTokens > 0 || len(req.StopSequences) > 0 {
		ollamaReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			TopP:        req.TopP,
			NumPredict:  req.MaxTokens,
			Stop:        req.StopSequences,
		}
	}

	return ollamaReq
}

// fromOllamaResponse converts Ollama response to our format.
func (p *OllamaProvider) fromOllamaResponse(resp *ollamaResponse) *CompletionResponse {
	finishReason := "stop"
	if resp.DoneReason == "length" {
		finishReason = "max_tokens"
	}

	return &CompletionResponse{
		Model:        resp.Model,
		Content:      resp.Message.Content,
		FinishReason: finishReason,
		Usage: TokenUsage{
			PromptTokens:     resp.PromptEvalCount,
			CompletionTokens: resp.EvalCount,
			TotalTokens:      resp.PromptEvalCount + resp.EvalCount,
		},
	}
}

// streamResponse handles streaming response from Ollama.
func (p *OllamaProvider) streamResponse(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer body.Close()
	defer close(ch)

	decoder := json.NewDecoder(body)
	var totalTokens TokenUsage

	for {
		select {
		case <-ctx.Done():
			ch <- StreamChunk{Error: ctx.Err()}
			return
		default:
		}

		var resp ollamaResponse
		if err := decoder.Decode(&resp); err != nil {
			if err == io.EOF {
				break
			}
			ch <- StreamChunk{Error: err}
			return
		}

		totalTokens.PromptTokens = resp.PromptEvalCount
		totalTokens.CompletionTokens = resp.EvalCount
		totalTokens.TotalTokens = resp.PromptEvalCount + resp.EvalCount

		if resp.Message.Content != "" {
			ch <- StreamChunk{Content: resp.Message.Content}
		}

		if resp.Done {
			ch <- StreamChunk{Done: true, Usage: &totalTokens}
			return
		}
	}
}

// IsAvailable checks if Ollama is available.
func (p *OllamaProvider) IsAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
