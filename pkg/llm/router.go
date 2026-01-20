package llm

import (
	"context"
	"fmt"
	"sync"
)

// Router provides multi-model routing capabilities.
// It routes requests to different models based on the task type.
type Router struct {
	mu sync.RWMutex

	// Provider is the underlying LLM provider.
	provider Provider

	// Model configurations
	planningModel   string
	executionModel  string
	validationModel string
	defaultModel    string
}

// NewRouter creates a new router with the given provider.
func NewRouter(provider Provider) *Router {
	models := provider.Models()
	defaultModel := ""
	if len(models) > 0 {
		defaultModel = models[0]
	}

	return &Router{
		provider:        provider,
		planningModel:   defaultModel,
		executionModel:  defaultModel,
		validationModel: defaultModel,
		defaultModel:    defaultModel,
	}
}

// SetPlanningModel sets the model for planning/architect tasks.
func (r *Router) SetPlanningModel(model string) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.planningModel = model
	return r
}

// SetExecutionModel sets the model for execution/worker tasks.
func (r *Router) SetExecutionModel(model string) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.executionModel = model
	return r
}

// SetValidationModel sets the model for validation tasks.
func (r *Router) SetValidationModel(model string) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.validationModel = model
	return r
}

// SetDefaultModel sets the default model.
func (r *Router) SetDefaultModel(model string) *Router {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultModel = model
	return r
}

// PlanningModel returns the planning model.
func (r *Router) PlanningModel() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.planningModel
}

// ExecutionModel returns the execution model.
func (r *Router) ExecutionModel() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.executionModel
}

// ValidationModel returns the validation model.
func (r *Router) ValidationModel() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.validationModel
}

// ForPlanning returns a provider configured for planning.
func (r *Router) ForPlanning() Provider {
	return &routedProvider{
		router: r,
		model:  r.PlanningModel(),
	}
}

// ForExecution returns a provider configured for execution.
func (r *Router) ForExecution() Provider {
	return &routedProvider{
		router: r,
		model:  r.ExecutionModel(),
	}
}

// ForValidation returns a provider configured for validation.
func (r *Router) ForValidation() Provider {
	return &routedProvider{
		router: r,
		model:  r.ValidationModel(),
	}
}

// Provider returns the underlying provider.
func (r *Router) Provider() Provider {
	return r.provider
}

// Name returns the router name.
func (r *Router) Name() string {
	return "router:" + r.provider.Name()
}

// Models returns available models.
func (r *Router) Models() []string {
	return r.provider.Models()
}

// Complete generates a completion using the default model.
func (r *Router) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if req.Model == "" {
		req.Model = r.defaultModel
	}
	return r.provider.Complete(ctx, req)
}

// Stream generates a streaming completion using the default model.
func (r *Router) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	if req.Model == "" {
		req.Model = r.defaultModel
	}
	return r.provider.Stream(ctx, req)
}

// CountTokens estimates token count.
func (r *Router) CountTokens(content string) (int, error) {
	return r.provider.CountTokens(content)
}

// routedProvider wraps a router with a fixed model.
type routedProvider struct {
	router *Router
	model  string
}

func (p *routedProvider) Name() string {
	return p.router.provider.Name()
}

func (p *routedProvider) Models() []string {
	return []string{p.model}
}

func (p *routedProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	req.Model = p.model
	return p.router.provider.Complete(ctx, req)
}

func (p *routedProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	req.Model = p.model
	return p.router.provider.Stream(ctx, req)
}

func (p *routedProvider) CountTokens(content string) (int, error) {
	return p.router.provider.CountTokens(content)
}

// MultiProvider combines multiple providers with fallback.
type MultiProvider struct {
	providers []Provider
	primary   int
}

// NewMultiProvider creates a provider with fallback support.
func NewMultiProvider(providers ...Provider) *MultiProvider {
	return &MultiProvider{
		providers: providers,
		primary:   0,
	}
}

// SetPrimary sets the primary provider index.
func (mp *MultiProvider) SetPrimary(index int) error {
	if index < 0 || index >= len(mp.providers) {
		return fmt.Errorf("invalid provider index: %d", index)
	}
	mp.primary = index
	return nil
}

// Name returns the provider name.
func (mp *MultiProvider) Name() string {
	if len(mp.providers) == 0 {
		return "multi:empty"
	}
	return "multi:" + mp.providers[mp.primary].Name()
}

// Models returns all available models across providers.
func (mp *MultiProvider) Models() []string {
	seen := make(map[string]bool)
	var models []string
	for _, p := range mp.providers {
		for _, m := range p.Models() {
			if !seen[m] {
				seen[m] = true
				models = append(models, m)
			}
		}
	}
	return models
}

// Complete tries providers in order until one succeeds.
func (mp *MultiProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if len(mp.providers) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}

	var lastErr error

	// Try primary first
	if resp, err := mp.providers[mp.primary].Complete(ctx, req); err == nil {
		return resp, nil
	} else {
		lastErr = err
		// Don't fallback on auth errors
		if IsAuthError(err) {
			return nil, err
		}
	}

	// Try fallbacks
	for i, p := range mp.providers {
		if i == mp.primary {
			continue
		}
		if resp, err := p.Complete(ctx, req); err == nil {
			return resp, nil
		} else {
			lastErr = err
		}
	}

	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// Stream tries providers in order until one succeeds.
func (mp *MultiProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	if len(mp.providers) == 0 {
		return nil, fmt.Errorf("no providers configured")
	}

	var lastErr error

	// Try primary first
	if ch, err := mp.providers[mp.primary].Stream(ctx, req); err == nil {
		return ch, nil
	} else {
		lastErr = err
		if IsAuthError(err) {
			return nil, err
		}
	}

	// Try fallbacks
	for i, p := range mp.providers {
		if i == mp.primary {
			continue
		}
		if ch, err := p.Stream(ctx, req); err == nil {
			return ch, nil
		} else {
			lastErr = err
		}
	}

	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// CountTokens uses the primary provider.
func (mp *MultiProvider) CountTokens(content string) (int, error) {
	if len(mp.providers) == 0 {
		return 0, fmt.Errorf("no providers configured")
	}
	return mp.providers[mp.primary].CountTokens(content)
}
