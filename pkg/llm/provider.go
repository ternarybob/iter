// Package llm provides LLM provider abstraction for multiple backends.
package llm

import (
	"context"
	"fmt"
)

// Provider defines the interface for LLM backends.
type Provider interface {
	// Name returns the provider name.
	Name() string

	// Models returns available model identifiers.
	Models() []string

	// Complete generates a completion.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// Stream generates a streaming completion.
	Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)

	// CountTokens estimates token count for content.
	CountTokens(content string) (int, error)
}

// CompletionRequest is a request to generate a completion.
type CompletionRequest struct {
	// Model is the model identifier.
	Model string `json:"model"`

	// Messages is the conversation history.
	Messages []Message `json:"messages"`

	// System is the system prompt.
	System string `json:"system,omitempty"`

	// MaxTokens limits the response length.
	MaxTokens int `json:"max_tokens,omitempty"`

	// Temperature controls randomness (0-1).
	Temperature float64 `json:"temperature,omitempty"`

	// TopP is nucleus sampling parameter.
	TopP float64 `json:"top_p,omitempty"`

	// StopSequences are strings that stop generation.
	StopSequences []string `json:"stop_sequences,omitempty"`

	// Tools are available function definitions.
	Tools []Tool `json:"tools,omitempty"`

	// ToolChoice controls tool selection behavior.
	// Values: "auto", "none", or specific tool name.
	ToolChoice string `json:"tool_choice,omitempty"`

	// Metadata contains additional request metadata.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// CompletionResponse is the response from a completion request.
type CompletionResponse struct {
	// ID is the response identifier.
	ID string `json:"id"`

	// Model is the model that generated the response.
	Model string `json:"model"`

	// Content is the text response.
	Content string `json:"content"`

	// ToolCalls are function invocations.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// FinishReason indicates why generation stopped.
	// Values: "stop", "max_tokens", "tool_use".
	FinishReason string `json:"finish_reason"`

	// Usage contains token counts.
	Usage TokenUsage `json:"usage"`
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	// PromptTokens is input token count.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is output token count.
	CompletionTokens int `json:"completion_tokens"`

	// TotalTokens is the sum.
	TotalTokens int `json:"total_tokens"`
}

// StreamChunk is a fragment of a streaming response.
type StreamChunk struct {
	// Content is the text fragment.
	Content string `json:"content,omitempty"`

	// ToolCall is a partial tool call.
	ToolCall *ToolCall `json:"tool_call,omitempty"`

	// Done indicates the stream is complete.
	Done bool `json:"done"`

	// Usage is populated on the final chunk.
	Usage *TokenUsage `json:"usage,omitempty"`

	// Error is any streaming error.
	Error error `json:"-"`
}

// Message represents a conversation message.
type Message struct {
	// Role is the message role.
	// Values: "user", "assistant", "system", "tool".
	Role string `json:"role"`

	// Content is the message content.
	Content string `json:"content"`

	// ToolCalls are tool invocations (for assistant messages).
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID links to a tool call (for tool result messages).
	ToolCallID string `json:"tool_call_id,omitempty"`

	// ToolResult is the result of a tool call (for tool messages).
	ToolResult string `json:"tool_result,omitempty"`

	// IsError indicates if the tool result is an error.
	IsError bool `json:"is_error,omitempty"`
}

// Tool defines a function the LLM can call.
type Tool struct {
	// Name is the function name.
	Name string `json:"name"`

	// Description describes what the function does.
	Description string `json:"description"`

	// Parameters is the JSON schema for parameters.
	Parameters map[string]any `json:"parameters,omitempty"`
}

// ToolCall represents a function invocation.
type ToolCall struct {
	// ID is the call identifier.
	ID string `json:"id"`

	// Name is the function name.
	Name string `json:"name"`

	// Arguments are the function arguments (JSON string).
	Arguments string `json:"arguments"`
}

// NewMessage creates a new message.
func NewMessage(role, content string) Message {
	return Message{
		Role:    role,
		Content: content,
	}
}

// UserMessage creates a user message.
func UserMessage(content string) Message {
	return NewMessage("user", content)
}

// AssistantMessage creates an assistant message.
func AssistantMessage(content string) Message {
	return NewMessage("assistant", content)
}

// SystemMessage creates a system message.
func SystemMessage(content string) Message {
	return NewMessage("system", content)
}

// ToolResultMessage creates a tool result message.
func ToolResultMessage(callID, result string, isError bool) Message {
	return Message{
		Role:       "tool",
		ToolCallID: callID,
		ToolResult: result,
		Content:    result,
		IsError:    isError,
	}
}

// ProviderError represents a provider-specific error.
type ProviderError struct {
	Provider string
	Code     string
	Message  string
	Err      error
}

func (e *ProviderError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%s): %v", e.Provider, e.Message, e.Code, e.Err)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Provider, e.Message, e.Code)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}

// IsRateLimitError checks if the error is a rate limit error.
func IsRateLimitError(err error) bool {
	if pe, ok := err.(*ProviderError); ok {
		return pe.Code == "rate_limit" || pe.Code == "rate_limit_exceeded"
	}
	return false
}

// IsAuthError checks if the error is an authentication error.
func IsAuthError(err error) bool {
	if pe, ok := err.(*ProviderError); ok {
		return pe.Code == "authentication_error" || pe.Code == "invalid_api_key"
	}
	return false
}

// IsContextLengthError checks if the error is a context length error.
func IsContextLengthError(err error) bool {
	if pe, ok := err.(*ProviderError); ok {
		return pe.Code == "context_length_exceeded"
	}
	return false
}
