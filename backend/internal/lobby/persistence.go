package lobby

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/azbagas/battleship/backend/internal/store"
)

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
		switch room.WinnerPlayerID {
		case room.HostPlayerID:
			slot = 1
		case room.GuestPlayerID:
			slot = 2
		}
		if slot != 0 {
			winnerSlot = &slot
		}
	}

	hostName := m.playerDisplayName(room.HostPlayerID, "Host Commander")
	guestName := m.playerDisplayName(room.GuestPlayerID, "Guest Commander")

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
