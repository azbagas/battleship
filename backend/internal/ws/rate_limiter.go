package ws

import (
	"sync"
	"time"
)

// RateLimiter implements a thread-safe token bucket rate limiter.
type RateLimiter struct {
	mu     sync.Mutex
	rate   float64 // tokens replenished per second
	burst  float64 // maximum capacity of bucket
	tokens float64
	last   time.Time
}

// NewRateLimiter creates a new token bucket rate limiter.
// A rate <= 0 disables rate limiting, causing Allow to always return true.
func NewRateLimiter(rate, burst float64) *RateLimiter {
	if burst <= 0 {
		burst = 1
	}
	return &RateLimiter{
		rate:   rate,
		burst:  burst,
		tokens: burst,
		last:   time.Now(),
	}
}

// Allow reports whether a single event is permitted to happen now.
func (r *RateLimiter) Allow() bool {
	if r == nil || r.rate <= 0 {
		return true
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.last).Seconds()
	r.last = now

	r.tokens += elapsed * r.rate
	if r.tokens > r.burst {
		r.tokens = r.burst
	}

	if r.tokens >= 1.0 {
		r.tokens -= 1.0
		return true
	}

	return false
}

