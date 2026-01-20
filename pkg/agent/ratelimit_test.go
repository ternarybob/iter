package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimiter_InitialState(t *testing.T) {
	rl := NewRateLimiter(100) // 100 per hour

	assert.True(t, rl.Allow(), "should allow initial request")
}

func TestRateLimiter_AllowConsumes(t *testing.T) {
	rl := NewRateLimiter(3600) // 1 per second, capacity ~360

	// Should be able to make multiple requests initially
	for i := 0; i < 5; i++ {
		assert.True(t, rl.Allow(), "should allow request %d", i)
	}
}

func TestRateLimiter_Tokens(t *testing.T) {
	rl := NewRateLimiter(100)

	initial := rl.Tokens()
	assert.Greater(t, initial, 0.0, "should have tokens initially")

	rl.Allow() // consume one

	after := rl.Tokens()
	assert.Less(t, after, initial, "should have fewer tokens after Allow")
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(3600) // Fast refill rate

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should complete without error
	err := rl.Wait(ctx)
	assert.NoError(t, err, "wait should succeed")
}

func TestRateLimiter_Wait_ContextCancelled(t *testing.T) {
	rl := NewRateLimiter(1) // Very slow rate

	// Exhaust tokens
	for rl.Allow() {
		// drain
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := rl.Wait(ctx)
	assert.Error(t, err, "wait should return error when context cancelled")
}

func TestRateLimiter_Stats(t *testing.T) {
	rl := NewRateLimiter(100)

	stats := rl.Stats()

	assert.Greater(t, stats.Tokens, 0.0, "should have tokens")
	assert.Greater(t, stats.Capacity, 0.0, "should have capacity")
	assert.Greater(t, stats.RefillRate, 0.0, "should have refill rate")
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(100)

	// Consume tokens
	rl.Allow()
	rl.Allow()
	rl.Allow()

	tokensBefore := rl.Tokens()

	// Reset
	rl.Reset()

	tokensAfter := rl.Tokens()
	assert.GreaterOrEqual(t, tokensAfter, tokensBefore, "should have more tokens after reset")
}

func TestRateLimiter_Reserve(t *testing.T) {
	rl := NewRateLimiter(3600) // Fast rate

	// Reserve should return 0 if tokens available
	wait := rl.Reserve()
	assert.Equal(t, time.Duration(0), wait, "should not need to wait when tokens available")
}

func TestRateLimiter_SetRate(t *testing.T) {
	rl := NewRateLimiter(100)

	statsBefore := rl.Stats()

	rl.SetRate(200) // Double the rate

	statsAfter := rl.Stats()
	assert.Greater(t, statsAfter.RefillRate, statsBefore.RefillRate, "refill rate should increase")
}

func TestRateLimiter_TableDriven(t *testing.T) {
	tests := []struct {
		name       string
		perHour    int
		allowCount int
		wantAllow  bool
	}{
		{
			name:       "allows initial",
			perHour:    100,
			allowCount: 1,
			wantAllow:  true,
		},
		{
			name:       "allows multiple",
			perHour:    3600,
			allowCount: 5,
			wantAllow:  true,
		},
		{
			name:       "very low rate",
			perHour:    1,
			allowCount: 1,
			wantAllow:  true, // At least 1 token initially
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.perHour)

			var lastResult bool
			for i := 0; i < tt.allowCount; i++ {
				lastResult = rl.Allow()
			}

			assert.Equal(t, tt.wantAllow, lastResult, "Allow() mismatch")
		})
	}
}
