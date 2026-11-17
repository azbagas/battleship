package lobby

import (
	"errors"
	"log/slog"
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
)

type shotDispatch struct {
	roomID       string
	hostID       string
	guestID      string
	shooterID    string
	cell         game.Coord
	outcome      game.ShotOutcome
	auto         bool
	isWon        bool
	winnerID     string
	endReason    string
	totalShots   int
	nextTurnID   string
	nextDeadline time.Time
	hostView     RoomStateView
	guestView    RoomStateView
	subscribers  []string
	totalRooms   int
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

	firstPlayerID := game.ChooseFirstPlayer(room.HostPlayerID, room.GuestPlayerID, m.rngOrCrypto())
	room.TurnPlayerID = firstPlayerID
	turnDeadline := m.scheduleTurnTimerLocked(room)

	hostView := m.buildRoomStateViewLocked(room, "host")
	guestView := m.buildRoomStateViewLocked(room, "guest")

	slog.Info("battle phase initialized", "roomId", room.ID, "firstPlayerId", firstPlayerID, "turnDeadline", turnDeadline)
	return firstPlayerID, turnDeadline, hostView, guestView
}

func (m *LobbyManager) scheduleTurnTimerLocked(room *Room) time.Time {
	deadline := m.clock.Now().Add(turnTimeout)
	room.TurnDeadline = deadline
	roomID := room.ID
	room.TurnTimer = m.clock.AfterFunc(turnTimeout, func() {
		m.expireTurnTimer(roomID)
	})
	return deadline
}

// FireShot executes a shot action by an active player against the opponent grid during PhaseBattle.
func (m *LobbyManager) FireShot(playerID string, cell game.Coord) error {
	m.mu.Lock()
	room, err := m.getPlayerRoomLocked(playerID)
	if err != nil {
		m.mu.Unlock()
		slog.Warn("fire shot rejected", "playerId", playerID, "error", err)
		return err
	}

	if room.Phase == PhaseFinished {
		m.mu.Unlock()
		slog.Warn("fire shot rejected: game already finished", "playerId", playerID, "roomId", room.ID)
		return ErrGameOver
	}

	if room.Phase != PhaseBattle {
		phase := room.Phase
		m.mu.Unlock()
		slog.Warn("fire shot rejected: not in battle phase", "playerId", playerID, "roomId", room.ID, "phase", phase)
		return ErrNotInBattle
	}

	if room.TurnPlayerID != playerID {
		turnID := room.TurnPlayerID
		m.mu.Unlock()
		slog.Warn("fire shot rejected: not your turn", "playerId", playerID, "turnPlayerId", turnID)
		return ErrNotYourTurn
	}

	targetBoard, opponentID := targetBoardForShooter(room, playerID)
	if targetBoard == nil {
		m.mu.Unlock()
		return errors.New("target board not initialized")
	}

	dispatch, err := m.executeShotLocked(room, playerID, targetBoard, opponentID, cell, false)
	m.mu.Unlock()
	if err != nil {
		slog.Warn("fire shot rejected: invalid shot", "playerId", playerID, "cell", cell, "error", err)
		return err
	}

	m.dispatchShotOutcome(dispatch)
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
	targetBoard, opponentID := targetBoardForShooter(room, activePlayerID)
	if targetBoard == nil {
		m.mu.Unlock()
		return
	}

	cell, err := game.RandomUntargetedCell(targetBoard, m.rngOrCrypto())
	if err != nil {
		m.mu.Unlock()
		slog.Error("failed to find untargeted cell for auto-shot", "roomId", roomID, "error", err)
		return
	}

	dispatch, err := m.executeShotLocked(room, activePlayerID, targetBoard, opponentID, cell, true)
	m.mu.Unlock()
	if err != nil {
		slog.Error("failed to execute auto-shot", "roomId", roomID, "cell", cell, "error", err)
		return
	}

	m.dispatchShotOutcome(dispatch)
}

func targetBoardForShooter(room *Room, shooterID string) (*game.Board, string) {
	if shooterID == room.HostPlayerID {
		return room.GuestBoard, room.GuestPlayerID
	}
	return room.HostBoard, room.HostPlayerID
}

// executeShotLocked applies a manual or automatic shot, updates turn/win state, and captures views.
// Caller must hold m.mu.
func (m *LobbyManager) executeShotLocked(
	room *Room,
	shooterID string,
	targetBoard *game.Board,
	opponentID string,
	cell game.Coord,
	auto bool,
) (shotDispatch, error) {
	outcome, err := targetBoard.FireShot(cell)
	if err != nil {
		return shotDispatch{}, err
	}

	if room.TurnTimer != nil {
		room.TurnTimer.Stop()
		room.TurnTimer = nil
	}

	record := ShotRecord{
		ShooterID: shooterID,
		Cell:      cell,
		Result:    outcome.Result,
		SunkShip:  outcome.SunkShip,
		Auto:      auto,
		Timestamp: m.clock.Now(),
	}
	room.ShotLog = append(room.ShotLog, record)

	slog.Info("shot executed", "roomId", room.ID, "shooterId", shooterID, "cell", cell.String(), "result", outcome.Result, "auto", auto)

	dispatch := shotDispatch{
		roomID:    room.ID,
		hostID:    room.HostPlayerID,
		guestID:   room.GuestPlayerID,
		shooterID: shooterID,
		cell:      cell,
		outcome:   outcome,
		auto:      auto,
		isWon:     targetBoard.AllShipsSunk(),
	}

	if dispatch.isWon {
		room.Phase = PhaseFinished
		room.WinnerPlayerID = shooterID
		room.EndReason = endReasonAllSunk
		dispatch.winnerID = shooterID
		dispatch.endReason = endReasonAllSunk
		slog.Info("game over: all ships sunk", "roomId", room.ID, "winnerId", shooterID, "totalShots", len(room.ShotLog), "auto", auto)
		m.recordGameLocked(room)
		dispatch.subscribers = m.subscriberListLocked()
		dispatch.totalRooms = m.activeRoomsCountLocked()
	} else {
		room.TurnPlayerID = opponentID
		dispatch.nextTurnID = opponentID
		dispatch.nextDeadline = m.scheduleTurnTimerLocked(room)
	}

	dispatch.hostView = m.buildRoomStateViewLocked(room, "host")
	dispatch.guestView = m.buildRoomStateViewLocked(room, "guest")
	dispatch.totalShots = len(room.ShotLog)

	return dispatch, nil
}

func (m *LobbyManager) dispatchShotOutcome(d shotDispatch) {
	m.sendToRoom(d.hostID, d.guestID, "battle.shotResult", BattleShotResultPayload{
		By:       d.shooterID,
		Cell:     d.cell,
		Result:   d.outcome.Result,
		SunkShip: d.outcome.SunkShip,
		Auto:     d.auto,
	})

	if d.isWon {
		m.sendToRoom(d.hostID, d.guestID, "game.over", GameOverPayload{
			WinnerPlayerID: d.winnerID,
			Reason:         d.endReason,
			TotalShots:     d.totalShots,
		})
		m.broadcastLobbyRemove(d.subscribers, d.roomID, d.totalRooms)
	} else {
		m.sendToRoom(d.hostID, d.guestID, "battle.turn", BattleTurnPayload{
			TurnPlayerID: d.nextTurnID,
			Deadline:     d.nextDeadline,
		})
	}

	m.send(d.hostID, "room.state", d.hostView)
	if d.guestID != "" {
		m.send(d.guestID, "room.state", d.guestView)
	}
}
