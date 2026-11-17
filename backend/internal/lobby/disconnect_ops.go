package lobby

import (
	"log/slog"
	"time"
)

// HandleDisconnect manages lobby subscriber cleanup and waiting room / match grace timers.
func (m *LobbyManager) HandleDisconnect(playerID string) {
	m.mu.Lock()
	delete(m.lobbySubscribers, playerID)

	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		m.mu.Unlock()
		return
	}

	switch room.Phase {
	case PhaseWaiting:
		if room.HostPlayerID == playerID {
			roomID := room.ID
			slog.Info("waiting room host disconnected, starting grace timer", "roomId", roomID, "hostId", playerID, "graceDuration", m.disconnectGrace.String())
			room.DisconnectTimer = m.clock.AfterFunc(m.disconnectGrace, func() {
				m.expireWaitingRoom(roomID, playerID)
			})
		}
		m.mu.Unlock()

	case PhaseReadyUp, PhasePlacement, PhaseBattle:
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

		if opponentID != "" {
			m.send(opponentID, "opponent.disconnected", OpponentDisconnectedPayload{
				GraceDeadline: deadline,
			})
		}

	case PhaseFinished:
		_, opponentID, _ := room.roleAndOpponent(playerID)

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
			m.send(opponentID, "rematch.cancelled", RematchCancelledPayload{Reason: "disconnected"})
		}

		if roomDeleted {
			m.broadcastLobbyRemove(subscribers, roomID, totalRooms)
		}

	default:
		m.mu.Unlock()
	}
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
	if p, ok := m.registry.GetByID(playerID); ok && p.ConnectionState == StateConnected {
		m.mu.Unlock()
		return
	}

	switch room.Phase {
	case PhaseReadyUp:
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

	case PhasePlacement, PhaseBattle:
		opponentID := room.GuestPlayerID
		opponentRole := "guest"
		if !isHost {
			opponentID = room.HostPlayerID
			opponentRole = "host"
		}

		room.stopMatchTimers()

		opp, oppOk := m.registry.GetByID(opponentID)
		oppDisconnected := !oppOk || opp.ConnectionState == StateDisconnected

		if oppDisconnected {
			// PRD F-06: records match as abandoned with no winner if both players are disconnected when grace expires
			room.Phase = PhaseFinished
			room.WinnerPlayerID = ""
			room.EndReason = endReasonAbandoned
			slog.Info("match abandoned: both players disconnected when grace period expired", "roomId", roomID)
			m.recordGameLocked(room)
			delete(m.rooms, roomID)
			_ = m.registry.ClearCurrentRoom(playerID)
			_ = m.registry.ClearCurrentRoom(opponentID)
			totalRooms := m.activeRoomsCountLocked()
			subscribers := m.subscriberListLocked()
			m.mu.Unlock()
			m.broadcastLobbyRemove(subscribers, roomID, totalRooms)
			return
		}

		room.Phase = PhaseFinished
		room.WinnerPlayerID = opponentID
		room.EndReason = endReasonDisconnectTimeout
		_ = m.registry.ClearCurrentRoom(playerID)
		totalShots := len(room.ShotLog)
		oppView := m.buildRoomStateViewLocked(room, opponentRole)

		slog.Info("game over via disconnect timeout", "roomId", roomID, "winnerId", opponentID, "loserId", playerID, "totalShots", totalShots)
		m.recordGameLocked(room)
		totalRooms := m.activeRoomsCountLocked()
		subscribers := m.subscriberListLocked()
		m.mu.Unlock()

		m.send(opponentID, "game.over", GameOverPayload{
			WinnerPlayerID: opponentID,
			Reason:         endReasonDisconnectTimeout,
			TotalShots:     totalShots,
		})
		m.send(opponentID, "room.state", oppView)
		m.broadcastLobbyRemove(subscribers, roomID, totalRooms)

	default:
		m.mu.Unlock()
	}
}

// expireWaitingRoom removes a waiting room when its host disconnect grace period lapses.
func (m *LobbyManager) expireWaitingRoom(roomID string, hostID string) {
	m.mu.Lock()
	room, ok := m.rooms[roomID]
	if !ok || room.HostPlayerID != hostID || room.Phase != PhaseWaiting {
		m.mu.Unlock()
		return
	}

	if host, ok := m.registry.GetByID(hostID); ok && host.ConnectionState == StateConnected {
		m.mu.Unlock()
		return
	}

	delete(m.rooms, roomID)
	_ = m.registry.ClearCurrentRoom(hostID)

	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	m.mu.Unlock()

	slog.Info("waiting room expired due to host disconnect timeout", "roomId", roomID, "hostId", hostID, "totalRooms", totalRooms)
	m.broadcastLobbyRemove(subscribers, roomID, totalRooms)
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

	if opponentID != "" {
		m.send(opponentID, "opponent.reconnected", OpponentReconnectedPayload{})
	}

	return &stateView, nil
}
