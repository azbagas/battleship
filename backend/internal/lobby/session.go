package lobby

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Sentinel errors for session operations.
var (
	ErrSessionNotFound      = errors.New("session not found")
	ErrSessionAlreadyActive = errors.New("session already active in another connection")
)

// ConnectionState represents whether a player currently has an active WebSocket connection.
type ConnectionState string

const (
	StateConnected    ConnectionState = "connected"
	StateDisconnected ConnectionState = "disconnected"
)

// Player represents an in-memory player identity.
type Player struct {
	ID              string          `json:"id"`
	DisplayName     string          `json:"displayName"`
	SessionToken    string          `json:"sessionToken"`
	ConnectionState ConnectionState `json:"connectionState"`
	CurrentRoomID   string          `json:"currentRoomId,omitempty"`
	DisconnectedAt  *time.Time      `json:"disconnectedAt,omitempty"`
}

// clone creates a shallow copy of Player with a cloned DisconnectedAt pointer.
func (p *Player) clone() *Player {
	cp := *p
	if p.DisconnectedAt != nil {
		t := *p.DisconnectedAt
		cp.DisconnectedAt = &t
	}
	return &cp
}

// SessionRegistry manages active and disconnected player sessions in memory.
type SessionRegistry struct {
	mu      sync.RWMutex
	byToken map[string]*Player
	byID    map[string]*Player
}

// NewSessionRegistry constructs an empty SessionRegistry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		byToken: make(map[string]*Player),
		byID:    make(map[string]*Player),
	}
}

// generateSecureToken creates an unguessable 32-byte URL-safe base64 string.
func generateSecureToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// generatePlayerID creates a unique internal player ID.
func generatePlayerID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate player id: %w", err)
	}
	return "p_" + hex.EncodeToString(b), nil
}

// CreatePlayer sanitizes the given display name, assigns a new unguessable session token
// and unique player ID, and registers the session as connected.
func (r *SessionRegistry) CreatePlayer(rawDisplayName string) (*Player, error) {
	sanitizedName, err := SanitizeDisplayName(rawDisplayName)
	if err != nil {
		return nil, fmt.Errorf("cannot create player: %w", err)
	}

	token, err := generateSecureToken()
	if err != nil {
		return nil, err
	}

	playerID, err := generatePlayerID()
	if err != nil {
		return nil, err
	}

	player := &Player{
		ID:              playerID,
		DisplayName:     sanitizedName,
		SessionToken:    token,
		ConnectionState: StateConnected,
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.byToken[token] = player
	r.byID[playerID] = player

	return player.clone(), nil
}

// GetByToken retrieves a player copy by their session token.
func (r *SessionRegistry) GetByToken(token string) (*Player, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	player, ok := r.byToken[token]
	if !ok {
		return nil, false
	}
	return player.clone(), true
}

// GetByID retrieves a player copy by their player ID.
func (r *SessionRegistry) GetByID(id string) (*Player, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	player, ok := r.byID[id]
	if !ok {
		return nil, false
	}
	return player.clone(), true
}

// AttachConnection marks an existing session as connected if currently disconnected.
// Returns ErrSessionAlreadyActive if a concurrent connection is already using this token.
func (r *SessionRegistry) AttachConnection(token string) (*Player, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.byToken[token]
	if !ok {
		return nil, ErrSessionNotFound
	}

	if player.ConnectionState == StateConnected {
		return nil, ErrSessionAlreadyActive
	}

	player.ConnectionState = StateConnected
	player.DisconnectedAt = nil

	return player.clone(), nil
}

// DetachConnection marks the session associated with the token as disconnected
// and records the disconnect timestamp for grace window tracking.
func (r *SessionRegistry) DetachConnection(token string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.byToken[token]
	if !ok {
		return
	}

	player.ConnectionState = StateDisconnected
	now := time.Now().UTC()
	player.DisconnectedAt = &now
}

// UpdateDisplayName updates the player's display name after validation.
func (r *SessionRegistry) UpdateDisplayName(token string, newDisplayName string) error {
	sanitizedName, err := SanitizeDisplayName(newDisplayName)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.byToken[token]
	if !ok {
		return ErrSessionNotFound
	}

	player.DisplayName = sanitizedName
	return nil
}

// SetCurrentRoom associates the player with an active room ID.
func (r *SessionRegistry) SetCurrentRoom(playerID string, roomID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.byID[playerID]
	if !ok {
		return ErrSessionNotFound
	}

	player.CurrentRoomID = roomID
	return nil
}

// ClearCurrentRoom removes the player's active room association.
func (r *SessionRegistry) ClearCurrentRoom(playerID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	player, ok := r.byID[playerID]
	if !ok {
		return ErrSessionNotFound
	}

	player.CurrentRoomID = ""
	return nil
}

// GetPlayerRoom returns the player's current room ID, if any.
func (r *SessionRegistry) GetPlayerRoom(playerID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	player, ok := r.byID[playerID]
	if !ok {
		return "", ErrSessionNotFound
	}

	return player.CurrentRoomID, nil
}


