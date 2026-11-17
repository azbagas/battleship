package lobby

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	mrand "math/rand"
	"slices"
	"sync"
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
	"github.com/azbagas/battleship/backend/internal/store"
)

// Sentinel errors for lobby operations.
var (
	ErrRoomCapReached     = errors.New("maximum room capacity reached (10 rooms)")
	ErrAlreadyInRoom      = errors.New("player already has an active room or game")
	ErrRoomNotFound       = errors.New("room not found")
	ErrRoomNotAvailable   = errors.New("game is no longer available")
	ErrNotInWaitingRoom   = errors.New("player is not hosting a waiting room")
	ErrCannotJoinOwnRoom  = errors.New("cannot join your own room")
	ErrNotInReadyUp       = errors.New("room is not in ready-up phase")
	ErrNotInRoom          = errors.New("player is not in an active room")
	ErrNotInPlacement     = errors.New("room is not in placement phase")
	ErrFleetAlreadyLocked = errors.New("fleet is already confirmed and locked")
	ErrInvalidPlacement   = errors.New("invalid fleet placement")
	ErrNotInBattle        = errors.New("room is not in battle phase")
	ErrNotYourTurn        = errors.New("not your turn to fire")
	ErrGameOver           = errors.New("game has already finished")
	ErrNotInFinished      = errors.New("room is not in finished phase")
	ErrRematchNotAllowed  = errors.New("rematch is only available after a normal game end")
	ErrOpponentNotPresent = errors.New("opponent is not in the room or disconnected")
)

// ClientSender specifies the interface for transmitting messages to a connected player.
type ClientSender interface {
	Send(msgType string, payload any) error
}

// Option configures LobbyManager instances.
type Option func(*LobbyManager)

// WithClock sets a custom Clock on the LobbyManager.
func WithClock(clock Clock) Option {
	return func(m *LobbyManager) {
		m.clock = clock
	}
}

// WithMaxRooms sets the maximum allowed concurrent rooms.
func WithMaxRooms(max int) Option {
	return func(m *LobbyManager) {
		m.maxRooms = max
	}
}

// WithDisconnectGrace sets the disconnect grace duration.
func WithDisconnectGrace(d time.Duration) Option {
	return func(m *LobbyManager) {
		m.disconnectGrace = d
	}
}

// WithRNG sets a custom random number generator for deterministic placements in tests.
func WithRNG(rng *mrand.Rand) Option {
	return func(m *LobbyManager) {
		m.rng = rng
	}
}

// WithStore configures the SQLite persistence store for recording completed matches.
func WithStore(s store.Store) Option {
	return func(m *LobbyManager) {
		m.store = s
	}
}

// LobbyManager coordinates public lobby state, room lifecycles, and broadcasts.
type LobbyManager struct {
	mu               sync.RWMutex
	registry         *SessionRegistry
	rooms            map[string]*Room
	lobbySubscribers map[string]struct{}
	senders          map[string]ClientSender
	clock            Clock
	maxRooms         int
	disconnectGrace  time.Duration
	rng              *mrand.Rand
	store            store.Store
}

// NewLobbyManager constructs a thread-safe LobbyManager.
func NewLobbyManager(registry *SessionRegistry, opts ...Option) *LobbyManager {
	m := &LobbyManager{
		registry:         registry,
		rooms:            make(map[string]*Room),
		lobbySubscribers: make(map[string]struct{}),
		senders:          make(map[string]ClientSender),
		clock:            RealClock{},
		maxRooms:         10,
		disconnectGrace:  60 * time.Second,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// generateRoomID creates a unique room ID prefixed with "r_".
func generateRoomID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return "r_" + hex.EncodeToString(b)
}

// generateGameID creates a unique match ID prefixed with "g_".
func generateGameID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return "g_" + hex.EncodeToString(b)
}

func (m *LobbyManager) rngOrCrypto() *mrand.Rand {
	if m.rng != nil {
		return m.rng
	}
	return game.NewCryptoSeededRNG()
}

// RegisterConnection attaches a player's active message sender.
func (m *LobbyManager) RegisterConnection(playerID string, sender ClientSender) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.senders[playerID] = sender
}

// UnregisterConnection detaches a player's active message sender.
func (m *LobbyManager) UnregisterConnection(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.senders, playerID)
}

// SubscribeToLobby marks a player as an active listener for public lobby updates.
func (m *LobbyManager) SubscribeToLobby(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lobbySubscribers[playerID] = struct{}{}
}

// UnsubscribeFromLobby removes a player from public lobby updates.
func (m *LobbyManager) UnsubscribeFromLobby(playerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.lobbySubscribers, playerID)
}

// getPlayerRoomLocked resolves the player's current room with m.mu held.
func (m *LobbyManager) getPlayerRoomLocked(playerID string) (*Room, error) {
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		return nil, ErrNotInRoom
	}
	room, ok := m.rooms[currentRoomID]
	if !ok {
		return nil, ErrRoomNotFound
	}
	return room, nil
}

func (m *LobbyManager) playerDisplayName(playerID, fallback string) string {
	if playerID == "" {
		return fallback
	}
	if p, ok := m.registry.GetByID(playerID); ok && p.DisplayName != "" {
		return p.DisplayName
	}
	return fallback
}

// activeRoomsCountLocked returns the number of active rooms (waiting or in-play).
// Concluded matches in PhaseFinished do not count toward active room capacity.
// Caller must hold m.mu.
func (m *LobbyManager) activeRoomsCountLocked() int {
	count := 0
	for _, r := range m.rooms {
		if r.Phase != PhaseFinished {
			count++
		}
	}
	return count
}

// subscriberListLocked returns a snapshot slice of subscribed player IDs.
func (m *LobbyManager) subscriberListLocked() []string {
	list := make([]string, 0, len(m.lobbySubscribers))
	for id := range m.lobbySubscribers {
		list = append(list, id)
	}
	return list
}

// send sends a typed payload to a specific player connection without holding m.mu.
func (m *LobbyManager) send(playerID string, msgType string, payload any) {
	m.mu.RLock()
	sender, ok := m.senders[playerID]
	m.mu.RUnlock()

	if ok && sender != nil {
		_ = sender.Send(msgType, payload)
	}
}

// sendToRoom sends the same message to hostID and (if non-empty) guestID.
func (m *LobbyManager) sendToRoom(hostID, guestID, msgType string, payload any) {
	m.send(hostID, msgType, payload)
	if guestID != "" {
		m.send(guestID, msgType, payload)
	}
}

// broadcast sends a typed payload to all specified player connections without holding m.mu.
func (m *LobbyManager) broadcast(playerIDs []string, msgType string, payload any) {
	for _, id := range playerIDs {
		m.send(id, msgType, payload)
	}
}

// broadcastLobbyRemove broadcasts a room removal update to subscribers excluding any listed player IDs.
func (m *LobbyManager) broadcastLobbyRemove(subscribers []string, roomID string, totalRooms int, excludePlayerIDs ...string) {
	targets := subscribers
	if len(excludePlayerIDs) > 0 {
		targets = make([]string, 0, len(subscribers))
		for _, sub := range subscribers {
			if !slices.Contains(excludePlayerIDs, sub) {
				targets = append(targets, sub)
			}
		}
	}

	m.broadcast(targets, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     roomID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})
}

// buildRoomStateViewLocked constructs a tailored RoomStateView for the given player role.
// Must be called with m.mu held.
func (m *LobbyManager) buildRoomStateViewLocked(room *Room, role string) RoomStateView {
	hostName := m.playerDisplayName(room.HostPlayerID, "Host")

	opponentName := ""
	if role == "guest" {
		opponentName = hostName
	} else if room.GuestPlayerID != "" {
		opponentName = m.playerDisplayName(room.GuestPlayerID, "")
	}

	var guestView *RoomPlayerView
	if room.GuestPlayerID != "" {
		gName := opponentName
		if role == "guest" {
			gName = m.playerDisplayName(room.GuestPlayerID, opponentName)
		}
		guestView = &RoomPlayerView{ID: room.GuestPlayerID, Name: gName}
	}

	view := RoomStateView{
		RoomID:         room.ID,
		Phase:          room.Phase,
		Role:           role,
		OpponentName:   opponentName,
		Host:           RoomPlayerView{ID: room.HostPlayerID, Name: hostName},
		Guest:          guestView,
		HostReady:      room.HostReady,
		GuestReady:     room.GuestReady,
		CreatedAt:      room.CreatedAt,
		WinnerPlayerID: room.WinnerPlayerID,
		EndReason:      room.EndReason,
		ShotLog:        room.ShotLog,
	}

	switch role {
	case "host":
		if room.GuestPlayerID != "" {
			if guest, ok := m.registry.GetByID(room.GuestPlayerID); ok && guest.ConnectionState == StateDisconnected && !room.GuestDisconnectDeadline.IsZero() {
				view.OpponentDisconnected = true
				dl := room.GuestDisconnectDeadline
				view.OpponentGraceDeadline = &dl
			}
		}
	case "guest":
		if host, ok := m.registry.GetByID(room.HostPlayerID); ok && host.ConnectionState == StateDisconnected && !room.HostDisconnectDeadline.IsZero() {
			view.OpponentDisconnected = true
			dl := room.HostDisconnectDeadline
			view.OpponentGraceDeadline = &dl
		}
	}

	switch room.Phase {
	case PhasePlacement:
		view.PlacementDeadline = &room.PlacementDeadline
		if role == "host" {
			view.FleetConfirmed = room.HostFleetConfirmed
			view.OpponentConfirmed = room.GuestFleetConfirmed
			view.OwnFleet = room.HostFleet
		} else {
			view.FleetConfirmed = room.GuestFleetConfirmed
			view.OpponentConfirmed = room.HostFleetConfirmed
			view.OwnFleet = room.GuestFleet
		}
	case PhaseBattle, PhaseFinished:
		view.FleetConfirmed = true
		view.OpponentConfirmed = true
		view.TurnPlayerID = room.TurnPlayerID
		if !room.TurnDeadline.IsZero() {
			view.TurnDeadline = &room.TurnDeadline
		}

		var ownBoard, targetBoard *game.Board
		if role == "host" {
			view.OwnFleet = room.HostFleet
			ownBoard = room.HostBoard
			targetBoard = room.GuestBoard
		} else {
			view.OwnFleet = room.GuestFleet
			ownBoard = room.GuestBoard
			targetBoard = room.HostBoard
		}

		if ownBoard != nil && targetBoard != nil {
			bv := game.BuildPlayerBattleView(ownBoard, targetBoard)
			view.BattleView = &bv
		}
	}

	return view
}

// TotalRooms returns the total number of registered active rooms (waiting and in-play).
func (m *LobbyManager) TotalRooms() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeRoomsCountLocked()
}

// TotalRoomsInMemory returns the total number of all room objects in memory, including finished.
func (m *LobbyManager) TotalRoomsInMemory() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}

// SetRoomFinishedForTesting sets a room's state to finished for integration testing purposes.
func (m *LobbyManager) SetRoomFinishedForTesting(roomID, winnerID, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if room, ok := m.rooms[roomID]; ok {
		room.Phase = PhaseFinished
		room.WinnerPlayerID = winnerID
		room.EndReason = reason
	}
}

// GetRoomGameIDForTesting returns the current GameID for a room in integration tests.
func (m *LobbyManager) GetRoomGameIDForTesting(roomID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if room, ok := m.rooms[roomID]; ok {
		return room.GameID
	}
	return ""
}
