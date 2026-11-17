package lobby

import (
	"errors"
	"log/slog"
)

// ReadyRoom marks a player as ready in their active ready-up room.
// When both players are ready, the room automatically transitions to placement.
func (m *LobbyManager) ReadyRoom(playerID string) error {
	return m.setReady(playerID, true)
}

// UnreadyRoom clears a player's ready state in their active ready-up room.
func (m *LobbyManager) UnreadyRoom(playerID string) error {
	return m.setReady(playerID, false)
}

func (m *LobbyManager) setReady(playerID string, ready bool) error {
	action := "ready"
	if !ready {
		action = "unready"
	}

	m.mu.Lock()
	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		m.mu.Unlock()
		slog.Warn(action+" action rejected", "playerId", playerID, "error", err)
		return err
	}

	if room.Phase != PhaseReadyUp {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn(action+" action rejected: not in ready-up phase", "playerId", playerID, "roomId", room.ID, "phase", phase)
		return ErrNotInReadyUp
	}

	switch playerID {
	case room.HostPlayerID:
		room.HostReady = ready
	case room.GuestPlayerID:
		room.GuestReady = ready
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

	slog.Info("player updated ready state", "roomId", room.ID, "playerId", playerID, "hostReady", room.HostReady, "guestReady", room.GuestReady)

	transitionToPlacement := ready && room.HostReady && room.GuestReady
	var hostPlacementState, guestPlacementState RoomStateView
	if transitionToPlacement {
		room.Phase = PhasePlacement
		if room.GameID == "" {
			room.GameID = generateGameID()
		}
		if room.StartedAt.IsZero() {
			room.StartedAt = m.clock.Now()
		}
		placementDeadline := m.schedulePlacementTimerLocked(room)
		hostPlacementState = m.buildRoomStateViewLocked(room, "host")
		guestPlacementState = m.buildRoomStateViewLocked(room, "guest")
		slog.Info("both players ready, room transitioned to placement", "roomId", room.ID, "hostId", hostID, "guestId", guestID, "deadline", placementDeadline)
	}

	m.mu.Unlock()

	m.sendToRoom(hostID, guestID, "room.readyState", readyState)
	if transitionToPlacement {
		m.send(hostID, "room.state", hostPlacementState)
		if guestID != "" {
			m.send(guestID, "room.state", guestPlacementState)
		}
	}

	return nil
}

// LeaveRoom handles explicit departures across any room phase:
// - waiting (host): cancels the waiting room
// - ready-up (guest): returns the room to waiting lobby for the host
// - ready-up (host): closes the room and returns the guest to lobby
// - placement/battle: forfeits the match in favor of the opponent
// - finished: returns player to lobby and deletes the room once both have left
func (m *LobbyManager) LeaveRoom(playerID string) error {
	m.mu.Lock()
	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			_ = m.registry.ClearCurrentRoom(playerID)
		}
		m.mu.Unlock()
		slog.Warn("leave action rejected", "playerId", playerID, "error", err)
		return err
	}

	switch room.Phase {
	case PhaseWaiting:
		m.mu.Unlock()
		return m.CancelRoom(playerID)
	case PhaseReadyUp:
		return m.leaveReadyUpLocked(room, playerID)
	case PhasePlacement, PhaseBattle:
		return m.leaveMatchLocked(room, playerID)
	case PhaseFinished:
		return m.leaveFinishedLocked(room, playerID)
	default:
		m.mu.Unlock()
		return ErrNotInRoom
	}
}

// leaveReadyUpLocked executes departure rules for a ready-up room with m.mu held.
func (m *LobbyManager) leaveReadyUpLocked(room *Room, playerID string) error {
	role, _, ok := room.roleAndOpponent(playerID)
	if !ok {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	if role == "guest" {
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

		_ = m.registry.ClearCurrentRoom(guestID)
		m.lobbySubscribers[guestID] = struct{}{}

		// If host had disconnected while guest was in room, ensure host disconnect timer is running
		if host, found := m.registry.GetByID(hostID); found && host.ConnectionState == StateDisconnected {
			if room.HostDisconnectTimer == nil && room.DisconnectTimer == nil {
				roomID := room.ID
				room.DisconnectTimer = m.clock.AfterFunc(m.disconnectGrace, func() {
					m.expireWaitingRoom(roomID, hostID)
				})
			}
		}

		hostName := m.playerDisplayName(hostID, "Host")
		waitingView := WaitingRoomView{
			ID:        room.ID,
			HostName:  hostName,
			CreatedAt: room.CreatedAt,
		}
		hostState := m.buildRoomStateViewLocked(room, "host")

		totalRooms := m.activeRoomsCountLocked()
		subscribers := m.subscriberListLocked()
		m.mu.Unlock()

		slog.Info("guest left ready-up, room returned to waiting lobby", "roomId", room.ID, "hostId", hostID, "guestId", guestID, "totalRooms", totalRooms)

		m.send(hostID, "room.state", hostState)
		m.send(guestID, "lobby.snapshot", m.GetLobbySnapshot())
		m.broadcast(subscribers, "lobby.update", LobbyUpdatePayload{
			Action:     "upsert",
			Room:       &waitingView,
			TotalRooms: totalRooms,
			MaxRooms:   m.maxRooms,
		})

		return nil
	}

	// Host left: close the room
	room.stopAllTimers()

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

	snap := m.GetLobbySnapshot()
	m.send(hostID, "lobby.snapshot", snap)
	if guestID != "" {
		m.send(guestID, "lobby.snapshot", snap)
	}

	m.broadcastLobbyRemove(subscribers, roomID, totalRooms, hostID, guestID)
	return nil
}

// leaveMatchLocked forfeits an in-progress match during PhasePlacement or PhaseBattle with m.mu held.
func (m *LobbyManager) leaveMatchLocked(room *Room, playerID string) error {
	role, opponentID, ok := room.roleAndOpponent(playerID)
	if !ok {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	opponentRole := "host"
	if role == "host" {
		opponentRole = "guest"
	}

	room.stopAllTimers()

	room.Phase = PhaseFinished
	room.WinnerPlayerID = opponentID
	room.EndReason = endReasonForfeit
	m.recordGameLocked(room)

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

	slog.Info("player forfeited match", "roomId", roomID, "playerId", playerID, "opponentId", opponentID, "totalShots", totalShots)
	m.mu.Unlock()

	m.send(playerID, "lobby.snapshot", m.GetLobbySnapshot())

	if opponentID != "" {
		m.send(opponentID, "game.over", GameOverPayload{
			WinnerPlayerID: opponentID,
			Reason:         endReasonForfeit,
			TotalShots:     totalShots,
		})
		m.send(opponentID, "room.state", opponentView)
	}

	m.broadcastLobbyRemove(subscribers, roomID, totalRooms, playerID)
	return nil
}

// leaveFinishedLocked handles departure from a finished match, returning player to lobby with m.mu held.
func (m *LobbyManager) leaveFinishedLocked(room *Room, playerID string) error {
	_, otherID, ok := room.roleAndOpponent(playerID)
	if !ok {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	_ = m.registry.ClearCurrentRoom(playerID)
	m.lobbySubscribers[playerID] = struct{}{}

	otherInRoom := false
	if otherID != "" {
		if otherRoom, err := m.registry.GetPlayerRoom(otherID); err == nil && otherRoom == room.ID {
			otherInRoom = true
		}
	}

	notifyOtherRematchCancelled := false
	if otherInRoom && otherID != "" && (room.HostRematch || room.GuestRematch) {
		room.HostRematch = false
		room.GuestRematch = false
		notifyOtherRematchCancelled = true
	}

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
		m.send(otherID, "rematch.cancelled", RematchCancelledPayload{Reason: "left"})
	}

	m.send(playerID, "lobby.snapshot", m.GetLobbySnapshot())

	if roomDeleted {
		m.broadcastLobbyRemove(subscribers, roomID, totalRooms)
	}

	return nil
}
