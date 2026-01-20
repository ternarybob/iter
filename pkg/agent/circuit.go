package agent

import (
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state.
type CircuitState int

const (
	// CircuitStateClosed means the circuit is healthy.
	CircuitStateClosed CircuitState = iota
	// CircuitStateOpen means the circuit is tripped.
	CircuitStateOpen
	// CircuitStateHalfOpen means the circuit is testing recovery.
	CircuitStateHalfOpen
)

// CircuitBreakerConfig configures the circuit breaker.
type CircuitBreakerConfig struct {
	// NoProgressThreshold is loops without file changes before tripping.
	NoProgressThreshold int

	// SameErrorThreshold is repeated errors before tripping.
	SameErrorThreshold int

	// OutputDeclineThreshold is output decline percentage before tripping.
	OutputDeclineThreshold int

	// RecoveryTimeout is time before half-open state.
	RecoveryTimeout time.Duration
}

// CircuitBreaker implements the circuit breaker pattern.
type CircuitBreaker struct {
	mu     sync.RWMutex
	config CircuitBreakerConfig

	state        CircuitState
	lastError    error
	errorCount   int
	noProgress   int
	lastOpenTime time.Time

	// Metrics
	successCount   int
	failureCount   int
	lastChangeSize int
}

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	// Set defaults
	if config.NoProgressThreshold == 0 {
		config.NoProgressThreshold = 3
	}
	if config.SameErrorThreshold == 0 {
		config.SameErrorThreshold = 5
	}
	if config.OutputDeclineThreshold == 0 {
		config.OutputDeclineThreshold = 70
	}
	if config.RecoveryTimeout == 0 {
		config.RecoveryTimeout = 5 * time.Minute
	}

	return &CircuitBreaker{
		config: config,
		state:  CircuitStateClosed,
	}
}

// State returns the current circuit state.
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// IsOpen returns true if the circuit is open (blocking).
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == CircuitStateClosed {
		return false
	}

	if cb.state == CircuitStateOpen {
		// Check if we should transition to half-open
		if time.Since(cb.lastOpenTime) >= cb.config.RecoveryTimeout {
			cb.state = CircuitStateHalfOpen
			return false
		}
		return true
	}

	// Half-open allows one request through
	return false
}

// RecordSuccess records a successful operation.
func (cb *CircuitBreaker) RecordSuccess(changeSize int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.successCount++

	if cb.state == CircuitStateHalfOpen {
		// Successful recovery
		cb.state = CircuitStateClosed
		cb.errorCount = 0
		cb.noProgress = 0
		cb.lastError = nil
	}

	// Track progress
	if changeSize == 0 {
		cb.noProgress++
		if cb.noProgress >= cb.config.NoProgressThreshold {
			cb.tripOpen("no progress detected")
		}
	} else {
		cb.noProgress = 0
	}

	// Track output decline
	if cb.lastChangeSize > 0 && changeSize > 0 {
		decline := 100 - (changeSize * 100 / cb.lastChangeSize)
		if decline >= cb.config.OutputDeclineThreshold {
			cb.tripOpen("output declining")
		}
	}

	cb.lastChangeSize = changeSize
}

// RecordError records a failed operation.
func (cb *CircuitBreaker) RecordError(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failureCount++

	if cb.state == CircuitStateHalfOpen {
		// Failed recovery, go back to open
		cb.tripOpen("failed recovery")
		return
	}

	// Check for same error
	if cb.lastError != nil && err != nil && cb.lastError.Error() == err.Error() {
		cb.errorCount++
		if cb.errorCount >= cb.config.SameErrorThreshold {
			cb.tripOpen("repeated error")
		}
	} else {
		cb.errorCount = 1
	}

	cb.lastError = err
}

// tripOpen transitions the circuit to open state.
func (cb *CircuitBreaker) tripOpen(reason string) {
	cb.state = CircuitStateOpen
	cb.lastOpenTime = time.Now()
	_ = reason // Could log this
}

// Reset manually resets the circuit breaker.
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.state = CircuitStateClosed
	cb.errorCount = 0
	cb.noProgress = 0
	cb.lastError = nil
}

// Stats returns circuit breaker statistics.
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return CircuitBreakerStats{
		State:        cb.state,
		SuccessCount: cb.successCount,
		FailureCount: cb.failureCount,
		ErrorCount:   cb.errorCount,
		NoProgress:   cb.noProgress,
	}
}

// CircuitBreakerStats contains circuit breaker statistics.
type CircuitBreakerStats struct {
	State        CircuitState
	SuccessCount int
	FailureCount int
	ErrorCount   int
	NoProgress   int
}

// String returns a string representation of the circuit state.
func (s CircuitState) String() string {
	switch s {
	case CircuitStateClosed:
		return "closed"
	case CircuitStateOpen:
		return "open"
	case CircuitStateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
