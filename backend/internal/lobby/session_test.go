package lobby

import (
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSessionRegistry_CreateAndGet(t *testing.T) {
	reg := NewSessionRegistry()

	player, err := reg.CreatePlayer("Admiral Yi")
	if err != nil {
		t.Fatalf("unexpected error creating player: %v", err)
	}

	if player.ID == "" {
		t.Errorf("expected non-empty player ID")
	}
	if player.SessionToken == "" {
		t.Errorf("expected non-empty session token")
	}
	if player.DisplayName != "Admiral Yi" {
		t.Errorf("expected display name 'Admiral Yi', got %q", player.DisplayName)
	}
	if player.ConnectionState != StateConnected {
		t.Errorf("expected state %s, got %s", StateConnected, player.ConnectionState)
	}

	// Lookup by token
	byToken, ok := reg.GetByToken(player.SessionToken)
	if !ok || byToken == nil {
		t.Fatalf("failed to find player by token")
	}
	if byToken.ID != player.ID {
		t.Errorf("token lookup ID mismatch: expected %s, got %s", player.ID, byToken.ID)
	}

	// Lookup by ID
	byID, ok := reg.GetByID(player.ID)
	if !ok || byID == nil {
		t.Fatalf("failed to find player by ID")
	}
	if byID.SessionToken != player.SessionToken {
		t.Errorf("ID lookup token mismatch: expected %s, got %s", player.SessionToken, byID.SessionToken)
	}

	// Unknown lookups
	if _, ok := reg.GetByToken("unknown-token"); ok {
		t.Errorf("expected false for unknown token")
	}
	if _, ok := reg.GetByID("p_nonexistent"); ok {
		t.Errorf("expected false for unknown ID")
	}
}

func TestSessionRegistry_CreateInvalidName(t *testing.T) {
	reg := NewSessionRegistry()

	_, err := reg.CreatePlayer("   ")
	if err == nil {
		t.Fatalf("expected error for empty/whitespace name, got nil")
	}
	if !errors.Is(err, ErrInvalidDisplayName) {
		t.Errorf("expected ErrInvalidDisplayName, got %v", err)
	}
}

func TestSessionRegistry_AttachDetach(t *testing.T) {
	reg := NewSessionRegistry()

	player, err := reg.CreatePlayer("Nelson")
	if err != nil {
		t.Fatalf("unexpected error creating player: %v", err)
	}

	// Attempting to attach while already connected returns ErrSessionAlreadyActive
	_, err = reg.AttachConnection(player.SessionToken)
	if !errors.Is(err, ErrSessionAlreadyActive) {
		t.Fatalf("expected ErrSessionAlreadyActive, got %v", err)
	}

	// Detach
	reg.DetachConnection(player.SessionToken)

	detached, ok := reg.GetByToken(player.SessionToken)
	if !ok {
		t.Fatalf("player not found after detach")
	}
	if detached.ConnectionState != StateDisconnected {
		t.Errorf("expected %s, got %s", StateDisconnected, detached.ConnectionState)
	}
	if detached.DisconnectedAt == nil {
		t.Errorf("expected DisconnectedAt to be non-nil after detach")
	}

	// Reattach
	reattached, err := reg.AttachConnection(player.SessionToken)
	if err != nil {
		t.Fatalf("unexpected error reattaching disconnected session: %v", err)
	}
	if reattached.ConnectionState != StateConnected {
		t.Errorf("expected %s, got %s", StateConnected, reattached.ConnectionState)
	}
	if reattached.DisconnectedAt != nil {
		t.Errorf("expected DisconnectedAt to be cleared on reattach")
	}

	// Attach with unknown token
	_, err = reg.AttachConnection("random-token")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected ErrSessionNotFound for unknown token, got %v", err)
	}
}

func TestSessionRegistry_UpdateDisplayName(t *testing.T) {
	reg := NewSessionRegistry()

	player, err := reg.CreatePlayer("Original")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Update with valid name
	if err := reg.UpdateDisplayName(player.SessionToken, "Updated Name"); err != nil {
		t.Fatalf("unexpected error updating name: %v", err)
	}

	updated, _ := reg.GetByToken(player.SessionToken)
	if updated.DisplayName != "Updated Name" {
		t.Errorf("expected 'Updated Name', got %q", updated.DisplayName)
	}

	// Update with invalid name fails and keeps existing
	if err := reg.UpdateDisplayName(player.SessionToken, "   "); err == nil {
		t.Fatalf("expected error updating with whitespace name")
	}

	current, _ := reg.GetByToken(player.SessionToken)
	if current.DisplayName != "Updated Name" {
		t.Errorf("expected 'Updated Name' retained, got %q", current.DisplayName)
	}
}

func TestSessionRegistry_Concurrency(t *testing.T) {
	reg := NewSessionRegistry()
	player, err := reg.CreatePlayer("ConcurrentTester")
	if err != nil {
		t.Fatalf("failed to create player: %v", err)
	}
	reg.DetachConnection(player.SessionToken)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				_ = reg.UpdateDisplayName(player.SessionToken, "NameCheck")
			} else {
				_, _ = reg.GetByToken(player.SessionToken)
			}
			_, _ = reg.GetByID(player.ID)
		}(i)
	}

	wg.Wait()
}

func TestSessionRegistry_PruneExpiredDisconnected(t *testing.T) {
	reg := NewSessionRegistry(WithIdleSessionTTL(0), WithMaxSessions(0))

	pIdleExpired, _ := reg.CreatePlayer("IdleExpired")
	pInRoom, _ := reg.CreatePlayer("InRoom")
	pConnected, _ := reg.CreatePlayer("StillOnline")

	_ = reg.SetCurrentRoom(pInRoom.ID, "r_active")
	reg.DetachConnection(pIdleExpired.SessionToken)
	reg.DetachConnection(pInRoom.SessionToken)

	// Force DisconnectedAt into the past for deterministic test
	past := time.Now().UTC().Add(-10 * time.Minute)
	reg.mu.Lock()
	reg.byID[pIdleExpired.ID].DisconnectedAt = &past
	reg.byID[pInRoom.ID].DisconnectedAt = &past
	reg.mu.Unlock()

	removed := reg.PruneExpiredDisconnected(time.Now().UTC().Add(-5 * time.Minute))
	if removed != 1 {
		t.Fatalf("expected 1 expired idle session pruned, got %d", removed)
	}

	if _, ok := reg.GetByToken(pIdleExpired.SessionToken); ok {
		t.Errorf("expected expired roomless session to be evicted")
	}
	if _, ok := reg.GetByToken(pInRoom.SessionToken); !ok {
		t.Errorf("expected disconnected session inside active room to be preserved")
	}
	if _, ok := reg.GetByToken(pConnected.SessionToken); !ok {
		t.Errorf("expected connected session to be preserved")
	}
}

func TestSessionRegistry_MaxSessionsEviction(t *testing.T) {
	reg := NewSessionRegistry(WithIdleSessionTTL(0), WithMaxSessions(3))

	p1, _ := reg.CreatePlayer("OldestIdle")
	p2, _ := reg.CreatePlayer("NewerIdle")
	p3, _ := reg.CreatePlayer("ActivePlayer")

	reg.DetachConnection(p1.SessionToken)
	reg.DetachConnection(p2.SessionToken)

	t1 := time.Now().UTC().Add(-2 * time.Minute)
	t2 := time.Now().UTC().Add(-1 * time.Minute)
	reg.mu.Lock()
	reg.byID[p1.ID].DisconnectedAt = &t1
	reg.byID[p2.ID].DisconnectedAt = &t2
	reg.mu.Unlock()

	// Creating 4th session should evict p1 (oldest disconnected idle session)
	p4, err := reg.CreatePlayer("NewestPlayer")
	if err != nil {
		t.Fatalf("unexpected error creating 4th player: %v", err)
	}

	if reg.Count() != 3 {
		t.Fatalf("expected session count bounded at 3, got %d", reg.Count())
	}
	if _, ok := reg.GetByID(p1.ID); ok {
		t.Errorf("expected oldest idle session p1 to be evicted")
	}
	if _, ok := reg.GetByID(p2.ID); !ok {
		t.Errorf("expected newer idle session p2 to remain")
	}
	if _, ok := reg.GetByID(p3.ID); !ok {
		t.Errorf("expected connected session p3 to remain")
	}
	if _, ok := reg.GetByID(p4.ID); !ok {
		t.Errorf("expected newly created session p4 to exist")
	}
}

