package agent

import (
	"sync"
	"time"
)

// LoopPhase represents the current phase of the autonomous loop.
type LoopPhase int

const (
	// LoopPhaseIdle means the loop is not running.
	LoopPhaseIdle LoopPhase = iota
	// LoopPhasePlanning means the architect is planning.
	LoopPhasePlanning
	// LoopPhaseExecuting means the worker is implementing.
	LoopPhaseExecuting
	// LoopPhaseValidating means the validator is reviewing.
	LoopPhaseValidating
	// LoopPhaseFinalValidating means final validation is in progress.
	LoopPhaseFinalValidating
	// LoopPhaseComplete means the loop has finished successfully.
	LoopPhaseComplete
	// LoopPhaseFailed means the loop has failed.
	LoopPhaseFailed
)

// String returns the string representation of a loop phase.
func (p LoopPhase) String() string {
	switch p {
	case LoopPhaseIdle:
		return "idle"
	case LoopPhasePlanning:
		return "planning"
	case LoopPhaseExecuting:
		return "executing"
	case LoopPhaseValidating:
		return "validating"
	case LoopPhaseFinalValidating:
		return "final_validating"
	case LoopPhaseComplete:
		return "complete"
	case LoopPhaseFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// LoopState tracks the state of the autonomous loop.
type LoopState struct {
	mu sync.RWMutex

	// Current phase
	Phase LoopPhase

	// Iteration tracking
	Iteration     int
	StepNumber    int
	StepIteration int // Retry count within a step

	// Progress tracking
	TotalSteps     int
	CompletedSteps int

	// Timing
	PhaseStartTime time.Time
	IterationStart time.Time
	LastTransition time.Time

	// Error tracking
	LastError          error
	ConsecutiveErrors  int
	ConsecutiveNoChange int

	// Exit tracking
	ExitIndicators int
	HasExitSignal  bool
	HasSummary     bool

	// History
	PhaseHistory []PhaseTransition
}

// PhaseTransition records a state change.
type PhaseTransition struct {
	From      LoopPhase
	To        LoopPhase
	Timestamp time.Time
	Reason    string
}

// NewLoopState creates a new loop state.
func NewLoopState() *LoopState {
	return &LoopState{
		Phase:          LoopPhaseIdle,
		LastTransition: time.Now(),
	}
}

// Transition changes the loop phase.
func (s *LoopState) Transition(phase LoopPhase) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Phase == phase {
		return
	}

	now := time.Now()
	s.PhaseHistory = append(s.PhaseHistory, PhaseTransition{
		From:      s.Phase,
		To:        phase,
		Timestamp: now,
	})

	s.Phase = phase
	s.PhaseStartTime = now
	s.LastTransition = now
}

// TransitionWithReason changes the loop phase with a reason.
func (s *LoopState) TransitionWithReason(phase LoopPhase, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Phase == phase {
		return
	}

	now := time.Now()
	s.PhaseHistory = append(s.PhaseHistory, PhaseTransition{
		From:      s.Phase,
		To:        phase,
		Timestamp: now,
		Reason:    reason,
	})

	s.Phase = phase
	s.PhaseStartTime = now
	s.LastTransition = now
}

// IncrementIteration moves to the next iteration.
func (s *LoopState) IncrementIteration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Iteration++
	s.IterationStart = time.Now()
}

// SetStep sets the current step.
func (s *LoopState) SetStep(stepNumber, totalSteps int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StepNumber = stepNumber
	s.TotalSteps = totalSteps
	s.StepIteration = 0
}

// IncrementStepIteration increments the step retry count.
func (s *LoopState) IncrementStepIteration() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StepIteration++
}

// CompleteStep marks a step as completed.
func (s *LoopState) CompleteStep() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CompletedSteps++
}

// RecordError records an error.
func (s *LoopState) RecordError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
	s.ConsecutiveErrors++
}

// ClearError clears error tracking.
func (s *LoopState) ClearError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = nil
	s.ConsecutiveErrors = 0
}

// RecordProgress records that progress was made.
func (s *LoopState) RecordProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConsecutiveNoChange = 0
}

// RecordNoProgress records that no progress was made.
func (s *LoopState) RecordNoProgress() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConsecutiveNoChange++
}

// SetExitIndicators sets the exit indicator count.
func (s *LoopState) SetExitIndicators(count int, hasSignal, hasSummary bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ExitIndicators = count
	s.HasExitSignal = hasSignal
	s.HasSummary = hasSummary
}

// Clone creates a copy of the state.
func (s *LoopState) Clone() *LoopState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	clone := &LoopState{
		Phase:               s.Phase,
		Iteration:           s.Iteration,
		StepNumber:          s.StepNumber,
		StepIteration:       s.StepIteration,
		TotalSteps:          s.TotalSteps,
		CompletedSteps:      s.CompletedSteps,
		PhaseStartTime:      s.PhaseStartTime,
		IterationStart:      s.IterationStart,
		LastTransition:      s.LastTransition,
		LastError:           s.LastError,
		ConsecutiveErrors:   s.ConsecutiveErrors,
		ConsecutiveNoChange: s.ConsecutiveNoChange,
		ExitIndicators:      s.ExitIndicators,
		HasExitSignal:       s.HasExitSignal,
		HasSummary:          s.HasSummary,
	}

	if s.PhaseHistory != nil {
		clone.PhaseHistory = make([]PhaseTransition, len(s.PhaseHistory))
		copy(clone.PhaseHistory, s.PhaseHistory)
	}

	return clone
}

// PhaseDuration returns how long the current phase has been running.
func (s *LoopState) PhaseDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.PhaseStartTime)
}

// IterationDuration returns how long the current iteration has been running.
func (s *LoopState) IterationDuration() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return time.Since(s.IterationStart)
}

// Progress returns completion percentage (0-100).
func (s *LoopState) Progress() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.TotalSteps == 0 {
		return 0
	}
	return s.CompletedSteps * 100 / s.TotalSteps
}

// IsTerminal returns true if the phase is a terminal state.
func (s *LoopState) IsTerminal() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Phase == LoopPhaseComplete || s.Phase == LoopPhaseFailed
}

// IsRunning returns true if the loop is actively running.
func (s *LoopState) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Phase != LoopPhaseIdle && !s.IsTerminal()
}
