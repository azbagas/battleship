package ws

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
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

// ExtractIP extracts the client IP address from the HTTP request's RemoteAddr.
// Proxy headers are ignored by default; use ExtractClientIP(r, true) when behind a trusted proxy.
func ExtractIP(r *http.Request) string {
	return ExtractClientIP(r, false)
}

// ExtractClientIP extracts the client IP address from an HTTP request.
// When trustProxy is true, X-Real-IP and X-Forwarded-For are inspected (and validated
// as IP addresses) before falling back to RemoteAddr. When trustProxy is false,
// forwarded headers are ignored to prevent IP spoofing.
func ExtractClientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if xrip := strings.TrimSpace(r.Header.Get("X-Real-IP")); xrip != "" {
			if ip := normalizeValidIP(xrip); ip != "" {
				return ip
			}
		}

		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				if ip := normalizeValidIP(strings.TrimSpace(parts[0])); ip != "" {
					return ip
				}
			}
		}
	}

	return normalizeIP(r.RemoteAddr)
}

// normalizeValidIP strips any port and returns the IP only if it parses as a valid IPv4/IPv6 address.
func normalizeValidIP(addr string) string {
	ip := normalizeIP(addr)
	if parsed := net.ParseIP(ip); parsed != nil {
		return parsed.String()
	}
	return ""
}

// normalizeIP strips the port from an IP:port address if present.
func normalizeIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err == nil && host != "" {
		return host
	}
	return addr
}

const (
	handshakeBucketIdleTTL = 2 * time.Minute
	maxHandshakeBuckets    = 4096
)

type ipHandshakeEntry struct {
	limiter  *RateLimiter
	lastSeen time.Time
}

// HandshakeLimiter enforces a per-IP token bucket rate limit on WebSocket upgrade requests
// and periodically prunes idle entries to keep memory bounded.
type HandshakeLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	entries map[string]*ipHandshakeEntry
}

// NewHandshakeLimiter constructs a per-IP HandshakeLimiter.
// A rate <= 0 disables handshake rate limiting.
func NewHandshakeLimiter(rate, burst float64) *HandshakeLimiter {
	if burst <= 0 {
		burst = 1
	}
	return &HandshakeLimiter{
		rate:    rate,
		burst:   burst,
		entries: make(map[string]*ipHandshakeEntry),
	}
}

// Allow reports whether a WebSocket upgrade from ip is permitted now.
func (l *HandshakeLimiter) Allow(ip string) bool {
	if l == nil || l.rate <= 0 {
		return true
	}

	now := time.Now()

	l.mu.Lock()
	entry, exists := l.entries[ip]
	if !exists {
		if len(l.entries) >= maxHandshakeBuckets {
			l.pruneLocked(now)
		}
		if len(l.entries) >= maxHandshakeBuckets {
			// Still at capacity after pruning stale entries; evict one entry to stay bounded.
			for k := range l.entries {
				delete(l.entries, k)
				break
			}
		}
		entry = &ipHandshakeEntry{
			limiter:  NewRateLimiter(l.rate, l.burst),
			lastSeen: now,
		}
		l.entries[ip] = entry
	} else {
		entry.lastSeen = now
	}
	limiter := entry.limiter
	l.mu.Unlock()

	return limiter.Allow()
}

func (l *HandshakeLimiter) pruneLocked(now time.Time) {
	cutoff := now.Add(-handshakeBucketIdleTTL)
	for ip, entry := range l.entries {
		if entry.lastSeen.Before(cutoff) {
			delete(l.entries, ip)
		}
	}
}

