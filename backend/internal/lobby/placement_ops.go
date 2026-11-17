package lobby

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
)

// schedulePlacementTimerLocked sets PlacementDeadline and arms the 60s placement timeout timer.
// Caller must hold m.mu.
func (m *LobbyManager) schedulePlacementTimerLocked(room *Room) time.Time {
	deadline := m.clock.Now().Add(placementTimeout)
	room.PlacementDeadline = deadline
	roomID := room.ID
	room.PlacementTimer = m.clock.AfterFunc(placementTimeout, func() {
		m.expirePlacementTimer(roomID)
	})
	return deadline
}

// UpdateDraftFleet saves a player's unconfirmed partial or complete ship placements (0..5 ships)
// during PhasePlacement without locking the fleet, so timeout auto-placement and reconnects preserve them.
func (m *LobbyManager) UpdateDraftFleet(playerID string, ships []game.PlacedShip) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		return err
	}

	if room.Phase != PhasePlacement {
		return ErrNotInPlacement
	}

	role, _, ok := room.roleAndOpponent(playerID)
	if !ok {
		return ErrNotInRoom
	}

	isHost := role == "host"
	if (isHost && room.HostFleetConfirmed) || (!isHost && room.GuestFleetConfirmed) {
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
	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		m.mu.Unlock()
		slog.Warn("submit fleet rejected", "playerId", playerID, "error", err)
		return err
	}

	if room.Phase != PhasePlacement {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: not in placement phase", "playerId", playerID, "roomId", room.ID, "phase", phase)
		return ErrNotInPlacement
	}

	role, opponentID, ok := room.roleAndOpponent(playerID)
	if !ok {
		m.mu.Unlock()
		return ErrNotInRoom
	}

	isHost := role == "host"
	if (isHost && room.HostFleetConfirmed) || (!isHost && room.GuestFleetConfirmed) {
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: fleet already locked", "playerId", playerID, "roomId", room.ID)
		return ErrFleetAlreadyLocked
	}

	if err := game.ValidateFleet(ships); err != nil {
		m.mu.Unlock()
		slog.Warn("submit fleet rejected: invalid fleet", "playerId", playerID, "roomId", room.ID, "error", err)
		return fmt.Errorf("%w: %w", ErrInvalidPlacement, err)
	}

	copied := make([]game.PlacedShip, len(ships))
	copy(copied, ships)

	if isHost {
		room.HostFleet = copied
		room.HostFleetConfirmed = true
	} else {
		room.GuestFleet = copied
		room.GuestFleetConfirmed = true
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

	m.send(playerID, "placement.confirmed", PlacementConfirmedPayload{
		Confirmed: true,
		Ships:     copied,
	})

	if opponentID != "" {
		m.send(opponentID, "placement.opponentReady", PlacementOpponentReadyPayload{
			RoomID: roomID,
		})
	}

	if transitionToBattle {
		m.broadcastBattleStart(hostID, guestID, firstPlayerID, turnDeadline, hostBattleState, guestBattleState)
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

	rng := m.rngOrCrypto()
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

	if hostAutoPlaced {
		m.send(hostID, "placement.confirmed", PlacementConfirmedPayload{
			Confirmed: true,
			Ships:     hostFleet,
			Auto:      true,
		})
	}
	if guestAutoPlaced && guestID != "" {
		m.send(guestID, "placement.confirmed", PlacementConfirmedPayload{
			Confirmed: true,
			Ships:     guestFleet,
			Auto:      true,
		})
	}

	m.sendToRoom(hostID, guestID, "placement.opponentReady", PlacementOpponentReadyPayload{RoomID: roomID})
	m.broadcastBattleStart(hostID, guestID, firstPlayerID, turnDeadline, hostBattleState, guestBattleState)
}

func (m *LobbyManager) broadcastBattleStart(hostID, guestID, firstPlayerID string, turnDeadline time.Time, hostView, guestView RoomStateView) {
	m.sendToRoom(hostID, guestID, "battle.start", BattleStartPayload{FirstPlayerID: firstPlayerID})
	m.sendToRoom(hostID, guestID, "battle.turn", BattleTurnPayload{
		TurnPlayerID: firstPlayerID,
		Deadline:     turnDeadline,
	})
	m.send(hostID, "room.state", hostView)
	if guestID != "" {
		m.send(guestID, "room.state", guestView)
	}
}
