package lobby

import (
	"sync"
	"time"
)

// Timer defines the interface for an active timer that can be stopped.
type Timer interface {
	Stop() bool
}

// Clock defines the interface for time queries and timer scheduling.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, f func()) Timer
}

// RealClock implements Clock using the standard library time package.
type RealClock struct{}

// Now returns the current UTC time.
func (RealClock) Now() time.Time {
	return time.Now().UTC()
}

// AfterFunc schedules f to run after duration d.
func (RealClock) AfterFunc(d time.Duration, f func()) Timer {
	return time.AfterFunc(d, f)
}

// MockClock provides a controllable, thread-safe Clock implementation for testing.
type MockClock struct {
	mu     sync.Mutex
	now    time.Time
	timers []*mockTimer
}

type mockTimer struct {
	clock   *MockClock
	trigger time.Time
	fn      func()
	stopped bool
}

func (t *mockTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	if t.stopped {
		return false
	}
	t.stopped = true
	return true
}

// NewMockClock creates a MockClock starting at the given time.
func NewMockClock(start time.Time) *MockClock {
	return &MockClock{
		now: start,
	}
}

// Now returns the mock clock's current simulated time.
func (m *MockClock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// AfterFunc registers a timer to fire when the simulated clock advances past duration d.
func (m *MockClock) AfterFunc(d time.Duration, f func()) Timer {
	m.mu.Lock()
	defer m.mu.Unlock()

	t := &mockTimer{
		clock:   m,
		trigger: m.now.Add(d),
		fn:      f,
		stopped: false,
	}
	m.timers = append(m.timers, t)
	return t
}

// Advance advances the simulated time by d and executes any timers that are due.
func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	m.now = m.now.Add(d)

	var toFire []func()
	var remaining []*mockTimer

	for _, t := range m.timers {
		if !t.stopped && !t.trigger.After(m.now) {
			t.stopped = true
			toFire = append(toFire, t.fn)
		} else if !t.stopped {
			remaining = append(remaining, t)
		}
	}
	m.timers = remaining
	m.mu.Unlock()

	for _, fn := range toFire {
		fn()
	}
}

