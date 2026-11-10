package lobby

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"sort"
	"sync"
	"time"
)

// Sentinel errors for lobby operations.
var (
	ErrRoomCapReached    = errors.New("maximum room capacity reached (10 rooms)")
	ErrAlreadyInRoom     = errors.New("player already has an active room or game")
	ErrRoomNotFound      = errors.New("room not found")
	ErrRoomNotAvailable  = errors.New("game is no longer available")
	ErrNotInWaitingRoom  = errors.New("player is not hosting a waiting room")
	ErrCannotJoinOwnRoom = errors.New("cannot join your own room")
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

// GetLobbySnapshot generates the current waiting rooms snapshot sorted oldest first.
func (m *LobbyManager) GetLobbySnapshot() LobbySnapshotPayload {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var waitingRooms []WaitingRoomView
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
		TotalRooms: len(m.rooms),
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
	if len(m.rooms) >= m.maxRooms {
		total := len(m.rooms)
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

	totalRooms := len(m.rooms)
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

	totalRooms := len(m.rooms)
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
	if room.DisconnectTimer != nil {
		room.DisconnectTimer.Stop()
		room.DisconnectTimer = nil
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
		CreatedAt:    room.CreatedAt,
	}

	guestState := RoomStateView{
		RoomID:       room.ID,
		Phase:        PhaseReadyUp,
		Role:         "guest",
		OpponentName: hostName,
		Host:         hostView,
		Guest:        &guestView,
		CreatedAt:    room.CreatedAt,
	}

	totalRooms := len(m.rooms)
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("player joined room", "roomId", room.ID, "hostId", room.HostPlayerID, "hostName", hostName, "guestId", playerID, "guestName", guest.DisplayName, "phase", room.Phase)

	// Notify both participants
	m.send(room.HostPlayerID, "room.state", hostState)
	m.send(playerID, "room.state", guestState)

	// Broadcast removal from public waiting list to lobby browsers
	m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
		Action:     "remove",
		RoomID:     room.ID,
		TotalRooms: totalRooms,
		MaxRooms:   m.maxRooms,
	})

	return room, nil
}

// HandleDisconnect manages lobby subscriber cleanup and waiting room host grace timers.
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

	totalRooms := len(m.rooms)
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

// HandleReconnect cancels active disconnect timers and restores waiting room view for host.
func (m *LobbyManager) HandleReconnect(playerID string) (*Room, error) {
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

	host, _ := m.registry.GetByID(room.HostPlayerID)
	hostName := "Host"
	if host != nil {
		hostName = host.DisplayName
	}

	role := "host"
	opponentName := ""
	if playerID != room.HostPlayerID {
		role = "guest"
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
			if me, ok := m.registry.GetByID(playerID); ok {
				gName = me.DisplayName
			}
		}
		guestView = &RoomPlayerView{ID: room.GuestPlayerID, Name: gName}
	}

	stateView := RoomStateView{
		RoomID:       room.ID,
		Phase:        room.Phase,
		Role:         role,
		OpponentName: opponentName,
		Host:         RoomPlayerView{ID: room.HostPlayerID, Name: hostName},
		Guest:        guestView,
		CreatedAt:    room.CreatedAt,
	}
	m.mu.Unlock()

	slog.Info("player reconnected to room", "roomId", room.ID, "playerId", playerID, "role", role, "phase", room.Phase)

	m.send(playerID, "room.state", stateView)
	return room, nil
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

// TotalRooms returns the total number of registered rooms (waiting and active).
func (m *LobbyManager) TotalRooms() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.rooms)
}
