package ws

import (
	"net"
	"net/http"
	"strings"
	"sync"
)

// IPTracker tracks and limits concurrent connections per IP address.
type IPTracker struct {
	mu       sync.Mutex
	counts   map[string]int
	maxPerIP int
}

// NewIPTracker constructs an IPTracker with the given max concurrent connection limit per IP.
// A limit <= 0 disables tracking and allows unlimited connections.
func NewIPTracker(maxPerIP int) *IPTracker {
	return &IPTracker{
		counts:   make(map[string]int),
		maxPerIP: maxPerIP,
	}
}

// Acquire increments the active connection count for the specified IP if below maxPerIP.
// Returns true if the connection is allowed, false if the cap has been reached.
func (t *IPTracker) Acquire(ip string) bool {
	if t == nil || t.maxPerIP <= 0 {
		return true
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.counts[ip] >= t.maxPerIP {
		return false
	}

	t.counts[ip]++
	return true
}

// Release decrements the active connection count for the specified IP.
func (t *IPTracker) Release(ip string) {
	if t == nil || t.maxPerIP <= 0 {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.counts[ip]--
	if t.counts[ip] <= 0 {
		delete(t.counts, ip)
	}
}

// Count returns the current active connection count for the given IP.
func (t *IPTracker) Count(ip string) int {
	if t == nil {
		return 0
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	return t.counts[ip]
}

// ExtractIP extracts the client IP address from an HTTP request.
// It checks X-Forwarded-For, X-Real-IP, and falls back to RemoteAddr.
func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return normalizeIP(ip)
			}
		}
	}

	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ip := strings.TrimSpace(xrip)
		if ip != "" {
			return normalizeIP(ip)
		}
	}

	return normalizeIP(r.RemoteAddr)
}

// normalizeIP strips the port from an IP:port address if present.
func normalizeIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil && host != "" {
		return host
	}
	return addr
}

