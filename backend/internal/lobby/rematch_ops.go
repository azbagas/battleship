package lobby

import "log/slog"

// AcceptRematch registers a player's acceptance of a rematch offer.
// If both players mutually accept, the room restarts directly into PhasePlacement with re-randomized first turn on battle start.
func (m *LobbyManager) AcceptRematch(playerID string) error {
	m.mu.Lock()
	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	if room.Phase != PhaseFinished {
		m.mu.Unlock()
		return ErrNotInFinished
	}

	if room.EndReason != endReasonAllSunk {
		m.mu.Unlock()
		return ErrRematchNotAllowed
	}

	role, opponentID, ok := room.roleAndOpponent(playerID)
	if !ok {
		m.mu.Unlock()
		return ErrNotInRoom
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

	if role == "host" {
		room.HostRematch = true
	} else {
		room.GuestRematch = true
	}

	slog.Info("rematch offer accepted by player", "roomId", room.ID, "playerId", playerID, "hostRematch", room.HostRematch, "guestRematch", room.GuestRematch)

	if !(room.HostRematch && room.GuestRematch) {
		m.mu.Unlock()
		m.sendToRoom(opponentID, playerID, "rematch.offered", RematchOfferedPayload{By: playerID})
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
	room.RematchCancelReason = ""
	room.GameID = generateGameID()
	room.StartedAt = m.clock.Now()
	room.Recorded = false

	placementDeadline := m.schedulePlacementTimerLocked(room)
	hostView := m.buildRoomStateViewLocked(room, "host")
	guestView := m.buildRoomStateViewLocked(room, "guest")
	totalRooms := m.activeRoomsCountLocked()
	subscribers := m.subscriberListLocked()
	hostID := room.HostPlayerID
	guestID := room.GuestPlayerID
	roomID := room.ID

	slog.Info("mutual rematch confirmed, room restarting into placement", "roomId", roomID, "newGameId", room.GameID, "deadline", placementDeadline)
	m.mu.Unlock()

	m.send(hostID, "room.state", hostView)
	m.send(guestID, "room.state", guestView)
	m.broadcastLobbyRemove(subscribers, roomID, totalRooms)
	return nil
}

// DeclineRematch explicitly declines a pending rematch offer and cancels it for both players.
func (m *LobbyManager) DeclineRematch(playerID string) error {
	m.mu.Lock()
	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		m.mu.Unlock()
		return err
	}

	if room.Phase != PhaseFinished {
		m.mu.Unlock()
		return ErrNotInFinished
	}

	_, opponentID, ok := room.roleAndOpponent(playerID)
	if !ok {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	room.HostRematch = false
	room.GuestRematch = false
	room.RematchCancelReason = "declined"

	otherInRoom := false
	if opponentID != "" {
		if oppRoom, err := m.registry.GetPlayerRoom(opponentID); err == nil && oppRoom == room.ID {
			otherInRoom = true
		}
	}

	slog.Info("rematch declined", "roomId", room.ID, "playerId", playerID)
	m.mu.Unlock()

	cancelled := RematchCancelledPayload{Reason: "declined"}
	if otherInRoom {
		m.send(opponentID, "rematch.cancelled", cancelled)
	}
	m.send(playerID, "rematch.cancelled", cancelled)
	return nil
}
