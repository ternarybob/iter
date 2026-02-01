package index

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

// LLMClient provides access to Gemini API for summarization.
type LLMClient struct {
	client   *genai.Client
	model    string
	thinking string
	timeout  time.Duration
}

// LLMConfig configures the LLM client.
type LLMConfig struct {
	APIKey   string
	Model    string
	Thinking string // NONE, LOW, NORMAL, HIGH
	Timeout  time.Duration
}

// DefaultLLMConfig returns the default LLM configuration.
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		APIKey:   os.Getenv("GOOGLE_GEMINI_API_KEY"),
		Model:    "gemini-3-flash-preview",
		Thinking: "NORMAL",
		Timeout:  30 * time.Second,
	}
}

// NewLLMClient creates a new LLM client using the Gemini SDK.
// Returns nil if no API key is configured.
func NewLLMClient(cfg LLMConfig) *LLMClient {
	if cfg.APIKey == "" {
		return nil
	}

	if cfg.Model == "" {
		cfg.Model = "gemini-3-flash-preview"
	}
	if cfg.Thinking == "" {
		cfg.Thinking = "NORMAL"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil
	}

	return &LLMClient{
		client:   client,
		model:    cfg.Model,
		thinking: cfg.Thinking,
		timeout:  cfg.Timeout,
	}
}

// thinkingLevel converts string thinking level to SDK enum.
func thinkingLevel(level string) genai.ThinkingLevel {
	switch strings.ToUpper(level) {
	case "NONE":
		return genai.ThinkingLevelMinimal
	case "LOW":
		return genai.ThinkingLevelLow
	case "NORMAL":
		return genai.ThinkingLevelMedium
	case "HIGH":
		return genai.ThinkingLevelHigh
	default:
		return genai.ThinkingLevelMedium
	}
}

// Generate generates text from a prompt using the Gemini API.
// Returns the generated text and the model used.
func (c *LLMClient) Generate(prompt string) (string, string, error) {
	if c == nil || c.client == nil {
		return "", "", fmt.Errorf("LLM client not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Configure generation with thinking level
	config := &genai.GenerateContentConfig{
		ThinkingConfig: &genai.ThinkingConfig{
			ThinkingLevel: thinkingLevel(c.thinking),
		},
	}

	// Use genai.Text helper to create content from text
	result, err := c.client.Models.GenerateContent(ctx, c.model, genai.Text(prompt), config)
	if err != nil {
		return "", "", fmt.Errorf("generate content: %w", err)
	}

	if result == nil || len(result.Candidates) == 0 {
		return "", "", fmt.Errorf("empty response from API")
	}

	// Extract text from response parts
	var text string
	if result.Candidates[0].Content != nil {
		for _, part := range result.Candidates[0].Content.Parts {
			if part != nil && part.Text != "" {
				text += part.Text
			}
		}
	}

	if text == "" {
		return "", "", fmt.Errorf("no text in response")
	}

	return text, c.model, nil
}

// SummarizeDiff generates a summary of a git diff.
func (c *LLMClient) SummarizeDiff(diff string, commitMessage string) (string, error) {
	if c == nil {
		return commitMessage, nil // Fallback to commit message
	}

	// Truncate diff if too long
	maxDiffLen := 4000
	if len(diff) > maxDiffLen {
		diff = diff[:maxDiffLen] + "\n... (truncated)"
	}

	prompt := fmt.Sprintf(`Summarize this git commit in 1-2 sentences. Focus on WHAT changed and WHY.

Commit message: %s

Diff:
%s

Summary:`, commitMessage, diff)

	text, _, err := c.Generate(prompt)
	if err != nil {
		return commitMessage, err // Fallback to commit message on error
	}

	return text, nil
}

// IsConfigured returns whether the LLM client has an API key.
func (c *LLMClient) IsConfigured() bool {
	return c != nil && c.client != nil
}

// Model returns the model name.
func (c *LLMClient) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}
