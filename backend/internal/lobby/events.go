package lobby

import (
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
)

// PlacementConfirmedPayload confirms a player's locked-in fleet.
type PlacementConfirmedPayload struct {
	Confirmed bool              `json:"confirmed"`
	Ships     []game.PlacedShip `json:"ships,omitempty"`
	Auto      bool              `json:"auto,omitempty"`
}

// PlacementOpponentReadyPayload notifies a player that their opponent locked in their fleet.
type PlacementOpponentReadyPayload struct {
	RoomID string `json:"roomId"`
}

// BattleStartPayload announces the start of combat and designated first player.
type BattleStartPayload struct {
	FirstPlayerID string `json:"firstPlayerId"`
}

// BattleTurnPayload announces the active shooter and deadline for the turn.
type BattleTurnPayload struct {
	TurnPlayerID string    `json:"turnPlayerId"`
	Deadline     time.Time `json:"deadline"`
}

// BattleShotResultPayload broadcasts the outcome of a shot to both players.
type BattleShotResultPayload struct {
	By       string             `json:"by"`
	Cell     game.Coord         `json:"cell"`
	Result   game.ShotResult    `json:"result"`
	SunkShip *game.SunkShipInfo `json:"sunkShip,omitempty"`
	Auto     bool               `json:"auto"`
}

// GameOverPayload broadcasts match conclusion when a game finishes.
type GameOverPayload struct {
	WinnerPlayerID string `json:"winnerPlayerId"`
	Reason         string `json:"reason"`
	TotalShots     int    `json:"totalShots"`
}

// OpponentDisconnectedPayload notifies a player that their opponent lost connection.
type OpponentDisconnectedPayload struct {
	GraceDeadline time.Time `json:"graceDeadline"`
}

// OpponentReconnectedPayload notifies a player that their opponent reconnected.
type OpponentReconnectedPayload struct{}

// RematchOfferedPayload broadcasts a rematch offer to players.
type RematchOfferedPayload struct {
	By string `json:"by"`
}

// RematchCancelledPayload announces cancellation of a rematch offer.
type RematchCancelledPayload struct {
	Reason string `json:"reason"`
}
