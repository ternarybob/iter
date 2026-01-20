// Package iter provides an SDK for building autonomous DevOps agents.
//
// Iter (Latin: "journey, path") is a pure Go SDK for building autonomous
// DevOps agents that iteratively improve codebases. Unlike CLI wrappers,
// Iter is designed as an embeddable library with a plugin architecture
// for extensible skills.
//
// # Quick Start
//
//	agent, err := iter.New(
//	    iter.WithAnthropicKey(apiKey),
//	    iter.WithWorkDir("."),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	task := iter.NewTask("Add a health check endpoint")
//	result, err := agent.Run(ctx, task)
//
// # Architecture
//
// Iter uses a multi-agent adversarial architecture:
//   - ARCHITECT: Analyzes requirements and creates step documents
//   - WORKER: Implements steps exactly as specified
//   - VALIDATOR: Adversarially reviews implementations (default REJECT)
//
// # Core Principles
//
//   - SDK-First: Embeddable library, not a CLI tool
//   - Plugin Architecture: Skills are first-class interfaces
//   - Codebase-Aware: Built-in indexing and semantic search
//   - Multi-Model: Strategic routing (planning vs execution)
//   - Adversarial Validation: Multi-agent architecture with hostile review
//   - Correctness over Speed: Requirements are law, validation is mandatory
package iter

import (
	"context"
	"log/slog"

	"github.com/ternarybob/iter/pkg/agent"
	"github.com/ternarybob/iter/pkg/config"
	"github.com/ternarybob/iter/pkg/index"
	"github.com/ternarybob/iter/pkg/llm"
	"github.com/ternarybob/iter/pkg/sdk"
)

// Agent is an alias for the core agent type.
type Agent = agent.Agent

// Task is an alias for the sdk task type.
type Task = sdk.Task

// Result is an alias for the sdk result type.
type Result = sdk.Result

// Skill is an alias for the sdk skill interface.
type Skill = sdk.Skill

// SkillMetadata is an alias for skill metadata.
type SkillMetadata = sdk.SkillMetadata

// Config is an alias for the sdk config type.
type Config = sdk.Config

// Option is a functional option for configuring an agent.
type Option = agent.Option

// New creates a new agent with the provided options.
func New(opts ...Option) (*Agent, error) {
	return agent.New(opts...)
}

// NewTask creates a new task with the given description.
func NewTask(description string) *Task {
	return sdk.NewTask(description)
}

// Option constructors

// WithAnthropicKey configures the Anthropic API.
func WithAnthropicKey(apiKey string) Option {
	return func(a *Agent) error {
		provider := llm.NewAnthropicProvider(apiKey)
		router := llm.NewRouter(provider)
		adapter := llm.NewSDKAdapter(router)
		return agent.WithLLM(adapter)(a)
	}
}

// WithOllama configures Ollama as the LLM provider.
func WithOllama(baseURL string) Option {
	return func(a *Agent) error {
		provider := llm.NewOllamaProvider(baseURL)
		router := llm.NewRouter(provider)
		adapter := llm.NewSDKAdapter(router)
		return agent.WithLLM(adapter)(a)
	}
}

// WithMemoryIndex configures in-memory indexing.
func WithMemoryIndex() Option {
	return func(a *Agent) error {
		idx := index.NewMemoryIndex()
		adapter := index.NewSDKAdapter(idx)
		return agent.WithIndex(adapter)(a)
	}
}

// WithWorkDir sets the working directory.
func WithWorkDir(dir string) Option {
	return agent.WithWorkDir(dir)
}

// WithClaudeConfig loads configuration from a .claude directory.
func WithClaudeConfig(dir string) Option {
	return func(a *Agent) error {
		cfg, err := config.Load(dir)
		if err != nil {
			// Non-fatal - use defaults
			slog.Warn("failed to load .claude config", "error", err)
			return nil
		}
		return agent.WithConfig(cfg)(a)
	}
}

// WithConfig sets the agent configuration.
func WithConfig(cfg *Config) Option {
	return agent.WithConfig(cfg)
}

// WithLogger sets the agent's logger.
func WithLogger(logger *slog.Logger) Option {
	return agent.WithLogger(logger)
}

// WithDryRun enables dry-run mode.
func WithDryRun(dryRun bool) Option {
	return agent.WithDryRun(dryRun)
}

// WithMaxIterations sets the maximum iterations.
func WithMaxIterations(max int) Option {
	return agent.WithMaxIterations(max)
}

// WithRateLimit sets the hourly rate limit.
func WithRateLimit(perHour int) Option {
	return agent.WithRateLimit(perHour)
}

// WithPlanningModel sets the model for architect agent.
func WithPlanningModel(model string) Option {
	return agent.WithPlanningModel(model)
}

// WithExecutionModel sets the model for worker agent.
func WithExecutionModel(model string) Option {
	return agent.WithExecutionModel(model)
}

// WithValidationModel sets the model for validator agent.
func WithValidationModel(model string) Option {
	return agent.WithValidationModel(model)
}

// WithSkills registers multiple skills at once.
func WithSkills(skills ...Skill) Option {
	return agent.WithSkills(skills...)
}

// Run executes a single task without looping.
func Run(ctx context.Context, a *Agent, task *Task) (*Result, error) {
	return a.Run(ctx, task)
}

// RunLoop executes the autonomous iteration loop.
func RunLoop(ctx context.Context, a *Agent, task *Task) error {
	return a.RunLoop(ctx, task)
}

// LoadConfig loads configuration from a directory.
func LoadConfig(dir string) (*Config, error) {
	return config.Load(dir)
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return config.DefaultConfig()
}
