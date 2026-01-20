package agent

import (
	"github.com/ternarybob/iter/pkg/sdk"
)

// ExitDetector determines when the autonomous loop should exit.
type ExitDetector struct {
	config sdk.ExitConfig

	// Tracking
	consecutiveComplete int
	consecutiveNoChange int
	consecutiveErrors   int
}

// NewExitDetector creates a new exit detector.
func NewExitDetector(config sdk.ExitConfig) *ExitDetector {
	// Set defaults
	if config.CompletionThreshold == 0 {
		config.CompletionThreshold = 2
	}
	if config.MaxConsecutiveNoProgress == 0 {
		config.MaxConsecutiveNoProgress = 3
	}
	if config.MaxConsecutiveErrors == 0 {
		config.MaxConsecutiveErrors = 5
	}

	return &ExitDetector{
		config: config,
	}
}

// ShouldExit determines if the loop should exit based on the result and state.
// Implements dual-condition exit detection:
// 1. completion_indicators >= threshold
// 2. Explicit ExitSignal = true
// Both conditions must be true for normal exit.
func (ed *ExitDetector) ShouldExit(result *sdk.Result, state *LoopState) bool {
	// Always exit on certain conditions
	if ed.shouldForceExit(result, state) {
		return true
	}

	// Count completion indicators
	indicators := ed.countIndicators(result, state)

	// Update state
	state.SetExitIndicators(indicators, result.ExitSignal, ed.hasSummary(result))

	// Dual-condition check
	if ed.config.RequireExplicitSignal {
		// Both conditions required
		return indicators >= ed.config.CompletionThreshold && result.ExitSignal
	}

	// Only indicators required
	return indicators >= ed.config.CompletionThreshold
}

// countIndicators counts completion heuristics.
func (ed *ExitDetector) countIndicators(result *sdk.Result, state *LoopState) int {
	indicators := 0

	// Indicator 1: Exit signal set
	if result.ExitSignal {
		indicators++
	}

	// Indicator 2: Summary file exists
	if ed.hasSummary(result) {
		indicators++
	}

	// Indicator 3: No more next tasks
	if len(result.NextTasks) == 0 {
		indicators++
	}

	// Indicator 4: Success with no changes (work complete)
	if result.Status == sdk.ResultStatusSuccess && len(result.Changes) == 0 {
		ed.consecutiveNoChange++
		if ed.consecutiveNoChange >= 2 {
			indicators++
		}
	} else {
		ed.consecutiveNoChange = 0
	}

	// Indicator 5: Multiple consecutive "complete" messages
	if ed.isCompleteMessage(result.Message) {
		ed.consecutiveComplete++
		if ed.consecutiveComplete >= 2 {
			indicators++
		}
	} else {
		ed.consecutiveComplete = 0
	}

	// Indicator 6: All steps completed (if using orchestration)
	if state.TotalSteps > 0 && state.CompletedSteps >= state.TotalSteps {
		indicators++
	}

	return indicators
}

// shouldForceExit checks for forced exit conditions.
func (ed *ExitDetector) shouldForceExit(result *sdk.Result, state *LoopState) bool {
	// Force exit on too many consecutive errors
	if state.ConsecutiveErrors >= ed.config.MaxConsecutiveErrors {
		return true
	}

	// Force exit on stagnation
	if state.ConsecutiveNoChange >= ed.config.MaxConsecutiveNoProgress {
		return true
	}

	return false
}

// hasSummary checks if a summary artifact exists.
func (ed *ExitDetector) hasSummary(result *sdk.Result) bool {
	if result.Artifacts == nil {
		return false
	}
	_, ok := result.Artifacts["summary"]
	return ok
}

// isCompleteMessage checks if the message indicates completion.
func (ed *ExitDetector) isCompleteMessage(msg string) bool {
	completeKeywords := []string{
		"complete",
		"finished",
		"done",
		"all tasks",
		"no more",
		"nothing left",
		"successfully",
	}

	msgLower := toLower(msg)
	for _, keyword := range completeKeywords {
		if containsWord(msgLower, keyword) {
			return true
		}
	}
	return false
}

// toLower converts to lowercase (simple ASCII version).
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// containsWord checks if s contains word (simple version).
func containsWord(s, word string) bool {
	if len(word) == 0 {
		return true
	}
	if len(s) < len(word) {
		return false
	}
	for i := 0; i <= len(s)-len(word); i++ {
		if s[i:i+len(word)] == word {
			return true
		}
	}
	return false
}

// Reset resets the exit detector state.
func (ed *ExitDetector) Reset() {
	ed.consecutiveComplete = 0
	ed.consecutiveNoChange = 0
	ed.consecutiveErrors = 0
}

// RecordError records an error for tracking.
func (ed *ExitDetector) RecordError() {
	ed.consecutiveErrors++
}

// ClearErrors clears error tracking.
func (ed *ExitDetector) ClearErrors() {
	ed.consecutiveErrors = 0
}

// ExitReason describes why the loop is exiting.
type ExitReason int

const (
	// ExitReasonNone means no exit.
	ExitReasonNone ExitReason = iota
	// ExitReasonComplete means normal completion.
	ExitReasonComplete
	// ExitReasonMaxIterations means iteration limit reached.
	ExitReasonMaxIterations
	// ExitReasonCircuitBreaker means circuit breaker tripped.
	ExitReasonCircuitBreaker
	// ExitReasonStagnation means no progress detected.
	ExitReasonStagnation
	// ExitReasonErrors means too many errors.
	ExitReasonErrors
	// ExitReasonCancelled means context was cancelled.
	ExitReasonCancelled
	// ExitReasonRateLimit means rate limit exhausted.
	ExitReasonRateLimit
)

// String returns the string representation.
func (r ExitReason) String() string {
	switch r {
	case ExitReasonNone:
		return "none"
	case ExitReasonComplete:
		return "complete"
	case ExitReasonMaxIterations:
		return "max_iterations"
	case ExitReasonCircuitBreaker:
		return "circuit_breaker"
	case ExitReasonStagnation:
		return "stagnation"
	case ExitReasonErrors:
		return "errors"
	case ExitReasonCancelled:
		return "cancelled"
	case ExitReasonRateLimit:
		return "rate_limit"
	default:
		return "unknown"
	}
}
