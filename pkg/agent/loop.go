package agent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ternarybob/iter/pkg/sdk"
)

// LoopController manages the autonomous iteration loop.
type LoopController struct {
	mu sync.RWMutex

	agent   *Agent
	state   *LoopState
	exit    *ExitDetector
	metrics *LoopMetrics

	// Control
	ctx    context.Context
	cancel context.CancelFunc
}

// NewLoopController creates a new loop controller.
func NewLoopController(agent *Agent) *LoopController {
	return &LoopController{
		agent:   agent,
		state:   NewLoopState(),
		exit:    NewExitDetector(agent.config.Exit),
		metrics: NewLoopMetrics(),
	}
}

// Run executes the autonomous loop.
func (lc *LoopController) Run(ctx context.Context, task *sdk.Task) error {
	lc.mu.Lock()
	if lc.state.Phase != LoopPhaseIdle {
		lc.mu.Unlock()
		return fmt.Errorf("loop already running")
	}
	lc.ctx, lc.cancel = context.WithCancel(ctx)
	lc.state.Transition(LoopPhasePlanning)
	lc.mu.Unlock()

	defer func() {
		lc.mu.Lock()
		lc.state.Transition(LoopPhaseIdle)
		lc.mu.Unlock()
	}()

	config := lc.agent.config.Loop
	maxIterations := config.MaxIterations
	if maxIterations == 0 {
		maxIterations = 1000
	}

	iterationTimeout := parseDuration(config.IterationTimeout, 15*time.Minute)
	cooldown := parseDuration(config.Cooldown, 5*time.Second)

	for iteration := 1; iteration <= maxIterations; iteration++ {
		// Check cancellation
		if lc.ctx.Err() != nil {
			return lc.ctx.Err()
		}

		// Check circuit breaker
		if lc.agent.circuit != nil && lc.agent.circuit.IsOpen() {
			return fmt.Errorf("circuit breaker open")
		}

		// Rate limiting
		if lc.agent.rateLimiter != nil {
			if err := lc.agent.rateLimiter.Wait(lc.ctx); err != nil {
				return err
			}
		}

		// Run iteration
		result, err := lc.runIteration(iteration, task, iterationTimeout)
		if err != nil {
			lc.metrics.RecordFailure()
			if lc.agent.circuit != nil {
				lc.agent.circuit.RecordError(err)
			}

			// Check if we should continue
			if lc.agent.circuit != nil && lc.agent.circuit.IsOpen() {
				return fmt.Errorf("circuit breaker opened: %w", err)
			}

			time.Sleep(cooldown)
			continue
		}

		lc.metrics.RecordSuccess(result)
		if lc.agent.circuit != nil {
			lc.agent.circuit.RecordSuccess(len(result.Changes))
		}

		// Check exit
		if lc.exit.ShouldExit(result, lc.state) {
			lc.state.Transition(LoopPhaseComplete)
			return nil
		}

		time.Sleep(cooldown)
	}

	return fmt.Errorf("max iterations (%d) reached", maxIterations)
}

// runIteration executes a single iteration.
func (lc *LoopController) runIteration(iteration int, task *sdk.Task, timeout time.Duration) (*sdk.Result, error) {
	iterCtx, cancel := context.WithTimeout(lc.ctx, timeout)
	defer cancel()

	lc.mu.Lock()
	lc.agent.iteration = iteration
	lc.mu.Unlock()

	execCtx := lc.agent.createExecutionContext()

	// Run hooks
	hookCtx := &sdk.HookContext{
		Type:      sdk.HookTypePreIteration,
		Iteration: iteration,
		Task:      task,
		ExecCtx:   execCtx,
	}
	if err := lc.agent.hooks.Run(iterCtx, hookCtx); err != nil {
		return nil, err
	}

	// Execute
	result, err := lc.agent.executeTask(iterCtx, execCtx, task)

	// Post-iteration hooks
	hookCtx.Type = sdk.HookTypePostIteration
	hookCtx.Result = result
	hookCtx.Error = err
	_ = lc.agent.hooks.Run(iterCtx, hookCtx)

	return result, err
}

// Stop signals the loop to stop.
func (lc *LoopController) Stop() {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	if lc.cancel != nil {
		lc.cancel()
	}
}

// State returns the current loop state.
func (lc *LoopController) State() *LoopState {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return lc.state.Clone()
}

// Metrics returns loop metrics.
func (lc *LoopController) Metrics() LoopMetrics {
	lc.mu.RLock()
	defer lc.mu.RUnlock()
	return *lc.metrics
}

// LoopMetrics tracks loop statistics.
type LoopMetrics struct {
	mu sync.Mutex

	TotalIterations int
	SuccessCount    int
	FailureCount    int
	TotalChanges    int
	TotalTokens     int
	StartTime       time.Time
	LastSuccess     time.Time
}

// NewLoopMetrics creates new metrics.
func NewLoopMetrics() *LoopMetrics {
	return &LoopMetrics{
		StartTime: time.Now(),
	}
}

// RecordSuccess records a successful iteration.
func (m *LoopMetrics) RecordSuccess(result *sdk.Result) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalIterations++
	m.SuccessCount++
	m.TotalChanges += len(result.Changes)
	m.TotalTokens += result.Metrics.TokensUsed
	m.LastSuccess = time.Now()
}

// RecordFailure records a failed iteration.
func (m *LoopMetrics) RecordFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalIterations++
	m.FailureCount++
}

// Duration returns elapsed time.
func (m *LoopMetrics) Duration() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return time.Since(m.StartTime)
}
