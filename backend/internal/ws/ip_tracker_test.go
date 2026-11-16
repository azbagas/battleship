package ws

import (
	"net/http"
	"testing"
)

func TestIPTracker(t *testing.T) {
	tracker := NewIPTracker(2)

	ip := "192.168.1.100"

	if !tracker.Acquire(ip) {
		t.Fatal("expected 1st acquire to succeed")
	}
	if tracker.Count(ip) != 1 {
		t.Fatalf("expected count 1, got %d", tracker.Count(ip))
	}

	if !tracker.Acquire(ip) {
		t.Fatal("expected 2nd acquire to succeed")
	}
	if tracker.Count(ip) != 2 {
		t.Fatalf("expected count 2, got %d", tracker.Count(ip))
	}

	// 3rd should be rejected
	if tracker.Acquire(ip) {
		t.Fatal("expected 3rd acquire to fail when limit is 2")
	}

	// Different IP should succeed
	otherIP := "192.168.1.101"
	if !tracker.Acquire(otherIP) {
		t.Fatal("expected acquire on separate IP to succeed")
	}

	// Release one from original IP
	tracker.Release(ip)
	if tracker.Count(ip) != 1 {
		t.Fatalf("expected count 1 after release, got %d", tracker.Count(ip))
	}

	// Now can acquire again
	if !tracker.Acquire(ip) {
		t.Fatal("expected acquire to succeed after release")
	}

	// Release both
	tracker.Release(ip)
	tracker.Release(ip)
	if tracker.Count(ip) != 0 {
		t.Fatalf("expected count 0 after releasing all, got %d", tracker.Count(ip))
	}
}

func TestExtractIP(t *testing.T) {
	// 1. From RemoteAddr
	req1, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	req1.RemoteAddr = "10.0.0.1:54321"
	if ip := ExtractIP(req1); ip != "10.0.0.1" {
		t.Errorf("expected 10.0.0.1, got %q", ip)
	}

	// 2. From X-Forwarded-For
	req2, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	req2.RemoteAddr = "127.0.0.1:54321"
	req2.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18")
	if ip := ExtractIP(req2); ip != "203.0.113.195" {
		t.Errorf("expected 203.0.113.195, got %q", ip)
	}

	// 3. From X-Real-IP
	req3, _ := http.NewRequest(http.MethodGet, "/ws", nil)
	req3.RemoteAddr = "127.0.0.1:54321"
	req3.Header.Set("X-Real-IP", "198.51.100.4")
	if ip := ExtractIP(req3); ip != "198.51.100.4" {
		t.Errorf("expected 198.51.100.4, got %q", ip)
	}
}

