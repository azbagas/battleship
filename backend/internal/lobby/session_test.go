package lobby

import (
	"errors"
	"sync"
	"testing"
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

