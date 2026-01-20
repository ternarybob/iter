package agent

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_InitialState(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})

	assert.False(t, cb.IsOpen(), "circuit breaker should not be open initially")
	assert.Equal(t, CircuitStateClosed, cb.State(), "initial state should be closed")
}

func TestCircuitBreaker_OpensAfterRepeatedErrors(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		SameErrorThreshold: 3,
	})

	err := errors.New("same error")

	// Record failures up to threshold
	for i := 0; i < 5; i++ {
		cb.RecordError(err)
	}

	assert.True(t, cb.IsOpen(), "circuit breaker should be open after threshold errors")
	assert.Equal(t, CircuitStateOpen, cb.State(), "state should be open")
}

func TestCircuitBreaker_RecordSuccess(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 5,
	})

	// Record success with changes
	cb.RecordSuccess(100)

	assert.False(t, cb.IsOpen(), "circuit breaker should not be open after success")
	assert.Equal(t, CircuitStateClosed, cb.State(), "state should be closed")
}

func TestCircuitBreaker_NoProgressDetection(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		NoProgressThreshold: 3,
	})

	// Record successes with no changes
	for i := 0; i < 3; i++ {
		cb.RecordSuccess(0) // No changes
	}

	assert.True(t, cb.IsOpen(), "circuit breaker should open after no progress")
	assert.Equal(t, CircuitStateOpen, cb.State(), "state should be open")
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{
		SameErrorThreshold: 2,
	})

	// Open the circuit
	err := errors.New("test error")
	for i := 0; i < 5; i++ {
		cb.RecordError(err)
	}
	require.True(t, cb.IsOpen(), "circuit should be open")

	// Reset
	cb.Reset()

	assert.False(t, cb.IsOpen(), "circuit breaker should not be open after reset")
	assert.Equal(t, CircuitStateClosed, cb.State(), "state should be closed after reset")
}

func TestCircuitBreaker_Stats(t *testing.T) {
	cb := NewCircuitBreaker(CircuitBreakerConfig{})

	cb.RecordSuccess(10)
	cb.RecordSuccess(20)
	cb.RecordError(errors.New("err"))

	stats := cb.Stats()

	assert.Equal(t, 2, stats.SuccessCount, "should track successes")
	assert.Equal(t, 1, stats.FailureCount, "should track failures")
}

func TestCircuitBreaker_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		config      CircuitBreakerConfig
		actions     []func(*CircuitBreaker)
		wantOpen    bool
		wantState   CircuitState
	}{
		{
			name:      "no actions",
			config:    CircuitBreakerConfig{},
			actions:   []func(*CircuitBreaker){},
			wantOpen:  false,
			wantState: CircuitStateClosed,
		},
		{
			name: "below error threshold",
			config: CircuitBreakerConfig{
				SameErrorThreshold: 5,
			},
			actions: []func(*CircuitBreaker){
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("err")) },
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("err")) },
			},
			wantOpen:  false,
			wantState: CircuitStateClosed,
		},
		{
			name: "at error threshold",
			config: CircuitBreakerConfig{
				SameErrorThreshold: 3,
			},
			actions: []func(*CircuitBreaker){
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("same")) },
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("same")) },
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("same")) },
			},
			wantOpen:  true,
			wantState: CircuitStateOpen,
		},
		{
			name: "different errors don't accumulate",
			config: CircuitBreakerConfig{
				SameErrorThreshold: 2,
			},
			actions: []func(*CircuitBreaker){
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("error1")) },
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("error2")) },
				func(cb *CircuitBreaker) { cb.RecordError(errors.New("error3")) },
			},
			wantOpen:  false,
			wantState: CircuitStateClosed,
		},
		{
			name: "no progress threshold",
			config: CircuitBreakerConfig{
				NoProgressThreshold: 2,
			},
			actions: []func(*CircuitBreaker){
				func(cb *CircuitBreaker) { cb.RecordSuccess(0) },
				func(cb *CircuitBreaker) { cb.RecordSuccess(0) },
			},
			wantOpen:  true,
			wantState: CircuitStateOpen,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := NewCircuitBreaker(tt.config)

			for _, action := range tt.actions {
				action(cb)
			}

			assert.Equal(t, tt.wantOpen, cb.IsOpen(), "IsOpen() mismatch")
			assert.Equal(t, tt.wantState, cb.State(), "State() mismatch")
		})
	}
}

func TestCircuitState_String(t *testing.T) {
	assert.Equal(t, "closed", CircuitStateClosed.String())
	assert.Equal(t, "open", CircuitStateOpen.String())
	assert.Equal(t, "half-open", CircuitStateHalfOpen.String())
}
