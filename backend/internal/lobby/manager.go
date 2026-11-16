package lobby

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	mrand "math/rand"
	"sort"
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

// GetLobbySnapshot generates the current waiting rooms snapshot sorted oldest first.
func (m *LobbyManager) GetLobbySnapshot() LobbySnapshotPayload {
	m.mu.RLock()
	defer m.mu.RUnlock()

	waitingRooms := make([]WaitingRoomView, 0)
	for _, room := range m.rooms {
		if room.Phase == PhaseWaiting {
			host, ok := m.registry.GetByID(room.HostPlayerID)
			hostName := "Host"
			if ok {
				hostName = host.DisplayName
			}
			waitingRooms = append(waitingRooms, WaitingRoomView{
				ID:        room.ID,
				HostName:  hostName,
				CreatedAt: room.CreatedAt,
			})
		}
	}

	// Sort oldest first (CreatedAt ascending)
	sort.Slice(waitingRooms, func(i, j int) bool {
		return waitingRooms[i].CreatedAt.Before(waitingRooms[j].CreatedAt)
	})

	return LobbySnapshotPayload{
		Rooms:      waitingRooms,
		TotalRooms: m.activeRoomsCountLocked(),
		MaxRooms:   m.maxRooms,
	}
}

// CreateRoom creates a new waiting room if under capacity and player is free.
func (m *LobbyManager) CreateRoom(playerID string) (*Room, error) {
	host, ok := m.registry.GetByID(playerID)
	if !ok {
		return nil, ErrSessionNotFound
	}

	m.mu.Lock()
	if m.activeRoomsCountLocked() >= m.maxRooms {
		total := m.activeRoomsCountLocked()
		m.mu.Unlock()
		slog.Warn("room creation rejected: room cap reached", "playerId", playerID, "totalRooms", total, "maxRooms", m.maxRooms)
		return nil, ErrRoomCapReached
	}

	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err == nil && currentRoomID != "" {
		m.mu.Unlock()
		slog.Warn("room creation rejected: already in room", "playerId", playerID, "currentRoomId", currentRoomID)
		return nil, ErrAlreadyInRoom
	}

	roomID := generateRoomID()
	room := &Room{
		ID:           roomID,
		HostPlayerID: playerID,
		Phase:        PhaseWaiting,
		CreatedAt:    m.clock.Now(),
	}
	m.rooms[roomID] = room
	_ = m.registry.SetCurrentRoom(playerID, roomID)
	delete(m.lobbySubscribers, playerID)

	waitingView := WaitingRoomView{
		ID:        room.ID,
		HostName:  host.DisplayName,
		CreatedAt: room.CreatedAt,
	}

	stateView := RoomStateView{
		RoomID:    room.ID,
		Phase:     PhaseWaiting,
		Role:      "host",
		Host:      RoomPlayerView{ID: host.ID, Name: host.DisplayName},
		CreatedAt: room.CreatedAt,
	}

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("room created", "roomId", room.ID, "hostId", playerID, "hostName", host.DisplayName, "totalRooms", totalRooms, "maxRooms", m.maxRooms)

	// Notify host
	m.send(playerID, "room.state", stateView)

	// Broadcast to remaining lobby subscribers
	m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "upsert",
		Room:       &waitingView,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})

	return room, nil
}

// CancelRoom allows the host to cancel their open waiting room.
func (m *LobbyManager) CancelRoom(playerID string) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		slog.Warn("room cancellation rejected: not in waiting room", "playerId", playerID)
		return ErrNotInWaitingRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok || room.HostPlayerID != playerID || room.Phase != PhaseWaiting {
		m.mu.Unlock()
		slog.Warn("room cancellation rejected: not host of waiting room", "playerId", playerID, "currentRoomId", currentRoomID)
		return ErrNotInWaitingRoom
	}

	if room.DisconnectTimer != nil {
		room.DisconnectTimer.Stop()
	}

	delete(m.rooms, room.ID)
	_ = m.registry.ClearCurrentRoom(playerID)
	m.lobbySubscribers[playerID] = struct{}{}

	totalRooms := m.activeRoomsCountLocked()
	removedRoomID := room.ID
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("room cancelled", "roomId", removedRoomID, "hostId", playerID, "totalRooms", totalRooms)

	// Send updated snapshot to the cancelling host
	m.send(playerID, "lobby.snapshot", m.GetLobbySnapshot())

	// Broadcast remove event to all other lobby subscribers
	m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     removedRoomID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})

	return nil
}

// JoinRoom atomically joins an open waiting room, transitioning it to ready-up.
func (m *LobbyManager) JoinRoom(playerID string, roomID string) (*Room, error) {
	guest, ok := m.registry.GetByID(playerID)
	if !ok {
		return nil, ErrSessionNotFound
	}

	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if !ok {
		m.mu.Unlock()
		slog.Warn("room join rejected: room not found", "playerId", playerID, "roomId", roomID)
		return nil, ErrRoomNotFound
	}

	if room.HostPlayerID == playerID {
		m.mu.Unlock()
		slog.Warn("room join rejected: cannot join own room", "playerId", playerID, "roomId", roomID)
		return nil, ErrCannotJoinOwnRoom
	}

	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err == nil && currentRoomID != "" {
		m.mu.Unlock()
		slog.Warn("room join rejected: already in room", "playerId", playerID, "currentRoomId", currentRoomID)
		return nil, ErrAlreadyInRoom
	}

	if room.Phase != PhaseWaiting || room.GuestPlayerID != "" {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn("room join rejected: room no longer available", "playerId", playerID, "roomId", roomID, "phase", phase)
		return nil, ErrRoomNotAvailable
	}

	// Atomic join transition
	room.GuestPlayerID = playerID
	room.Phase = PhaseReadyUp
	room.HostReady = false
	room.GuestReady = false
	if room.DisconnectTimer != nil {
		room.DisconnectTimer.Stop()
		room.DisconnectTimer = nil
	}
	if room.HostDisconnectTimer != nil {
		room.HostDisconnectTimer.Stop()
		room.HostDisconnectTimer = nil
	}
	if room.GuestDisconnectTimer != nil {
		room.GuestDisconnectTimer.Stop()
		room.GuestDisconnectTimer = nil
	}
	room.HostDisconnectDeadline = time.Time{}
	room.GuestDisconnectDeadline = time.Time{}
	if room.PlacementTimer != nil {
		room.PlacementTimer.Stop()
		room.PlacementTimer = nil
	}
	if room.TurnTimer != nil {
		room.TurnTimer.Stop()
		room.TurnTimer = nil
	}

	_ = m.registry.SetCurrentRoom(playerID, roomID)
	delete(m.lobbySubscribers, playerID)

	host, _ := m.registry.GetByID(room.HostPlayerID)
	hostName := "Host"
	if host != nil {
		hostName = host.DisplayName
	}

	hostView := RoomPlayerView{ID: room.HostPlayerID, Name: hostName}
	guestView := RoomPlayerView{ID: guest.ID, Name: guest.DisplayName}

	hostState := RoomStateView{
		RoomID:       room.ID,
		Phase:        PhaseReadyUp,
		Role:         "host",
		OpponentName: guest.DisplayName,
		Host:         hostView,
		Guest:        &guestView,
		HostReady:    false,
		GuestReady:   false,
		CreatedAt:    room.CreatedAt,
	}

	guestState := RoomStateView{
		RoomID:       room.ID,
		Phase:        PhaseReadyUp,
		Role:         "guest",
		OpponentName: hostName,
		Host:         hostView,
		Guest:        &guestView,
		HostReady:    false,
		GuestReady:   false,
		CreatedAt:    room.CreatedAt,
	}

	readyState := RoomReadyStateView{
		RoomID:     room.ID,
		HostReady:  false,
		GuestReady: false,
	}

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("player joined room", "roomId", room.ID, "hostId", room.HostPlayerID, "hostName", hostName, "guestId", playerID, "guestName", guest.DisplayName, "phase", room.Phase)

	// Notify both participants
	m.send(room.HostPlayerID, "room.state", hostState)
	m.send(room.HostPlayerID, "room.readyState", readyState)
	m.send(playerID, "room.state", guestState)
	m.send(playerID, "room.readyState", readyState)

	// Broadcast removal from public waiting list to lobby browsers
	m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     room.ID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})

	return room, nil
}

// ReadyRoom marks a player as ready in their active ready-up room.
// When both players are ready, the room automatically transitions to placement.
func (m *LobbyManager) ReadyRoom(playerID string) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		slog.Warn("ready action rejected: not in room", "playerId", playerID)
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		m.mu.Unlock()
		slog.Warn("ready action rejected: room not found", "playerId", playerID, "roomId", currentRoomID)
		return ErrRoomNotFound
	}

	if room.Phase != PhaseReadyUp {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn("ready action rejected: not in ready-up phase", "playerId", playerID, "roomId", currentRoomID, "phase", phase)
		return ErrNotInReadyUp
	}

	switch playerID {
	case room.HostPlayerID:
		room.HostReady = true
	case room.GuestPlayerID:
		room.GuestReady = true
	default:
		m.mu.Unlock()
		return ErrNotInRoom
	}

	readyState := RoomReadyStateView{
		RoomID:     room.ID,
		HostReady:  room.HostReady,
		GuestReady: room.GuestReady,
	}

	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID

	slog.Info("player set ready", "roomId", room.ID, "playerId", playerID, "hostReady", room.HostReady, "guestReady", room.GuestReady)

	// Check if both players are now ready -> transition to placement
	transitionToPlacement := room.HostReady && room.GuestReady
	var hostPlacementState, guestPlacementState RoomStateView
	if transitionToPlacement {
		room.Phase = PhasePlacement
		if room.GameID == "" {
			room.GameID = generateGameID()
		}
		if room.StartedAt.IsZero() {
			room.StartedAt = m.clock.Now()
		}
		placementDeadline := m.clock.Now().Add(60 * time.Second)
		room.PlacementDeadline = placementDeadline
		roomID := room.ID
		room.PlacementTimer = m.clock.AfterFunc(60*time.Second, func() {
			m.expirePlacementTimer(roomID)
		})

		host, _ := m.registry.GetByID(hostID)
		guest, _ := m.registry.GetByID(guestID)
		hostName := "Host"
		if host != nil {
			hostName = host.DisplayName
		}
		guestName := "Guest"
		if guest != nil {
			guestName = guest.DisplayName
		}

		hostView := RoomPlayerView{ID: hostID, Name: hostName}
		guestView := RoomPlayerView{ID: guestID, Name: guestName}

		hostPlacementState = RoomStateView{
			RoomID:            room.ID,
			Phase:             PhasePlacement,
			Role:              "host",
			OpponentName:      guestName,
			Host:              hostView,
			Guest:             &guestView,
			HostReady:         true,
			GuestReady:        true,
			PlacementDeadline: &placementDeadline,
			CreatedAt:         room.CreatedAt,
		}
		guestPlacementState = RoomStateView{
			RoomID:            room.ID,
			Phase:             PhasePlacement,
			Role:              "guest",
			OpponentName:      hostName,
			Host:              hostView,
			Guest:             &guestView,
			HostReady:         true,
			GuestReady:        true,
			PlacementDeadline: &placementDeadline,
			CreatedAt:         room.CreatedAt,
		}
		slog.Info("both players ready, room transitioned to placement", "roomId", room.ID, "hostId", hostID, "guestId", guestID, "deadline", placementDeadline)
	}

	m.mu.Unlock()

	// Broadcast readyState
	m.send(hostID, "room.readyState", readyState)
	if guestID != "" {
		m.send(guestID, "room.readyState", readyState)
	}

	// If transitioned to placement, broadcast placement room.state
	if transitionToPlacement {
		m.send(hostID, "room.state", hostPlacementState)
		if guestID != "" {
			m.send(guestID, "room.state", guestPlacementState)
		}
	}

	return nil
}

// UnreadyRoom clears a player's ready state in their active ready-up room.
func (m *LobbyManager) UnreadyRoom(playerID string) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		slog.Warn("unready action rejected: not in room", "playerId", playerID)
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		m.mu.Unlock()
		slog.Warn("unready action rejected: room not found", "playerId", playerID, "roomId", currentRoomID)
		return ErrRoomNotFound
	}

	if room.Phase != PhaseReadyUp {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn("unready action rejected: not in ready-up phase", "playerId", playerID, "roomId", currentRoomID, "phase", phase)
		return ErrNotInReadyUp
	}

	switch playerID {
	case room.HostPlayerID:
		room.HostReady = false
	case room.GuestPlayerID:
		room.GuestReady = false
	default:
		m.mu.Unlock()
		return ErrNotInRoom
	}

	readyState := RoomReadyStateView{
		RoomID:     room.ID,
		HostReady:  room.HostReady,
		GuestReady: room.GuestReady,
	}

	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID

	slog.Info("player set unready", "roomId", room.ID, "playerId", playerID, "hostReady", room.HostReady, "guestReady", room.GuestReady)
	m.mu.Unlock()

	m.send(hostID, "room.readyState", readyState)
	if guestID != "" {
		m.send(guestID, "room.readyState", readyState)
	}

	return nil
}

// UpdateDraftFleet saves a player's unconfirmed partial or complete ship placements (0..5 ships)
// during PhasePlacement without locking the fleet, so timeout auto-placement and reconnects preserve them.
func (m *LobbyManager) UpdateDraftFleet(playerID string, ships []game.PlacedShip) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		return ErrRoomNotFound
	}

	if room.Phase != PhasePlacement {
		return ErrNotInPlacement
	}

	isHost := playerID == room.HostPlayerID
	isGuest := playerID == room.GuestPlayerID
	if !isHost && !isGuest {
		return ErrNotInRoom
	}

	if (isHost && room.HostFleetConfirmed) || (isGuest && room.GuestFleetConfirmed) {
		return ErrFleetAlreadyLocked
	}

	if err := game.ValidatePartialFleet(ships); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidPlacement, err)
	}

	copied := make([]game.PlacedShip, len(ships))
	copy(copied, ships)

	if isHost {
		room.HostFleet = copied
	} else {
		room.GuestFleet = copied
	}

	return nil
}

// SubmitFleet locks in a player's ship placements during PhasePlacement.
func (m *LobbyManager) SubmitFleet(playerID string, ships []game.PlacedShip) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: not in room", "playerId", playerID)
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: room not found", "playerId", playerID, "roomId", currentRoomID)
		return ErrRoomNotFound
	}

	if room.Phase != PhasePlacement {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: not in placement phase", "playerId", playerID, "roomId", currentRoomID, "phase", phase)
		return ErrNotInPlacement
	}

	isHost := playerID == room.HostPlayerID
	isGuest := playerID == room.GuestPlayerID
	if !isHost && !isGuest {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	if (isHost && room.HostFleetConfirmed) || (isGuest && room.GuestFleetConfirmed) {
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: fleet already locked", "playerId", playerID, "roomId", currentRoomID)
		return ErrFleetAlreadyLocked
	}

	if err := game.ValidateFleet(ships); err != nil {
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: invalid fleet", "playerId", playerID, "roomId", currentRoomID, "error", err)
		return fmt.Errorf("%w: %w", ErrInvalidPlacement, err)
	}

	copied := make([]game.PlacedShip, len(ships))
	copy(copied, ships)

	var opponentID string
	if isHost {
		room.HostFleet = copied
		room.HostFleetConfirmed = true
		opponentID = room.GuestPlayerID
	} else {
		room.GuestFleet = copied
		room.GuestFleetConfirmed = true
		opponentID = room.HostPlayerID
	}

	slog.Info("player locked in fleet", "roomId", room.ID, "playerId", playerID, "isHost", isHost)

	transitionToBattle := room.HostFleetConfirmed && room.GuestFleetConfirmed
	var firstPlayerID string
	var turnDeadline time.Time
	var hostBattleState, guestBattleState RoomStateView
	roomID := room.ID
	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID

	if transitionToBattle {
		firstPlayerID, turnDeadline, hostBattleState, guestBattleState = m.initBattleLocked(room)
		slog.Info("both fleets confirmed, room transitioned to battle", "roomId", room.ID, "firstPlayerId", firstPlayerID)
	}

	m.mu.Unlock()

	// Notify submitting player of confirmation
	m.send(playerID, "placement.confirmed", map[string]any{
		"confirmed": true,
		"ships":     copied,
	})

	// Notify opponent that player confirmed without leaking coordinates
	if opponentID != "" {
		m.send(opponentID, "placement.opponentReady", map[string]any{
			"roomId": roomID,
		})
	}

	// If battle started, broadcast battle start, turn, and battle room.state
	if transitionToBattle {
		m.send(hostID, "battle.start", map[string]any{"firstPlayerId": firstPlayerID})
		if guestID != "" {
			m.send(guestID, "battle.start", map[string]any{"firstPlayerId": firstPlayerID})
		}

		turnPayload := map[string]any{
			"turnPlayerId": firstPlayerID,
			"deadline":     turnDeadline,
		}
		m.send(hostID, "battle.turn", turnPayload)
		if guestID != "" {
			m.send(guestID, "battle.turn", turnPayload)
		}

		m.send(hostID, "room.state", hostBattleState)
		if guestID != "" {
			m.send(guestID, "room.state", guestBattleState)
		}
	}

	return nil
}

// expirePlacementTimer handles timeout of the 60s fleet placement phase.
// Any unconfirmed player's remaining unplaced ships are auto-placed while keeping
// already placed ships untouched. The room then transitions to PhaseBattle.
func (m *LobbyManager) expirePlacementTimer(roomID string) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if !ok || room.Phase != PhasePlacement {
		m.mu.Unlock()
		return
	}

	if room.PlacementTimer != nil {
		room.PlacementTimer.Stop()
		room.PlacementTimer = nil
	}

	rng := m.rng
	if rng == nil {
		rng = mrand.New(mrand.NewSource(m.clock.Now().UnixNano()))
	}

	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID
	hostAutoPlaced := false
	guestAutoPlaced := false

	if !room.HostFleetConfirmed {
		fleet, err := game.AutoPlaceRemaining(room.HostFleet, rng)
		if err != nil {
			fleet = game.GenerateRandomFleet(rng)
		}
		room.HostFleet = fleet
		room.HostFleetConfirmed = true
		hostAutoPlaced = true
		slog.Info("placement timer expired, auto-placed host fleet", "roomId", roomID, "hostId", hostID)
	}

	if !room.GuestFleetConfirmed {
		fleet, err := game.AutoPlaceRemaining(room.GuestFleet, rng)
		if err != nil {
			fleet = game.GenerateRandomFleet(rng)
		}
		room.GuestFleet = fleet
		room.GuestFleetConfirmed = true
		guestAutoPlaced = true
		slog.Info("placement timer expired, auto-placed guest fleet", "roomId", roomID, "guestId", guestID)
	}

	firstPlayerID, turnDeadline, hostBattleState, guestBattleState := m.initBattleLocked(room)

	hostFleet := room.HostFleet
	guestFleet := room.GuestFleet

	m.mu.Unlock()

	// Notify auto-confirmed players of their final fleet
	if hostAutoPlaced {
		m.send(hostID, "placement.confirmed", map[string]any{
			"confirmed": true,
			"ships":     hostFleet,
			"auto":      true,
		})
	}
	if guestAutoPlaced && guestID != "" {
		m.send(guestID, "placement.confirmed", map[string]any{
			"confirmed": true,
			"ships":     guestFleet,
			"auto":      true,
		})
	}

	// Notify both of opponent readiness
	m.send(hostID, "placement.opponentReady", map[string]any{"roomId": roomID})
	if guestID != "" {
		m.send(guestID, "placement.opponentReady", map[string]any{"roomId": roomID})
	}

	// Broadcast battle.start, battle.turn, and room.state
	m.send(hostID, "battle.start", map[string]any{"firstPlayerId": firstPlayerID})
	if guestID != "" {
		m.send(guestID, "battle.start", map[string]any{"firstPlayerId": firstPlayerID})
	}

	turnPayload := map[string]any{
		"turnPlayerId": firstPlayerID,
		"deadline":     turnDeadline,
	}
	m.send(hostID, "battle.turn", turnPayload)
	if guestID != "" {
		m.send(guestID, "battle.turn", turnPayload)
	}

	m.send(hostID, "room.state", hostBattleState)
	if guestID != "" {
		m.send(guestID, "room.state", guestBattleState)
	}
}

// initBattleLocked initializes the battle state for a room whose fleets are confirmed.
// Must be called with m.mu locked.
func (m *LobbyManager) initBattleLocked(room *Room) (string, time.Time, RoomStateView, RoomStateView) {
	if room.PlacementTimer != nil {
		room.PlacementTimer.Stop()
		room.PlacementTimer = nil
	}
	room.Phase = PhaseBattle

	room.HostBoard, _ = game.NewBoard(room.HostFleet)
	room.GuestBoard, _ = game.NewBoard(room.GuestFleet)
	room.ShotLog = make([]ShotRecord, 0)

	rng := m.rng
	if rng == nil {
		rng = mrand.New(mrand.NewSource(m.clock.Now().UnixNano()))
	}

	firstPlayerID := game.ChooseFirstPlayer(room.HostPlayerID, room.GuestPlayerID, rng)
	room.TurnPlayerID = firstPlayerID
	turnDeadline := m.clock.Now().Add(30 * time.Second)
	room.TurnDeadline = turnDeadline

	roomID := room.ID
	room.TurnTimer = m.clock.AfterFunc(30*time.Second, func() {
		m.expireTurnTimer(roomID)
	})

	hostView := m.buildRoomStateViewLocked(room, "host")
	guestView := m.buildRoomStateViewLocked(room, "guest")

	slog.Info("battle phase initialized", "roomId", room.ID, "firstPlayerId", firstPlayerID, "turnDeadline", turnDeadline)
	return firstPlayerID, turnDeadline, hostView, guestView
}

// buildRoomStateViewLocked constructs a tailored RoomStateView for the given player role.
// Must be called with m.mu held.
func (m *LobbyManager) buildRoomStateViewLocked(room *Room, role string) RoomStateView {
	host, _ := m.registry.GetByID(room.HostPlayerID)
	hostName := "Host"
	if host != nil {
		hostName = host.DisplayName
	}

	opponentName := ""
	if role == "guest" {
		opponentName = hostName
	} else if room.GuestPlayerID != "" {
		if guest, ok := m.registry.GetByID(room.GuestPlayerID); ok {
			opponentName = guest.DisplayName
		}
	}

	var guestView *RoomPlayerView
	if room.GuestPlayerID != "" {
		gName := opponentName
		if role == "guest" {
			if me, ok := m.registry.GetByID(room.GuestPlayerID); ok {
				gName = me.DisplayName
			}
		}
		guestView = &RoomPlayerView{ID: room.GuestPlayerID, Name: gName}
	}

	hostView := RoomPlayerView{ID: room.HostPlayerID, Name: hostName}

	view := RoomStateView{
		RoomID:         room.ID,
		Phase:          room.Phase,
		Role:           role,
		OpponentName:   opponentName,
		Host:           hostView,
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

// FireShot executes a shot action by an active player against the opponent grid during PhaseBattle.
func (m *LobbyManager) FireShot(playerID string, cell game.Coord) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		slog.Warn("fire shot rejected: not in room", "playerId", playerID)
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		m.mu.Unlock()
		slog.Warn("fire shot rejected: room not found", "playerId", playerID, "roomId", currentRoomID)
		return ErrRoomNotFound
	}

	if room.Phase == PhaseFinished {
		m.mu.Unlock()
		slog.Warn("fire shot rejected: game already finished", "playerId", playerID, "roomId", currentRoomID)
		return ErrGameOver
	}

	if room.Phase != PhaseBattle {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn("fire shot rejected: not in battle phase", "playerId", playerID, "roomId", currentRoomID, "phase", phase)
		return ErrNotInBattle
	}

	if room.TurnPlayerID != playerID {
		turnID := room.TurnPlayerID
		m.mu.Unlock()
		slog.Warn("fire shot rejected: not your turn", "playerId", playerID, "turnPlayerId", turnID)
		return ErrNotYourTurn
	}

	isHost := (playerID == room.HostPlayerID)
	var targetBoard *game.Board
	var opponentID string
	if isHost {
		targetBoard = room.GuestBoard
		opponentID = room.GuestPlayerID
	} else {
		targetBoard = room.HostBoard
		opponentID = room.HostPlayerID
	}

	if targetBoard == nil {
		m.mu.Unlock()
		return errors.New("target board not initialized")
	}

	outcome, err := targetBoard.FireShot(cell)
	if err != nil {
		m.mu.Unlock()
		slog.Warn("fire shot rejected: invalid shot", "playerId", playerID, "cell", cell, "error", err)
		return err
	}

	// Shot succeeded, stop active turn timer
	if room.TurnTimer != nil {
		room.TurnTimer.Stop()
		room.TurnTimer = nil
	}

	record := ShotRecord{
		ShooterID: playerID,
		Cell:      cell,
		Result:    outcome.Result,
		SunkShip:  outcome.SunkShip,
		Auto:      false,
		Timestamp: m.clock.Now(),
	}
	room.ShotLog = append(room.ShotLog, record)

	slog.Info("shot executed", "roomId", room.ID, "shooterId", playerID, "cell", cell.String(), "result", outcome.Result)

	isWon := targetBoard.AllShipsSunk()
	var winnerID, endReason string
	var nextTurnID string
	var nextDeadline time.Time
	roomID := room.ID
	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID

	var wonSubscribers []string
	var wonTotalRooms int
	if isWon {
		room.Phase = PhaseFinished
		room.WinnerPlayerID = playerID
		room.EndReason = "all_sunk"
		winnerID = playerID
		endReason = "all_sunk"
		slog.Info("game over: all ships sunk", "roomId", room.ID, "winnerId", winnerID, "totalShots", len(room.ShotLog))
		m.recordGameLocked(room)
		wonSubscribers = m.subscriberListLocked()
		wonTotalRooms = m.activeRoomsCountLocked()
	} else {
		nextTurnID = opponentID
		room.TurnPlayerID = nextTurnID
		nextDeadline = m.clock.Now().Add(30 * time.Second)
		room.TurnDeadline = nextDeadline
		room.TurnTimer = m.clock.AfterFunc(30*time.Second, func() {
			m.expireTurnTimer(roomID)
		})
	}

	hostView := m.buildRoomStateViewLocked(room, "host")
	guestView := m.buildRoomStateViewLocked(room, "guest")
	totalShots := len(room.ShotLog)

	m.mu.Unlock()

	// Broadcast battle.shotResult
	shotResultPayload := map[string]any{
		"by":       playerID,
		"cell":     cell,
		"result":   outcome.Result,
		"sunkShip": outcome.SunkShip,
		"auto":     false,
	}
	m.send(hostID, "battle.shotResult", shotResultPayload)
	if guestID != "" {
		m.send(guestID, "battle.shotResult", shotResultPayload)
	}

	if isWon {
		gameOverPayload := map[string]any{
			"winnerPlayerId": winnerID,
			"reason":         endReason,
			"totalShots":     totalShots,
		}
		m.send(hostID, "game.over", gameOverPayload)
		if guestID != "" {
			m.send(guestID, "game.over", gameOverPayload)
		}
		m.broadcast(wonSubscribers, "lobby.update", LobbyUpdatePayload{
			Action:     "remove",
			RoomID:     roomID,
			TotalRooms: wonTotalRooms,
			MaxRooms:   m.maxRooms,
		})
	} else {
		turnPayload := map[string]any{
			"turnPlayerId": nextTurnID,
			"deadline":     nextDeadline,
		}
		m.send(hostID, "battle.turn", turnPayload)
		if guestID != "" {
			m.send(guestID, "battle.turn", turnPayload)
		}
	}

	m.send(hostID, "room.state", hostView)
	if guestID != "" {
		m.send(guestID, "room.state", guestView)
	}

	return nil
}

// expireTurnTimer automatically executes a random untargeted shot on behalf of an active player upon 30s timeout.
func (m *LobbyManager) expireTurnTimer(roomID string) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if !ok || room.Phase != PhaseBattle {
		m.mu.Unlock()
		return
	}

	if room.TurnTimer != nil {
		room.TurnTimer.Stop()
		room.TurnTimer = nil
	}

	activePlayerID := room.TurnPlayerID
	isHost := (activePlayerID == room.HostPlayerID)
	var targetBoard *game.Board
	var opponentID string
	if isHost {
		targetBoard = room.GuestBoard
		opponentID = room.GuestPlayerID
	} else {
		targetBoard = room.HostBoard
		opponentID = room.HostPlayerID
	}

	if targetBoard == nil {
		m.mu.Unlock()
		return
	}

	rng := m.rng
	if rng == nil {
		rng = mrand.New(mrand.NewSource(m.clock.Now().UnixNano()))
	}

	cell, err := game.RandomUntargetedCell(targetBoard, rng)
	if err != nil {
		m.mu.Unlock()
		slog.Error("failed to find untargeted cell for auto-shot", "roomId", roomID, "error", err)
		return
	}

	outcome, err := targetBoard.FireShot(cell)
	if err != nil {
		m.mu.Unlock()
		slog.Error("failed to execute auto-shot", "roomId", roomID, "cell", cell, "error", err)
		return
	}

	record := ShotRecord{
		ShooterID: activePlayerID,
		Cell:      cell,
		Result:    outcome.Result,
		SunkShip:  outcome.SunkShip,
		Auto:      true,
		Timestamp: m.clock.Now(),
	}
	room.ShotLog = append(room.ShotLog, record)

	slog.Info("turn timer expired, auto-shot executed", "roomId", roomID, "shooterId", activePlayerID, "cell", cell.String(), "result", outcome.Result)

	isWon := targetBoard.AllShipsSunk()
	var winnerID, endReason string
	var nextTurnID string
	var nextDeadline time.Time
	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID

	var wonSubscribers []string
	var wonTotalRooms int
	if isWon {
		room.Phase = PhaseFinished
		room.WinnerPlayerID = activePlayerID
		room.EndReason = "all_sunk"
		winnerID = activePlayerID
		endReason = "all_sunk"
		slog.Info("game over via auto-shot: all ships sunk", "roomId", room.ID, "winnerId", winnerID, "totalShots", len(room.ShotLog))
		m.recordGameLocked(room)
		wonSubscribers = m.subscriberListLocked()
		wonTotalRooms = m.activeRoomsCountLocked()
	} else {
		nextTurnID = opponentID
		room.TurnPlayerID = nextTurnID
		nextDeadline = m.clock.Now().Add(30 * time.Second)
		room.TurnDeadline = nextDeadline
		room.TurnTimer = m.clock.AfterFunc(30*time.Second, func() {
			m.expireTurnTimer(roomID)
		})
	}

	hostView := m.buildRoomStateViewLocked(room, "host")
	guestView := m.buildRoomStateViewLocked(room, "guest")
	totalShots := len(room.ShotLog)

	m.mu.Unlock()

	// Broadcast battle.shotResult with auto: true
	shotResultPayload := map[string]any{
		"by":       activePlayerID,
		"cell":     cell,
		"result":   outcome.Result,
		"sunkShip": outcome.SunkShip,
		"auto":     true,
	}
	m.send(hostID, "battle.shotResult", shotResultPayload)
	if guestID != "" {
		m.send(guestID, "battle.shotResult", shotResultPayload)
	}

	if isWon {
		gameOverPayload := map[string]any{
			"winnerPlayerId": winnerID,
			"reason":         endReason,
			"totalShots":     totalShots,
		}
		m.send(hostID, "game.over", gameOverPayload)
		if guestID != "" {
			m.send(guestID, "game.over", gameOverPayload)
		}
		m.broadcast(wonSubscribers, "lobby.update", LobbyUpdatePayload{
			Action:     "remove",
			RoomID:     roomID,
			TotalRooms: wonTotalRooms,
			MaxRooms:   m.maxRooms,
		})
	} else {
		turnPayload := map[string]any{
			"turnPlayerId": nextTurnID,
			"deadline":     nextDeadline,
		}
		m.send(hostID, "battle.turn", turnPayload)
		if guestID != "" {
			m.send(guestID, "battle.turn", turnPayload)
		}
	}

	m.send(hostID, "room.state", hostView)
	if guestID != "" {
		m.send(guestID, "room.state", guestView)
	}
}

// LeaveRoom handles explicit departures from waiting or ready-up rooms.
// If in waiting and host: cancels the waiting room.
// If in ready-up and guest: returns the room to waiting lobby for the host.
// If in ready-up and host: closes the room and returns the guest to lobby.
func (m *LobbyManager) LeaveRoom(playerID string) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		slog.Warn("leave action rejected: not in room", "playerId", playerID)
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		_ = m.registry.ClearCurrentRoom(playerID)
		m.mu.Unlock()
		slog.Warn("leave action rejected: room not found", "playerId", playerID, "roomId", currentRoomID)
		return ErrRoomNotFound
	}

	if room.Phase == PhaseWaiting {
		m.mu.Unlock()
		return m.CancelRoom(playerID)
	}

	if room.Phase == PhaseReadyUp {
		return m.leaveReadyUpLocked(room, playerID)
	}

	if room.Phase == PhasePlacement || room.Phase == PhaseBattle {
		return m.leaveMatchLocked(room, playerID)
	}

	if room.Phase == PhaseFinished {
		return m.leaveFinishedLocked(room, playerID)
	}

	m.mu.Unlock()
	return ErrNotInRoom
}

// leaveMatchLocked forfeits an in-progress match during PhasePlacement or PhaseBattle with m.mu held.
func (m *LobbyManager) leaveMatchLocked(room *Room, playerID string) error {
	isHost := (playerID == room.HostPlayerID)
	isGuest := (playerID == room.GuestPlayerID)

	if !isHost && !isGuest {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	var opponentID string
	var opponentRole string
	if isHost {
		opponentID = room.GuestPlayerID
		opponentRole = "guest"
	} else {
		opponentID = room.HostPlayerID
		opponentRole = "host"
	}

	// Stop all active timers
	if room.PlacementTimer != nil {
		room.PlacementTimer.Stop()
		room.PlacementTimer = nil
	}
	if room.TurnTimer != nil {
		room.TurnTimer.Stop()
		room.TurnTimer = nil
	}
	if room.HostDisconnectTimer != nil {
		room.HostDisconnectTimer.Stop()
		room.HostDisconnectTimer = nil
	}
	if room.GuestDisconnectTimer != nil {
		room.GuestDisconnectTimer.Stop()
		room.GuestDisconnectTimer = nil
	}
	room.HostDisconnectDeadline = time.Time{}
	room.GuestDisconnectDeadline = time.Time{}

	// Transition to finished with forfeit
	room.Phase = PhaseFinished
	room.WinnerPlayerID = opponentID
	room.EndReason = "forfeit"
	m.recordGameLocked(room)

	// Clear leaving player's room attachment
	_ = m.registry.ClearCurrentRoom(playerID)
	m.lobbySubscribers[playerID] = struct{}{}

	totalShots := len(room.ShotLog)
	var opponentView RoomStateView
	if opponentID != "" {
		opponentView = m.buildRoomStateViewLocked(room, opponentRole)
	}

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	roomID := room.ID

	slog.Info("player forfeited match", "roomId", room.ID, "playerId", playerID, "opponentId", opponentID, "totalShots", totalShots)
	m.mu.Unlock()

	// Send leaving player fresh lobby snapshot
	m.send(playerID, "lobby.snapshot", m.GetLobbySnapshot())

	// Notify opponent of victory via forfeit and provide updated room view
	if opponentID != "" {
		m.send(opponentID, "game.over", map[string]any{
			"winnerPlayerId": opponentID,
			"reason":         "forfeit",
			"totalShots":     totalShots,
		})
		m.send(opponentID, "room.state", opponentView)
	}

	otherSubscribers := make([]string, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub != playerID {
			otherSubscribers = append(otherSubscribers, sub)
		}
	}
	m.broadcast(otherSubscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     roomID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})

	return nil
}

// leaveFinishedLocked handles departure from a finished match, returning player to lobby with m.mu held.
func (m *LobbyManager) leaveFinishedLocked(room *Room, playerID string) error {
	isHost := (playerID == room.HostPlayerID)
	isGuest := (playerID == room.GuestPlayerID)

	if !isHost && !isGuest {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	// Clear leaving player's room
	_ = m.registry.ClearCurrentRoom(playerID)
	m.lobbySubscribers[playerID] = struct{}{}

	var otherID string
	if isHost {
		otherID = room.GuestPlayerID
	} else {
		otherID = room.HostPlayerID
	}

	// Check if other player is also no longer in this room
	otherInRoom := false
	if otherID != "" {
		if otherRoom, err := m.registry.GetPlayerRoom(otherID); err == nil && otherRoom == room.ID {
			otherInRoom = true
		}
	}

	// If a rematch offer was pending or active, flag for notification after unlock
	notifyOtherRematchCancelled := false
	if otherInRoom && otherID != "" {
		if room.HostRematch || room.GuestRematch {
			room.HostRematch = false
			room.GuestRematch = false
			notifyOtherRematchCancelled = true
		}
	}

	// If other player has also left, remove room from memory
	roomDeleted := false
	if !otherInRoom {
		delete(m.rooms, room.ID)
		roomDeleted = true
		slog.Info("both players left finished room, cleaned up", "roomId", room.ID)
	}

	slog.Info("player left finished room, returned to lobby", "roomId", room.ID, "playerId", playerID)

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	roomID := room.ID
	m.mu.Unlock()

	if notifyOtherRematchCancelled {
		m.send(otherID, "rematch.cancelled", map[string]any{
			"reason": "left",
		})
	}

	m.send(playerID, "lobby.snapshot", m.GetLobbySnapshot())

	if roomDeleted {
		m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
			Action:     "remove",
			RoomID:     roomID,
			TotalRooms: totalRooms,
			MaxRooms:   m.maxRooms,
		})
	}

	return nil
}

// recordGameLocked writes the finished match record to SQLite if a store is configured.
// Caller must hold m.mu.
func (m *LobbyManager) recordGameLocked(room *Room) {
	if m.store == nil || room == nil || room.Recorded {
		return
	}
	room.Recorded = true
	room.EndedAt = m.clock.Now()

	var winnerSlot *int
	if room.WinnerPlayerID != "" {
		var slot int
		if room.WinnerPlayerID == room.HostPlayerID {
			slot = 1
		} else if room.WinnerPlayerID == room.GuestPlayerID {
			slot = 2
		}
		if slot != 0 {
			winnerSlot = &slot
		}
	}

	host, _ := m.registry.GetByID(room.HostPlayerID)
	guest, _ := m.registry.GetByID(room.GuestPlayerID)
	hostName := "Host Commander"
	if host != nil && host.DisplayName != "" {
		hostName = host.DisplayName
	}
	guestName := "Guest Commander"
	if guest != nil && guest.DisplayName != "" {
		guestName = guest.DisplayName
	}

	shotLogBytes, err := json.Marshal(room.ShotLog)
	if err != nil {
		slog.Error("failed to marshal shot log for game record", "roomId", room.ID, "gameId", room.GameID, "error", err)
		shotLogBytes = []byte("[]")
	}

	startedAt := room.StartedAt
	if startedAt.IsZero() {
		startedAt = room.CreatedAt
	}

	gameID := room.GameID
	if gameID == "" {
		gameID = generateGameID()
		room.GameID = gameID
	}

	record := &store.GameRecord{
		ID:          gameID,
		StartedAt:   startedAt,
		EndedAt:     room.EndedAt,
		Player1Name: hostName,
		Player2Name: guestName,
		WinnerSlot:  winnerSlot,
		EndReason:   room.EndReason,
		TotalShots:  len(room.ShotLog),
		ShotLog:     string(shotLogBytes),
	}

	go func(rec *store.GameRecord) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.SaveGame(ctx, rec); err != nil {
			slog.Error("failed to persist game record to store", "gameId", rec.ID, "error", err)
		} else {
			slog.Info("game record persisted successfully", "gameId", rec.ID, "reason", rec.EndReason, "totalShots", rec.TotalShots)
		}
	}(record)
}

// AcceptRematch registers a player's acceptance of a rematch offer.
// If both players mutually accept, the room restarts directly into PhasePlacement with re-randomized first turn on battle start.
func (m *LobbyManager) AcceptRematch(playerID string) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		m.mu.Unlock()
		return ErrRoomNotFound
	}

	if room.Phase != PhaseFinished {
		m.mu.Unlock()
		return ErrNotInFinished
	}

	if room.EndReason != "all_sunk" {
		m.mu.Unlock()
		return ErrRematchNotAllowed
	}

	isHost := (playerID == room.HostPlayerID)
	isGuest := (playerID == room.GuestPlayerID)
	if !isHost && !isGuest {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	var opponentID string
	if isHost {
		opponentID = room.GuestPlayerID
	} else {
		opponentID = room.HostPlayerID
	}

	if opponentID == "" {
		m.mu.Unlock()
		return ErrOpponentNotPresent
	}

	// Opponent must still be attached to this room and connected
	oppRoom, err := m.registry.GetPlayerRoom(opponentID)
	if err != nil || oppRoom != room.ID {
		m.mu.Unlock()
		return ErrOpponentNotPresent
	}
	opp, oppOk := m.registry.GetByID(opponentID)
	if !oppOk || opp.ConnectionState == StateDisconnected {
		m.mu.Unlock()
		return ErrOpponentNotPresent
	}

	if isHost {
		room.HostRematch = true
	} else {
		room.GuestRematch = true
	}

	slog.Info("rematch offer accepted by player", "roomId", room.ID, "playerId", playerID, "hostRematch", room.HostRematch, "guestRematch", room.GuestRematch)

	mutualAccepted := room.HostRematch && room.GuestRematch

	if !mutualAccepted {
		m.mu.Unlock()
		// Broadcast rematch.offered to both players so local UI and opponent UI update
		m.send(opponentID, "rematch.offered", map[string]any{
			"by": playerID,
		})
		m.send(playerID, "rematch.offered", map[string]any{
			"by": playerID,
		})
		return nil
	}

	// Mutual rematch accepted: reset match state and transition directly to PhasePlacement
	room.Phase = PhasePlacement
	room.HostFleet = nil
	room.GuestFleet = nil
	room.HostFleetConfirmed = false
	room.GuestFleetConfirmed = false
	room.HostBoard = nil
	room.GuestBoard = nil
	room.ShotLog = make([]ShotRecord, 0)
	room.WinnerPlayerID = ""
	room.EndReason = ""
	room.HostRematch = false
	room.GuestRematch = false
	room.GameID = generateGameID()
	room.StartedAt = m.clock.Now()
	room.Recorded = false

	placementDeadline := m.clock.Now().Add(60 * time.Second)
	room.PlacementDeadline = placementDeadline
	roomID := room.ID
	room.PlacementTimer = m.clock.AfterFunc(60*time.Second, func() {
		m.expirePlacementTimer(roomID)
	})

	hostView := m.buildRoomStateViewLocked(room, "host")
	guestView := m.buildRoomStateViewLocked(room, "guest")
	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()

	slog.Info("mutual rematch confirmed, room restarting into placement", "roomId", room.ID, "newGameId", room.GameID, "deadline", placementDeadline)
	m.mu.Unlock()

	m.send(room.HostPlayerID, "room.state", hostView)
	m.send(room.GuestPlayerID, "room.state", guestView)

	m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     room.ID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})
	return nil
}

// DeclineRematch explicitly declines a pending rematch offer and cancels it for both players.
func (m *LobbyManager) DeclineRematch(playerID string) error {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		m.mu.Unlock()
		return ErrRoomNotFound
	}

	if room.Phase != PhaseFinished {
		m.mu.Unlock()
		return ErrNotInFinished
	}

	isHost := (playerID == room.HostPlayerID)
	isGuest := (playerID == room.GuestPlayerID)
	if !isHost && !isGuest {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	var opponentID string
	if isHost {
		opponentID = room.GuestPlayerID
	} else {
		opponentID = room.HostPlayerID
	}

	room.HostRematch = false
	room.GuestRematch = false

	slog.Info("rematch declined", "roomId", room.ID, "playerId", playerID)
	m.mu.Unlock()

	if opponentID != "" {
		m.send(opponentID, "rematch.cancelled", map[string]any{
			"reason": "declined",
		})
	}
	m.send(playerID, "rematch.cancelled", map[string]any{
		"reason": "declined",
	})
	return nil
}

// leaveReadyUpLocked executes departure rules for a ready-up room with m.mu held.
func (m *LobbyManager) leaveReadyUpLocked(room *Room, playerID string) error {
	isHost := (playerID == room.HostPlayerID)
	isGuest := (playerID == room.GuestPlayerID)

	if !isHost && !isGuest {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	if isGuest {
		if room.GuestDisconnectTimer != nil {
			room.GuestDisconnectTimer.Stop()
			room.GuestDisconnectTimer = nil
		}
		guestID := room.GuestPlayerID
		hostID := room.HostPlayerID

		// Reset room back to PhaseWaiting
		room.GuestPlayerID = ""
		room.GuestReady = false
		room.HostReady = false
		room.Phase = PhaseWaiting

		// Clear guest's room and subscribe back to lobby
		_ = m.registry.ClearCurrentRoom(guestID)
		m.lobbySubscribers[guestID] = struct{}{}

		// If host had disconnected while guest was in room, ensure host disconnect timer is running
		host, _ := m.registry.GetByID(hostID)
		if host != nil && host.ConnectionState == StateDisconnected {
			if room.HostDisconnectTimer == nil && room.DisconnectTimer == nil {
				roomID := room.ID
				room.DisconnectTimer = m.clock.AfterFunc(m.disconnectGrace, func() {
					m.expireWaitingRoom(roomID, hostID)
				})
			}
		}

		hostName := "Host"
		if host != nil {
			hostName = host.DisplayName
		}

		waitingView := WaitingRoomView{
			ID:        room.ID,
			HostName:  hostName,
			CreatedAt: room.CreatedAt,
		}

		hostState := RoomStateView{
			RoomID:    room.ID,
			Phase:     PhaseWaiting,
			Role:      "host",
			Host:      RoomPlayerView{ID: hostID, Name: hostName},
			CreatedAt: room.CreatedAt,
		}

		totalRooms := m.activeRoomsCountLocked()
		subscribers := m.subscriberListLocked()
		m.mu.Unlock()

		slog.Info("guest left ready-up, room returned to waiting lobby", "roomId", room.ID, "hostId", hostID, "guestId", guestID, "totalRooms", totalRooms)

		// Send host updated waiting room state
		m.send(hostID, "room.state", hostState)

		// Send guest fresh lobby snapshot
		m.send(guestID, "lobby.snapshot", m.GetLobbySnapshot())

		// Re-broadcast waiting room to all lobby subscribers
		m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
			Action:     "upsert",
			Room:       &waitingView,
			TotalRooms: totalRooms,
			MaxRooms:   m.maxRooms,
		})

		return nil
	}

	// Host left: close the room
	if room.HostDisconnectTimer != nil {
		room.HostDisconnectTimer.Stop()
		room.HostDisconnectTimer = nil
	}
	if room.GuestDisconnectTimer != nil {
		room.GuestDisconnectTimer.Stop()
		room.GuestDisconnectTimer = nil
	}
	room.HostDisconnectDeadline = time.Time{}
	room.GuestDisconnectDeadline = time.Time{}
	if room.PlacementTimer != nil {
		room.PlacementTimer.Stop()
		room.PlacementTimer = nil
	}
	if room.TurnTimer != nil {
		room.TurnTimer.Stop()
		room.TurnTimer = nil
	}

	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID
	roomID := room.ID

	delete(m.rooms, roomID)
	_ = m.registry.ClearCurrentRoom(hostID)
	_ = m.registry.ClearCurrentRoom(guestID)

	m.lobbySubscribers[hostID] = struct{}{}
	if guestID != "" {
		m.lobbySubscribers[guestID] = struct{}{}
	}

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("host left ready-up, room closed", "roomId", roomID, "hostId", hostID, "guestId", guestID, "totalRooms", totalRooms)

	// Send lobby snapshots to both players
	m.send(hostID, "lobby.snapshot", m.GetLobbySnapshot())
	if guestID != "" {
		m.send(guestID, "lobby.snapshot", m.GetLobbySnapshot())
	}

	otherSubscribers := make([]string, 0, len(subscribers))
	for _, sub := range subscribers {
		if sub != hostID && sub != guestID {
			otherSubscribers = append(otherSubscribers, sub)
		}
	}
	m.broadcast(otherSubscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     roomID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})

	return nil
}

// HandleDisconnect manages lobby subscriber cleanup and waiting room / match grace timers.
func (m *LobbyManager) HandleDisconnect(playerID string) {
	m.mu.Lock()
	delete(m.lobbySubscribers, playerID)

	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		return
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		m.mu.Unlock()
		return
	}

	// If player is host of a waiting room, start the 60s disconnect grace timer
	if room.Phase == PhaseWaiting && room.HostPlayerID == playerID {
		roomID := room.ID
		slog.Info("waiting room host disconnected, starting grace timer", "roomId", roomID, "hostId", playerID, "graceDuration", m.disconnectGrace.String())
		room.DisconnectTimer = m.clock.AfterFunc(m.disconnectGrace, func() {
			m.expireWaitingRoom(roomID, playerID)
		})
		m.mu.Unlock()
		return
	}

	// Active multi-player phases: PhaseReadyUp, PhasePlacement, PhaseBattle
	if room.Phase == PhaseReadyUp || room.Phase == PhasePlacement || room.Phase == PhaseBattle {
		roomID := room.ID
		deadline := m.clock.Now().Add(m.disconnectGrace)
		var opponentID string

		switch playerID {
		case room.HostPlayerID:
			if room.HostDisconnectTimer != nil {
				room.HostDisconnectTimer.Stop()
			}
			room.HostDisconnectDeadline = deadline
			opponentID = room.GuestPlayerID
			slog.Info("room host disconnected, starting grace timer", "roomId", roomID, "hostId", playerID, "phase", room.Phase, "graceDeadline", deadline)
			room.HostDisconnectTimer = m.clock.AfterFunc(m.disconnectGrace, func() {
				m.expireDisconnectGrace(roomID, playerID, true)
			})
		case room.GuestPlayerID:
			if room.GuestDisconnectTimer != nil {
				room.GuestDisconnectTimer.Stop()
			}
			room.GuestDisconnectDeadline = deadline
			opponentID = room.HostPlayerID
			slog.Info("room guest disconnected, starting grace timer", "roomId", roomID, "guestId", playerID, "phase", room.Phase, "graceDeadline", deadline)
			room.GuestDisconnectTimer = m.clock.AfterFunc(m.disconnectGrace, func() {
				m.expireDisconnectGrace(roomID, playerID, false)
			})
		}

		m.mu.Unlock()

		// Notify connected opponent about disconnection with graceDeadline
		if opponentID != "" {
			m.send(opponentID, "opponent.disconnected", map[string]any{
				"graceDeadline": deadline,
			})
		}
		return
	}

	// Finished phase: cancel any pending rematch offers and notify opponent
	if room.Phase == PhaseFinished {
		var opponentID string
		if playerID == room.HostPlayerID {
			opponentID = room.GuestPlayerID
		} else {
			opponentID = room.HostPlayerID
		}

		hadRematchOffer := room.HostRematch || room.GuestRematch
		room.HostRematch = false
		room.GuestRematch = false

		_ = m.registry.ClearCurrentRoom(playerID)

		otherInRoom := false
		if opponentID != "" {
			if oppRoom, err := m.registry.GetPlayerRoom(opponentID); err == nil && oppRoom == room.ID {
				otherInRoom = true
			}
		}

		roomDeleted := false
		if !otherInRoom {
			delete(m.rooms, room.ID)
			roomDeleted = true
			slog.Info("finished room cleaned up on disconnect", "roomId", room.ID, "playerId", playerID)
		}

		totalRooms := m.activeRoomsCountLocked()
		subscribers := m.subscriberListLocked()
		roomID := room.ID
		m.mu.Unlock()

		if hadRematchOffer && opponentID != "" {
			m.send(opponentID, "rematch.cancelled", map[string]any{
				"reason": "disconnected",
			})
		}

		if roomDeleted {
			m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
				Action:     "remove",
				RoomID:     roomID,
				TotalRooms: totalRooms,
				MaxRooms:   m.maxRooms,
			})
		}
		return
	}

	m.mu.Unlock()
}

// expireDisconnectGrace handles disconnect timeout during ready-up, placement, and battle.
func (m *LobbyManager) expireDisconnectGrace(roomID string, playerID string, isHost bool) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if !ok {
		m.mu.Unlock()
		return
	}

	// Verify player is still disconnected before taking action
	p, ok := m.registry.GetByID(playerID)
	if ok && p.ConnectionState == StateConnected {
		m.mu.Unlock()
		return
	}

	if room.Phase == PhaseReadyUp {
		if isHost && room.HostPlayerID == playerID {
			slog.Info("ready-up host disconnect grace expired, closing room", "roomId", roomID, "hostId", playerID)
			_ = m.leaveReadyUpLocked(room, playerID)
			return
		}

		if !isHost && room.GuestPlayerID == playerID {
			slog.Info("ready-up guest disconnect grace expired, returning room to waiting lobby", "roomId", roomID, "guestId", playerID)
			_ = m.leaveReadyUpLocked(room, playerID)
			return
		}

		m.mu.Unlock()
		return
	}

	if room.Phase == PhasePlacement || room.Phase == PhaseBattle {
		var opponentID string
		var opponentRole string
		if isHost {
			opponentID = room.GuestPlayerID
			opponentRole = "guest"
		} else {
			opponentID = room.HostPlayerID
			opponentRole = "host"
		}

		// Stop any active placement or turn timers
		if room.PlacementTimer != nil {
			room.PlacementTimer.Stop()
			room.PlacementTimer = nil
		}
		if room.TurnTimer != nil {
			room.TurnTimer.Stop()
			room.TurnTimer = nil
		}

		opp, oppOk := m.registry.GetByID(opponentID)
		oppDisconnected := !oppOk || opp.ConnectionState == StateDisconnected

		if oppDisconnected {
			// PRD F-06: records match as abandoned with no winner if both players are disconnected when grace expires
			room.Phase = PhaseFinished
			room.WinnerPlayerID = ""
			room.EndReason = "abandoned"
			slog.Info("match abandoned: both players disconnected when grace period expired", "roomId", roomID)
			m.recordGameLocked(room)
			delete(m.rooms, roomID)
			_ = m.registry.ClearCurrentRoom(playerID)
			_ = m.registry.ClearCurrentRoom(opponentID)
			totalRooms := m.activeRoomsCountLocked()
			subscribers := m.subscriberListLocked()
			m.mu.Unlock()
			m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
				Action:     "remove",
				RoomID:     roomID,
				TotalRooms: totalRooms,
				MaxRooms:   m.maxRooms,
			})
			return
		}

		// Opponent is connected -> awards win to connected opponent
		room.Phase = PhaseFinished
		room.WinnerPlayerID = opponentID
		room.EndReason = "disconnect_timeout"
		_ = m.registry.ClearCurrentRoom(playerID)
		totalShots := len(room.ShotLog)
		oppView := m.buildRoomStateViewLocked(room, opponentRole)

		slog.Info("game over via disconnect timeout", "roomId", roomID, "winnerId", opponentID, "loserId", playerID, "totalShots", totalShots)
		m.recordGameLocked(room)
		totalRooms := m.activeRoomsCountLocked()
		subscribers := m.subscriberListLocked()
		m.mu.Unlock()

		m.send(opponentID, "game.over", map[string]any{
			"winnerPlayerId": opponentID,
			"reason":         "disconnect_timeout",
			"totalShots":     totalShots,
		})
		m.send(opponentID, "room.state", oppView)
		m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
			Action:     "remove",
			RoomID:     roomID,
			TotalRooms: totalRooms,
			MaxRooms:   m.maxRooms,
		})
		return
	}

	m.mu.Unlock()
}

// expireWaitingRoom removes a waiting room when its host disconnect grace period lapses.
func (m *LobbyManager) expireWaitingRoom(roomID string, hostID string) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if !ok || room.HostPlayerID != hostID || room.Phase != PhaseWaiting {
		m.mu.Unlock()
		return
	}

	// Verify host is still disconnected before deleting
	host, ok := m.registry.GetByID(hostID)
	if ok && host.ConnectionState == StateConnected {
		m.mu.Unlock()
		return
	}

	delete(m.rooms, roomID)
	_ = m.registry.ClearCurrentRoom(hostID)

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("waiting room expired due to host disconnect timeout", "roomId", roomID, "hostId", hostID, "totalRooms", totalRooms)

	m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     roomID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})
}

// HandleReconnect cancels active disconnect timers and restores room state view for the player.
func (m *LobbyManager) HandleReconnect(playerID string) (*RoomStateView, error) {
	m.mu.Lock()
	currentRoomID, err := m.registry.GetPlayerRoom(playerID)
	if err != nil || currentRoomID == "" {
		m.mu.Unlock()
		return nil, nil
	}

	room, ok := m.rooms[currentRoomID]
	if !ok {
		_ = m.registry.ClearCurrentRoom(playerID)
		m.mu.Unlock()
		return nil, nil
	}

	if room.Phase == PhaseWaiting && room.HostPlayerID == playerID {
		if room.DisconnectTimer != nil {
			room.DisconnectTimer.Stop()
			room.DisconnectTimer = nil
		}
	}

	var opponentID string
	switch playerID {
	case room.HostPlayerID:
		if room.HostDisconnectTimer != nil {
			room.HostDisconnectTimer.Stop()
			room.HostDisconnectTimer = nil
		}
		room.HostDisconnectDeadline = time.Time{}
		opponentID = room.GuestPlayerID
	case room.GuestPlayerID:
		if room.GuestDisconnectTimer != nil {
			room.GuestDisconnectTimer.Stop()
			room.GuestDisconnectTimer = nil
		}
		room.GuestDisconnectDeadline = time.Time{}
		opponentID = room.HostPlayerID
	}

	role := "host"
	if playerID != room.HostPlayerID {
		role = "guest"
	}

	stateView := m.buildRoomStateViewLocked(room, role)

	var readyState *RoomReadyStateView
	if room.Phase == PhaseReadyUp {
		readyState = &RoomReadyStateView{
			RoomID:     room.ID,
			HostReady:  room.HostReady,
			GuestReady: room.GuestReady,
		}
	}
	m.mu.Unlock()

	slog.Info("player reconnected to room", "roomId", room.ID, "playerId", playerID, "role", role, "phase", room.Phase)

	m.send(playerID, "room.state", stateView)
	if readyState != nil {
		m.send(playerID, "room.readyState", *readyState)
	}

	// Notify connected opponent that player reconnected
	if opponentID != "" {
		m.send(opponentID, "opponent.reconnected", map[string]any{})
	}

	return &stateView, nil
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

// broadcast sends a typed payload to all specified player connections without holding m.mu.
func (m *LobbyManager) broadcast(playerIDs []string, msgType string, payload any) {
	for _, id := range playerIDs {
		m.send(id, msgType, payload)
	}
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
