package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ternarybob/iter/pkg/sdk"
)

func TestExitDetector_ShouldExit_NoExit(t *testing.T) {
	config := sdk.ExitConfig{
		CompletionThreshold:       3,
		RequireExplicitSignal:     true,
		MaxConsecutiveNoProgress:  5,
		MaxConsecutiveErrors:      5,
	}
	ed := NewExitDetector(config)
	state := NewLoopState()

	result := &sdk.Result{
		Status:     sdk.ResultStatusSuccess,
		ExitSignal: false,
		NextTasks:  []*sdk.Task{sdk.NewTask("more work")},
	}

	shouldExit := ed.ShouldExit(result, state)
	assert.False(t, shouldExit, "should not exit with work remaining")
}

func TestExitDetector_ShouldExit_WithSignalAndIndicators(t *testing.T) {
	config := sdk.ExitConfig{
		CompletionThreshold:   2,
		RequireExplicitSignal: true,
	}
	ed := NewExitDetector(config)
	state := NewLoopState()

	result := &sdk.Result{
		Status:     sdk.ResultStatusSuccess,
		ExitSignal: true,
		NextTasks:  nil, // No more tasks - indicator
		Artifacts:  map[string]string{"summary": "done"}, // Summary exists - indicator
	}

	shouldExit := ed.ShouldExit(result, state)
	assert.True(t, shouldExit, "should exit with signal and indicators")
}

func TestExitDetector_ShouldExit_SignalOnly(t *testing.T) {
	config := sdk.ExitConfig{
		CompletionThreshold:   3,
		RequireExplicitSignal: true,
	}
	ed := NewExitDetector(config)
	state := NewLoopState()

	result := &sdk.Result{
		Status:     sdk.ResultStatusSuccess,
		ExitSignal: true,
		NextTasks:  []*sdk.Task{sdk.NewTask("more work")}, // Still has work
	}

	shouldExit := ed.ShouldExit(result, state)
	assert.False(t, shouldExit, "should not exit on signal alone when indicators below threshold")
}

func TestExitDetector_ShouldExit_ForceOnErrors(t *testing.T) {
	config := sdk.ExitConfig{
		MaxConsecutiveErrors: 3,
	}
	ed := NewExitDetector(config)
	state := NewLoopState()

	// Simulate consecutive errors
	state.ConsecutiveErrors = 5

	result := &sdk.Result{
		Status:     sdk.ResultStatusSuccess,
		ExitSignal: false,
	}

	shouldExit := ed.ShouldExit(result, state)
	assert.True(t, shouldExit, "should force exit on too many errors")
}

func TestExitDetector_ShouldExit_ForceOnNoProgress(t *testing.T) {
	config := sdk.ExitConfig{
		MaxConsecutiveNoProgress: 3,
	}
	ed := NewExitDetector(config)
	state := NewLoopState()

	// Simulate no progress
	state.ConsecutiveNoChange = 5

	result := &sdk.Result{
		Status:     sdk.ResultStatusSuccess,
		ExitSignal: false,
	}

	shouldExit := ed.ShouldExit(result, state)
	assert.True(t, shouldExit, "should force exit on no progress")
}

func TestExitDetector_Reset(t *testing.T) {
	config := sdk.ExitConfig{}
	ed := NewExitDetector(config)

	ed.RecordError()
	ed.RecordError()

	ed.Reset()

	// After reset, should start fresh
	assert.NotNil(t, ed)
}

func TestExitReason_String(t *testing.T) {
	tests := []struct {
		reason ExitReason
		want   string
	}{
		{ExitReasonNone, "none"},
		{ExitReasonComplete, "complete"},
		{ExitReasonMaxIterations, "max_iterations"},
		{ExitReasonCircuitBreaker, "circuit_breaker"},
		{ExitReasonStagnation, "stagnation"},
		{ExitReasonErrors, "errors"},
		{ExitReasonCancelled, "cancelled"},
		{ExitReasonRateLimit, "rate_limit"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.reason.String())
		})
	}
}
