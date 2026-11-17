package lobby

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
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

const (
	defaultIdleSessionTTL = 5 * time.Minute
	defaultMaxSessions    = 2000
)

// SessionOption configures a SessionRegistry instance.
type SessionOption func(*SessionRegistry)

// WithIdleSessionTTL sets how long disconnected, roomless sessions remain resumable before pruning.
// A duration <= 0 disables age-based pruning.
func WithIdleSessionTTL(ttl time.Duration) SessionOption {
	return func(r *SessionRegistry) {
		r.idleSessionTTL = ttl
	}
}

// WithMaxSessions sets the upper bound of stored sessions before oldest idle disconnected sessions are evicted.
// A limit <= 0 disables the cap.
func WithMaxSessions(max int) SessionOption {
	return func(r *SessionRegistry) {
		r.maxSessions = max
	}
}

// SessionRegistry manages active and disconnected player sessions in memory.
type SessionRegistry struct {
	mu             sync.RWMutex
	byToken        map[string]*Player
	byID           map[string]*Player
	idleSessionTTL time.Duration
	maxSessions    int
}

// NewSessionRegistry constructs an empty SessionRegistry.
func NewSessionRegistry(opts ...SessionOption) *SessionRegistry {
	r := &SessionRegistry{
		byToken:        make(map[string]*Player),
		byID:           make(map[string]*Player),
		idleSessionTTL: defaultIdleSessionTTL,
		maxSessions:    defaultMaxSessions,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
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

	r.pruneIdleLocked(time.Now().UTC())

	r.byToken[token] = player
	r.byID[playerID] = player

	return player.clone(), nil
}

// PruneExpiredDisconnected removes disconnected sessions that have no active room
// and whose DisconnectedAt timestamp is strictly before cutoff.
func (r *SessionRegistry) PruneExpiredDisconnected(cutoff time.Time) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pruneExpiredLocked(cutoff)
}

// Count returns the total number of sessions currently retained in memory.
func (r *SessionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

func (r *SessionRegistry) pruneExpiredLocked(cutoff time.Time) int {
	removed := 0
	for id, p := range r.byID {
		if p.ConnectionState == StateDisconnected && p.CurrentRoomID == "" && p.DisconnectedAt != nil && p.DisconnectedAt.Before(cutoff) {
			delete(r.byID, id)
			delete(r.byToken, p.SessionToken)
			removed++
		}
	}
	return removed
}

// pruneIdleLocked keeps memory bounded by removing expired idle sessions first,
// then evicting the oldest disconnected roomless sessions if maxSessions is reached.
// Caller must hold r.mu.
func (r *SessionRegistry) pruneIdleLocked(now time.Time) {
	if r.idleSessionTTL > 0 {
		r.pruneExpiredLocked(now.Add(-r.idleSessionTTL))
	}

	if r.maxSessions <= 0 || len(r.byID) < r.maxSessions {
		return
	}

	var evictable []*Player
	for _, p := range r.byID {
		if p.ConnectionState == StateDisconnected && p.CurrentRoomID == "" {
			evictable = append(evictable, p)
		}
	}

	sort.Slice(evictable, func(i, j int) bool {
		ti := time.Time{}
		if evictable[i].DisconnectedAt != nil {
			ti = *evictable[i].DisconnectedAt
		}
		tj := time.Time{}
		if evictable[j].DisconnectedAt != nil {
			tj = *evictable[j].DisconnectedAt
		}
		return ti.Before(tj)
	})

	for _, p := range evictable {
		if len(r.byID) < r.maxSessions {
			break
		}
		delete(r.byID, p.ID)
		delete(r.byToken, p.SessionToken)
	}
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


