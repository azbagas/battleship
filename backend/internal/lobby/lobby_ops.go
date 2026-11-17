package lobby

import (
	"log/slog"
	"sort"
)

// GetLobbySnapshot generates the current waiting rooms snapshot sorted oldest first.
func (m *LobbyManager) GetLobbySnapshot() LobbySnapshotPayload {
	m.mu.RLock()
	defer m.mu.RUnlock()

	waitingRooms := make([]WaitingRoomView, 0)
	for _, room := range m.rooms {
		if room.Phase == PhaseWaiting {
			waitingRooms = append(waitingRooms, WaitingRoomView{
				ID:        room.ID,
				HostName:  m.playerDisplayName(room.HostPlayerID, "Host"),
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
	if total := m.activeRoomsCountLocked(); total >= m.maxRooms {
		m.mu.Unlock()
		slog.Warn("room creation rejected: room cap reached", "playerId", playerID, "totalRooms", total, "maxRooms", m.maxRooms)
		return nil, ErrRoomCapReached
	}

	if currentRoomID, err := m.registry.GetPlayerRoom(playerID); err == nil && currentRoomID != "" {
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

	m.send(playerID, "room.state", stateView)
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

	room.stopDisconnectTimers()

	delete(m.rooms, room.ID)
	_ = m.registry.ClearCurrentRoom(playerID)
	m.lobbySubscribers[playerID] = struct{}{}

	totalRooms := m.activeRoomsCountLocked()
	removedRoomID := room.ID
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("room cancelled", "roomId", removedRoomID, "hostId", playerID, "totalRooms", totalRooms)

	m.send(playerID, "lobby.snapshot", m.GetLobbySnapshot())
	m.broadcastLobbyRemove(subscribers, removedRoomID, totalRooms)

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

	if currentRoomID, err := m.registry.GetPlayerRoom(playerID); err == nil && currentRoomID != "" {
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
	room.stopAllTimers()

	_ = m.registry.SetCurrentRoom(playerID, roomID)
	delete(m.lobbySubscribers, playerID)

	hostName := m.playerDisplayName(room.HostPlayerID, "Host")
	hostState := m.buildRoomStateViewLocked(room, "host")
	guestState := m.buildRoomStateViewLocked(room, "guest")

	readyState := RoomReadyStateView{
		RoomID:     room.ID,
		HostReady:  false,
		GuestReady: false,
	}

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	hostID := room.HostPlayerID
	m.mu.Unlock()

	slog.Info("player joined room", "roomId", room.ID, "hostId", hostID, "hostName", hostName, "guestId", playerID, "guestName", guest.DisplayName, "phase", PhaseReadyUp)

	m.send(hostID, "room.state", hostState)
	m.send(hostID, "room.readyState", readyState)
	m.send(playerID, "room.state", guestState)
	m.send(playerID, "room.readyState", readyState)

	m.broadcastLobbyRemove(subscribers, room.ID, totalRooms)

	return room, nil
}
