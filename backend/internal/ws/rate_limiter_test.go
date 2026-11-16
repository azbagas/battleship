package ws

import (
	"testing"
	"time"
)

func TestRateLimiter(t *testing.T) {
	// 5 tokens per second, burst 2
	limiter := NewRateLimiter(5.0, 2.0)

	// First 2 should succeed immediately from burst capacity
	if !limiter.Allow() {
		t.Fatal("expected 1st allow to succeed")
	}
	if !limiter.Allow() {
		t.Fatal("expected 2nd allow to succeed")
	}

	// 3rd immediate call should be rejected
	if limiter.Allow() {
		t.Fatal("expected 3rd immediate allow to be rejected")
	}

	// Wait 250ms -> 0.25s * 5 = 1.25 tokens replenished
	time.Sleep(250 * time.Millisecond)
	if !limiter.Allow() {
		t.Fatal("expected allow after replenishment to succeed")
	}

	// Immediate follow-up should fail
	if limiter.Allow() {
		t.Fatal("expected immediate allow after single token use to fail")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	// Rate <= 0 disables rate limiting
	limiter := NewRateLimiter(0, 0)
	for i := 0; i < 100; i++ {
		if !limiter.Allow() {
			t.Fatalf("expected rate limiter with rate 0 to always allow, failed at %d", i)
		}
	}
}

