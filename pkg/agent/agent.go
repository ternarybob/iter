// Package agent provides the core agent implementation for Iter.
// It orchestrates skill execution, manages the autonomous loop, and
// coordinates with the multi-agent system.
package agent

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ternarybob/iter/pkg/sdk"
)

// Agent is the main orchestrator for autonomous DevOps tasks.
type Agent struct {
	mu sync.RWMutex

	// Core components
	config   *sdk.Config
	registry *Registry
	hooks    *sdk.HookRegistry
	logger   *slog.Logger

	// Provider interfaces (set via options)
	llm       sdk.LLMRouter
	index     sdk.Index
	session   sdk.Session
	orchestra sdk.Orchestrator
	workdir   sdk.WorkdirManager

	// File system (for testing)
	fs fs.FS

	// State
	workDir     string
	iteration   int
	running     bool
	lastResult  *sdk.Result
	exitSignal  bool
	startTime   time.Time
	totalTokens int

	// Loop control
	loopCtx    context.Context
	loopCancel context.CancelFunc

	// Circuit breaker and rate limiting
	circuit     *CircuitBreaker
	rateLimiter *RateLimiter

	// Monitoring
	monitor Monitor

	// Options
	dryRun bool
}

// New creates a new Agent with the provided options.
func New(opts ...Option) (*Agent, error) {
	a := &Agent{
		registry: NewRegistry(),
		hooks:    sdk.NewHookRegistry(),
		logger:   slog.Default(),
		config:   defaultConfig(),
		workDir:  ".",
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(a); err != nil {
			return nil, fmt.Errorf("apply option: %w", err)
		}
	}

	// Initialize circuit breaker if not set
	if a.circuit == nil {
		a.circuit = NewCircuitBreaker(CircuitBreakerConfig{
			NoProgressThreshold:    a.config.Circuit.NoProgressThreshold,
			SameErrorThreshold:     a.config.Circuit.SameErrorThreshold,
			OutputDeclineThreshold: a.config.Circuit.OutputDeclineThreshold,
			RecoveryTimeout:        parseDuration(a.config.Circuit.RecoveryTimeout, 5*time.Minute),
		})
	}

	// Initialize rate limiter if not set
	if a.rateLimiter == nil && a.config.Loop.RateLimitPerHour > 0 {
		a.rateLimiter = NewRateLimiter(a.config.Loop.RateLimitPerHour)
	}

	return a, nil
}

// RegisterSkill adds a skill to the agent's registry.
func (a *Agent) RegisterSkill(skill sdk.Skill) error {
	return a.registry.Register(skill)
}

// RegisterSkillFunc creates and registers a functional skill.
func (a *Agent) RegisterSkillFunc(meta sdk.SkillMetadata,
	canHandle func(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (bool, float64),
	execute func(ctx context.Context, execCtx *sdk.ExecutionContext, plan *sdk.Plan) (*sdk.Result, error)) error {

	skill := sdk.NewSkillFunc(meta).
		OnCanHandle(canHandle).
		OnExecute(execute)

	return a.registry.Register(skill)
}

// RegisterHook adds a lifecycle hook.
func (a *Agent) RegisterHook(hookType sdk.HookType, hook sdk.Hook) {
	a.hooks.Register(hookType, hook)
}

// Run executes a single task without looping.
func (a *Agent) Run(ctx context.Context, task *sdk.Task) (*sdk.Result, error) {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil, fmt.Errorf("agent is already running")
	}
	a.running = true
	a.startTime = time.Now()
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.running = false
		a.mu.Unlock()
	}()

	execCtx := a.createExecutionContext()
	return a.executeTask(ctx, execCtx, task)
}

// RunLoop executes the autonomous iteration loop.
func (a *Agent) RunLoop(ctx context.Context, task *sdk.Task) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("agent is already running")
	}
	a.running = true
	a.startTime = time.Now()
	a.iteration = 0
	a.exitSignal = false
	a.loopCtx, a.loopCancel = context.WithCancel(ctx)
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.running = false
		a.loopCancel()
		a.mu.Unlock()
	}()

	return a.runLoop(task)
}

// Stop signals the agent to stop after the current iteration.
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loopCancel != nil {
		a.loopCancel()
	}
	a.exitSignal = true
}

// IsRunning returns whether the agent is currently executing.
func (a *Agent) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// Iteration returns the current iteration number.
func (a *Agent) Iteration() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.iteration
}

// LastResult returns the result from the last iteration.
func (a *Agent) LastResult() *sdk.Result {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastResult
}

// Index indexes a directory into the codebase store.
func (a *Agent) Index(ctx context.Context, root string, patterns []string) error {
	if a.index == nil {
		return fmt.Errorf("no index configured")
	}
	// Index implementation will be in the index package
	a.logger.Info("indexing codebase", "root", root, "patterns", patterns)
	return nil
}

// GetSkills returns metadata for all registered skills.
func (a *Agent) GetSkills() []sdk.SkillMetadata {
	return a.registry.ListMetadata()
}

// createExecutionContext builds the context for skill execution.
func (a *Agent) createExecutionContext() *sdk.ExecutionContext {
	return &sdk.ExecutionContext{
		Codebase:  a.index,
		LLM:       a.llm,
		Session:   a.session,
		Config:    a.config,
		FS:        a.fs,
		WorkDir:   a.workDir,
		Logger:    a.logger,
		Iteration: a.iteration,
		DryRun:    a.dryRun,
		Orchestra: a.orchestra,
		Workdir:   a.workdir,
	}
}

// executeTask runs a single task through the skill pipeline.
func (a *Agent) executeTask(ctx context.Context, execCtx *sdk.ExecutionContext, task *sdk.Task) (*sdk.Result, error) {
	// Find best matching skill
	skill, confidence := a.registry.FindBest(ctx, execCtx, task)
	if skill == nil {
		return nil, fmt.Errorf("no skill can handle task: %s", task.Description)
	}

	a.logger.Info("selected skill",
		"skill", skill.Metadata().Name,
		"confidence", confidence,
		"task", task.ID)

	// Run pre-plan hooks
	hookCtx := &sdk.HookContext{
		Type:    sdk.HookTypePrePlan,
		Task:    task,
		ExecCtx: execCtx,
	}
	if err := a.hooks.Run(ctx, hookCtx); err != nil {
		return nil, fmt.Errorf("pre-plan hook: %w", err)
	}

	// Plan
	plan, err := skill.Plan(ctx, execCtx, task)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}

	// Run post-plan hooks
	hookCtx.Type = sdk.HookTypePostPlan
	hookCtx.Plan = plan
	if err := a.hooks.Run(ctx, hookCtx); err != nil {
		return nil, fmt.Errorf("post-plan hook: %w", err)
	}

	// Run pre-execute hooks
	hookCtx.Type = sdk.HookTypePreExecute
	if err := a.hooks.Run(ctx, hookCtx); err != nil {
		return nil, fmt.Errorf("pre-execute hook: %w", err)
	}

	// Execute
	result, err := skill.Execute(ctx, execCtx, plan)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	// Run post-execute hooks
	hookCtx.Type = sdk.HookTypePostExecute
	hookCtx.Result = result
	if err := a.hooks.Run(ctx, hookCtx); err != nil {
		return nil, fmt.Errorf("post-execute hook: %w", err)
	}

	// Run pre-validate hooks
	hookCtx.Type = sdk.HookTypePreValidate
	if err := a.hooks.Run(ctx, hookCtx); err != nil {
		return nil, fmt.Errorf("pre-validate hook: %w", err)
	}

	// Validate
	if valErr := skill.Validate(ctx, execCtx, result); valErr != nil {
		result.Status = sdk.ResultStatusFailed
		result.Error = valErr
		result.ErrorMessage = valErr.Error()
	}

	// Run post-validate hooks
	hookCtx.Type = sdk.HookTypePostValidate
	if err := a.hooks.Run(ctx, hookCtx); err != nil {
		return nil, fmt.Errorf("post-validate hook: %w", err)
	}

	a.mu.Lock()
	a.lastResult = result
	a.totalTokens += result.Metrics.TokensUsed
	a.mu.Unlock()

	return result, nil
}

// runLoop implements the autonomous iteration loop.
func (a *Agent) runLoop(task *sdk.Task) error {
	maxIterations := a.config.Loop.MaxIterations
	if maxIterations == 0 {
		maxIterations = 1000 // Reasonable default
	}

	iterationTimeout := parseDuration(a.config.Loop.IterationTimeout, 15*time.Minute)
	cooldown := parseDuration(a.config.Loop.Cooldown, 5*time.Second)

	for {
		a.mu.Lock()
		a.iteration++
		iteration := a.iteration
		a.mu.Unlock()

		// Check exit conditions
		if iteration > maxIterations {
			a.logger.Info("max iterations reached", "iterations", maxIterations)
			return nil
		}

		if a.loopCtx.Err() != nil {
			a.logger.Info("loop cancelled")
			return a.loopCtx.Err()
		}

		// Check circuit breaker
		if a.circuit != nil && a.circuit.IsOpen() {
			a.logger.Warn("circuit breaker open, stopping loop")
			return fmt.Errorf("circuit breaker open")
		}

		// Check rate limiter
		if a.rateLimiter != nil {
			if err := a.rateLimiter.Wait(a.loopCtx); err != nil {
				return fmt.Errorf("rate limit: %w", err)
			}
		}

		// Create iteration context with timeout
		iterCtx, iterCancel := context.WithTimeout(a.loopCtx, iterationTimeout)

		// Run pre-iteration hooks
		execCtx := a.createExecutionContext()
		hookCtx := &sdk.HookContext{
			Type:      sdk.HookTypePreIteration,
			Iteration: iteration,
			Task:      task,
			ExecCtx:   execCtx,
		}
		if err := a.hooks.Run(iterCtx, hookCtx); err != nil {
			iterCancel()
			return fmt.Errorf("pre-iteration hook: %w", err)
		}

		// Execute task
		a.logger.Info("starting iteration", "iteration", iteration)
		result, err := a.executeTask(iterCtx, execCtx, task)

		// Run post-iteration hooks
		hookCtx.Type = sdk.HookTypePostIteration
		hookCtx.Result = result
		hookCtx.Error = err
		_ = a.hooks.Run(iterCtx, hookCtx) // Don't fail on post-iteration hook errors

		iterCancel()

		if err != nil {
			a.logger.Error("iteration failed", "iteration", iteration, "error", err)
			if a.circuit != nil {
				a.circuit.RecordError(err)
			}

			// Run error hooks
			hookCtx.Type = sdk.HookTypeOnError
			_ = a.hooks.Run(a.loopCtx, hookCtx)

			// Check if we should continue
			if a.circuit != nil && a.circuit.IsOpen() {
				return fmt.Errorf("circuit breaker opened after error: %w", err)
			}

			// Continue with cooldown
			time.Sleep(cooldown)
			continue
		}

		// Update circuit breaker with success
		if a.circuit != nil {
			a.circuit.RecordSuccess(len(result.Changes))
		}

		// Check for exit signal
		if result.ExitSignal {
			a.logger.Info("exit signal received", "iteration", iteration)
			if a.checkExitConditions(result) {
				// Run exit hooks
				hookCtx.Type = sdk.HookTypeOnExit
				_ = a.hooks.Run(a.loopCtx, hookCtx)
				return nil
			}
		}

		// Cooldown between iterations
		time.Sleep(cooldown)
	}
}

// checkExitConditions implements dual-condition exit detection.
func (a *Agent) checkExitConditions(result *sdk.Result) bool {
	if !a.config.Exit.RequireExplicitSignal && result.ExitSignal {
		return true
	}

	// Count completion indicators
	indicators := 0

	// Indicator: Exit signal set
	if result.ExitSignal {
		indicators++
	}

	// Indicator: Summary file exists (check artifacts)
	if _, ok := result.Artifacts["summary"]; ok {
		indicators++
	}

	// Indicator: No more next tasks
	if len(result.NextTasks) == 0 {
		indicators++
	}

	// Indicator: Status is success with no changes
	if result.Status == sdk.ResultStatusSuccess && len(result.Changes) == 0 {
		indicators++
	}

	// Check threshold
	threshold := a.config.Exit.CompletionThreshold
	if threshold == 0 {
		threshold = 2 // Default
	}

	return indicators >= threshold && result.ExitSignal
}

// defaultConfig returns default configuration.
func defaultConfig() *sdk.Config {
	return &sdk.Config{
		Project: sdk.ProjectConfig{
			RootDir:        ".",
			IgnorePatterns: []string{"vendor/", "node_modules/", ".git/"},
			IndexPatterns:  []string{"*.go", "*.ts", "*.py", "*.js"},
		},
		Models: sdk.ModelConfig{
			Planning:   "claude-sonnet-4-20250514",
			Execution:  "claude-sonnet-4-20250514",
			Validation: "claude-sonnet-4-20250514",
		},
		Loop: sdk.LoopConfig{
			MaxIterations:        100,
			RateLimitPerHour:     100,
			IterationTimeout:     "15m",
			Cooldown:             "5s",
			MaxValidationRetries: 5,
			ParallelSteps:        true,
		},
		Exit: sdk.ExitConfig{
			RequireExplicitSignal:    true,
			CompletionThreshold:      2,
			MaxConsecutiveNoProgress: 3,
			MaxConsecutiveErrors:     5,
		},
		Circuit: sdk.CircuitConfig{
			NoProgressThreshold:    3,
			SameErrorThreshold:     5,
			OutputDeclineThreshold: 70,
			RecoveryTimeout:        "5m",
		},
		Monitor: sdk.MonitorConfig{
			Enabled: false,
			Port:    8080,
		},
	}
}

// parseDuration parses a duration string with a default fallback.
func parseDuration(s string, def time.Duration) time.Duration {
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return def
	}
	return d
}

// Monitor interface for live monitoring.
type Monitor interface {
	Start(ctx context.Context) error
	Stop() error
	Emit(event Event)
}

// Event represents a monitoring event.
type Event struct {
	Type      string
	Timestamp time.Time
	Data      map[string]any
}

// Stats returns current agent statistics.
func (a *Agent) Stats() AgentStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var duration time.Duration
	if !a.startTime.IsZero() {
		duration = time.Since(a.startTime)
	}

	return AgentStats{
		Iteration:   a.iteration,
		Running:     a.running,
		Duration:    duration,
		TotalTokens: a.totalTokens,
		SkillCount:  a.registry.Count(),
	}
}

// AgentStats contains agent runtime statistics.
type AgentStats struct {
	Iteration   int
	Running     bool
	Duration    time.Duration
	TotalTokens int
	SkillCount  int
}

// SetLogger sets the agent's logger.
func (a *Agent) SetLogger(logger *slog.Logger) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.logger = logger
}

// WorkDir returns the current working directory.
func (a *Agent) WorkDir() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.workDir
}

// Config returns the agent's configuration.
func (a *Agent) Config() *sdk.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.config
}

// Close releases agent resources.
func (a *Agent) Close() error {
	a.Stop()
	return nil
}

// init sets default filesystem if not provided
func init() {
	// Ensure os.DirFS is used by default
	_ = os.DirFS(".")
}
