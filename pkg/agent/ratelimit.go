package agent

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu sync.Mutex

	// Configuration
	capacity   float64       // Max tokens
	refillRate float64       // Tokens per second
	interval   time.Duration // Minimum interval between requests

	// State
	tokens    float64
	lastTime  time.Time
	waitCount int
}

// NewRateLimiter creates a new rate limiter with the given hourly limit.
func NewRateLimiter(perHour int) *RateLimiter {
	if perHour <= 0 {
		perHour = 100 // Default
	}

	capacity := float64(perHour) / 10 // Allow some burst
	if capacity < 1 {
		capacity = 1
	}

	return &RateLimiter{
		capacity:   capacity,
		refillRate: float64(perHour) / 3600.0, // tokens per second
		interval:   time.Second,               // Minimum 1 second between requests
		tokens:     capacity,
		lastTime:   time.Now(),
	}
}

// Allow checks if a request can proceed immediately.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// Wait blocks until a request can proceed or context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()

		if rl.tokens >= 1 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}

		// Calculate wait time
		deficit := 1 - rl.tokens
		waitDuration := time.Duration(deficit/rl.refillRate*1000) * time.Millisecond
		if waitDuration < rl.interval {
			waitDuration = rl.interval
		}

		rl.waitCount++
		rl.mu.Unlock()

		// Wait with context
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			// Continue loop to try again
		}
	}
}

// Reserve reserves a token and returns how long to wait.
func (rl *RateLimiter) Reserve() time.Duration {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= 1 {
		rl.tokens--
		return 0
	}

	// Calculate wait time
	deficit := 1 - rl.tokens
	waitDuration := time.Duration(deficit/rl.refillRate*1000) * time.Millisecond

	// Pre-decrement
	rl.tokens--

	return waitDuration
}

// refill adds tokens based on elapsed time.
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()

	if elapsed > 0 {
		rl.tokens += elapsed * rl.refillRate
		if rl.tokens > rl.capacity {
			rl.tokens = rl.capacity
		}
		rl.lastTime = now
	}
}

// Tokens returns the current number of available tokens.
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}

// Stats returns rate limiter statistics.
func (rl *RateLimiter) Stats() RateLimiterStats {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()

	return RateLimiterStats{
		Tokens:     rl.tokens,
		Capacity:   rl.capacity,
		RefillRate: rl.refillRate,
		WaitCount:  rl.waitCount,
	}
}

// RateLimiterStats contains rate limiter statistics.
type RateLimiterStats struct {
	Tokens     float64
	Capacity   float64
	RefillRate float64
	WaitCount  int
}

// Reset restores the rate limiter to full capacity.
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.tokens = rl.capacity
	rl.lastTime = time.Now()
}

// SetRate changes the rate limit.
func (rl *RateLimiter) SetRate(perHour int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if perHour <= 0 {
		perHour = 100
	}

	rl.refillRate = float64(perHour) / 3600.0
	rl.capacity = float64(perHour) / 10
	if rl.capacity < 1 {
		rl.capacity = 1
	}
	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}
}
