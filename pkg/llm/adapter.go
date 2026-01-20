package llm

import (
	"context"

	"github.com/ternarybob/iter/pkg/sdk"
)

// SDKAdapter wraps a Router to implement sdk.LLMRouter.
type SDKAdapter struct {
	router *Router
	ctx    context.Context
}

// NewSDKAdapter creates an adapter that implements sdk.LLMRouter.
func NewSDKAdapter(router *Router) *SDKAdapter {
	return &SDKAdapter{
		router: router,
		ctx:    context.Background(),
	}
}

// WithContext sets the context for LLM calls.
func (a *SDKAdapter) WithContext(ctx context.Context) *SDKAdapter {
	return &SDKAdapter{
		router: a.router,
		ctx:    ctx,
	}
}

// Complete implements sdk.LLMRouter.
func (a *SDKAdapter) Complete(req sdk.CompletionRequest) (*sdk.CompletionResponse, error) {
	llmReq := convertToLLMRequest(req)
	resp, err := a.router.Complete(a.ctx, llmReq)
	if err != nil {
		return nil, err
	}
	return convertToSDKResponse(resp), nil
}

// Stream implements sdk.LLMRouter.
func (a *SDKAdapter) Stream(req sdk.CompletionRequest) (<-chan sdk.StreamChunk, error) {
	llmReq := convertToLLMRequest(req)
	llmCh, err := a.router.Stream(a.ctx, llmReq)
	if err != nil {
		return nil, err
	}

	sdkCh := make(chan sdk.StreamChunk)
	go func() {
		defer close(sdkCh)
		for chunk := range llmCh {
			sdkChunk := sdk.StreamChunk{
				Content: chunk.Content,
				Done:    chunk.Done,
				Error:   chunk.Error,
			}
			if chunk.ToolCall != nil {
				sdkChunk.ToolCall = &sdk.ToolCall{
					ID:        chunk.ToolCall.ID,
					Name:      chunk.ToolCall.Name,
					Arguments: chunk.ToolCall.Arguments,
				}
			}
			sdkCh <- sdkChunk
		}
	}()

	return sdkCh, nil
}

// CountTokens implements sdk.LLMRouter.
func (a *SDKAdapter) CountTokens(content string) (int, error) {
	return a.router.CountTokens(content)
}

// ForPlanning implements sdk.LLMRouter.
func (a *SDKAdapter) ForPlanning() sdk.LLMProvider {
	return &providerAdapter{
		provider: a.router.ForPlanning(),
		ctx:      a.ctx,
	}
}

// ForExecution implements sdk.LLMRouter.
func (a *SDKAdapter) ForExecution() sdk.LLMProvider {
	return &providerAdapter{
		provider: a.router.ForExecution(),
		ctx:      a.ctx,
	}
}

// ForValidation implements sdk.LLMRouter.
func (a *SDKAdapter) ForValidation() sdk.LLMProvider {
	return &providerAdapter{
		provider: a.router.ForValidation(),
		ctx:      a.ctx,
	}
}

// providerAdapter wraps a Provider to implement sdk.LLMProvider.
type providerAdapter struct {
	provider Provider
	ctx      context.Context
}

// Name implements sdk.LLMProvider.
func (p *providerAdapter) Name() string {
	return p.provider.Name()
}

// Complete implements sdk.LLMProvider.
func (p *providerAdapter) Complete(req sdk.CompletionRequest) (*sdk.CompletionResponse, error) {
	llmReq := convertToLLMRequest(req)
	resp, err := p.provider.Complete(p.ctx, llmReq)
	if err != nil {
		return nil, err
	}
	return convertToSDKResponse(resp), nil
}

// Stream implements sdk.LLMProvider.
func (p *providerAdapter) Stream(req sdk.CompletionRequest) (<-chan sdk.StreamChunk, error) {
	llmReq := convertToLLMRequest(req)
	llmCh, err := p.provider.Stream(p.ctx, llmReq)
	if err != nil {
		return nil, err
	}

	sdkCh := make(chan sdk.StreamChunk)
	go func() {
		defer close(sdkCh)
		for chunk := range llmCh {
			sdkChunk := sdk.StreamChunk{
				Content: chunk.Content,
				Done:    chunk.Done,
				Error:   chunk.Error,
			}
			if chunk.ToolCall != nil {
				sdkChunk.ToolCall = &sdk.ToolCall{
					ID:        chunk.ToolCall.ID,
					Name:      chunk.ToolCall.Name,
					Arguments: chunk.ToolCall.Arguments,
				}
			}
			sdkCh <- sdkChunk
		}
	}()

	return sdkCh, nil
}

// CountTokens implements sdk.LLMProvider.
func (p *providerAdapter) CountTokens(content string) (int, error) {
	return p.provider.CountTokens(content)
}

// convertToLLMRequest converts sdk.CompletionRequest to llm.CompletionRequest.
func convertToLLMRequest(req sdk.CompletionRequest) *CompletionRequest {
	llmReq := &CompletionRequest{
		Model:         req.Model,
		System:        req.System,
		MaxTokens:     req.MaxTokens,
		Temperature:   req.Temperature,
		TopP:          req.TopP,
		StopSequences: req.StopWords,
		ToolChoice:    req.ToolChoice,
	}

	// Convert messages
	for _, msg := range req.Messages {
		llmMsg := Message{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		for _, tc := range msg.ToolCalls {
			llmMsg.ToolCalls = append(llmMsg.ToolCalls, ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})
		}
		llmReq.Messages = append(llmReq.Messages, llmMsg)
	}

	// Convert tools
	for _, tool := range req.Tools {
		llmReq.Tools = append(llmReq.Tools, Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		})
	}

	return llmReq
}

// convertToSDKResponse converts llm.CompletionResponse to sdk.CompletionResponse.
func convertToSDKResponse(resp *CompletionResponse) *sdk.CompletionResponse {
	sdkResp := &sdk.CompletionResponse{
		Content:      resp.Content,
		FinishReason: resp.FinishReason,
		Usage: sdk.TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	for _, tc := range resp.ToolCalls {
		sdkResp.ToolCalls = append(sdkResp.ToolCalls, sdk.ToolCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: tc.Arguments,
		})
	}

	return sdkResp
}
