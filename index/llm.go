package index

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// LLMClient provides access to LLM APIs for summarization.
type LLMClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// LLMConfig configures the LLM client.
type LLMConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// DefaultLLMConfig returns the default LLM configuration.
// Uses Gemini Flash by default.
func DefaultLLMConfig() LLMConfig {
	return LLMConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		BaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Model:   "gemini-1.5-flash",
		Timeout: 30 * time.Second,
	}
}

// NewLLMClient creates a new LLM client.
// Returns nil if no API key is configured.
func NewLLMClient(cfg LLMConfig) *LLMClient {
	if cfg.APIKey == "" {
		return nil
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-1.5-flash"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &LLMClient{
		apiKey:  cfg.APIKey,
		baseURL: cfg.BaseURL,
		model:   cfg.Model,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// Generate generates text from a prompt using the LLM.
// Returns the generated text and the model used.
func (c *LLMClient) Generate(prompt string) (string, string, error) {
	if c == nil {
		return "", "", fmt.Errorf("LLM client not configured")
	}

	// Build Gemini API request
	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:     0.3,
			MaxOutputTokens: 256,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("marshal request: %w", err)
	}

	// Make request
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s",
		c.baseURL, c.model, c.apiKey)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp geminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", "", fmt.Errorf("empty response from API")
	}

	text := geminiResp.Candidates[0].Content.Parts[0].Text
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
	return c != nil && c.apiKey != ""
}

// Model returns the model name.
func (c *LLMClient) Model() string {
	if c == nil {
		return ""
	}
	return c.model
}

// Gemini API types

type geminiRequest struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}
