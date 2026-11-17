package ws

import (
	"encoding/json"

	"github.com/azbagas/battleship/backend/internal/game"
	"github.com/azbagas/battleship/backend/internal/lobby"
)

// MessageType represents the domain-action identifier of a WebSocket message.
type MessageType string

const (
	TypeHello                  MessageType = "hello"
	TypeHelloOK                MessageType = "hello.ok"
	TypeError                  MessageType = "error"
	TypeLobbyCreate            MessageType = "lobby.create"
	TypeLobbyCancel            MessageType = "lobby.cancel"
	TypeLobbyJoin              MessageType = "lobby.join"
	TypeLobbySnapshot          MessageType = "lobby.snapshot"
	TypeLobbyUpdate            MessageType = "lobby.update"
	TypeRoomState              MessageType = "room.state"
	TypeRoomReady              MessageType = "room.ready"
	TypeRoomUnready            MessageType = "room.unready"
	TypeRoomLeave              MessageType = "room.leave"
	TypeGameLeave              MessageType = "game.leave"
	TypeRoomReadyState         MessageType = "room.readyState"
	TypePlacementSubmit        MessageType = "placement.submit"
	TypePlacementReady         MessageType = "placement.ready"
	TypePlacementOpponentReady MessageType = "placement.opponentReady"
	TypePlacementConfirmed     MessageType = "placement.confirmed"
	TypeBattleFire             MessageType = "battle.fire"
	TypeBattleStart            MessageType = "battle.start"
	TypeBattleShotResult       MessageType = "battle.shotResult"
	TypeBattleTurn             MessageType = "battle.turn"
	TypeGameOver               MessageType = "game.over"
	TypeOpponentDisconnected   MessageType = "opponent.disconnected"
	TypeOpponentReconnected    MessageType = "opponent.reconnected"
	TypeRematchAccept          MessageType = "rematch.accept"
	TypeRematchDecline         MessageType = "rematch.decline"
	TypeRematchOffered         MessageType = "rematch.offered"
	TypeRematchCancelled       MessageType = "rematch.cancelled"
)

// InboundMessage represents a received client JSON envelope.
type InboundMessage struct {
	Type    MessageType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// OutboundMessage represents a typed server JSON envelope sent to clients.
type OutboundMessage struct {
	Type    MessageType `json:"type"`
	Payload any         `json:"payload"`
}

// HelloPayload contains the client's handshake parameters.
type HelloPayload struct {
	Name         string `json:"name"`
	SessionToken string `json:"sessionToken,omitempty"`
}

// HelloOKPayload is sent to the client upon a successful handshake.
type HelloOKPayload struct {
	PlayerID      string `json:"playerId"`
	SessionToken  string `json:"sessionToken"`
	DisplayName   string `json:"displayName"`
	RestoredState any    `json:"restoredState"`
}

// ErrorPayload describes a structured error returned to the client.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// LobbyJoinPayload contains the target room ID when joining a room.
type LobbyJoinPayload struct {
	RoomID string `json:"roomId"`
}

// PlacementSubmitPayload contains a player's proposed ship arrangement.
// When Draft is true, partial placements (0..5 ships) are saved without locking the fleet.
type PlacementSubmitPayload struct {
	Ships []game.PlacedShip `json:"ships"`
	Draft bool              `json:"draft,omitempty"`
}

// PlacementConfirmedPayload confirms a player's locked-in fleet.
type PlacementConfirmedPayload = lobby.PlacementConfirmedPayload

// PlacementOpponentReadyPayload notifies a player that their opponent locked in their fleet.
type PlacementOpponentReadyPayload = lobby.PlacementOpponentReadyPayload

// BattleFirePayload represents a player's intent to fire at a target cell.
type BattleFirePayload struct {
	Cell game.Coord `json:"cell"`
}

// BattleStartPayload announces the start of combat and designated first player.
type BattleStartPayload = lobby.BattleStartPayload

// BattleShotResultPayload broadcasts the outcome of a shot to both players.
type BattleShotResultPayload = lobby.BattleShotResultPayload

// BattleTurnPayload announces the active shooter and deadline for the turn.
type BattleTurnPayload = lobby.BattleTurnPayload

// GameOverPayload broadcasts match conclusion when all 5 vessels of a fleet are sunk.
type GameOverPayload = lobby.GameOverPayload

// OpponentDisconnectedPayload notifies a player that their opponent lost connection.
type OpponentDisconnectedPayload = lobby.OpponentDisconnectedPayload

// OpponentReconnectedPayload notifies a player that their opponent reconnected.
type OpponentReconnectedPayload = lobby.OpponentReconnectedPayload

// RematchOfferedPayload broadcasts a rematch offer to players.
type RematchOfferedPayload = lobby.RematchOfferedPayload

// RematchCancelledPayload announces cancellation of a rematch offer.
type RematchCancelledPayload = lobby.RematchCancelledPayload

// Standard protocol error codes.
const (
	ErrCodeInvalidHandshake     = "INVALID_HANDSHAKE"
	ErrCodeInvalidName          = "INVALID_NAME"
	ErrCodeSessionAlreadyActive = "SESSION_ALREADY_ACTIVE"
	ErrCodeBadRequest           = "BAD_REQUEST"
	ErrCodeRoomCapReached       = "ROOM_CAP_REACHED"
	ErrCodeAlreadyInRoom        = "ALREADY_IN_ROOM"
	ErrCodeRoomNotFound         = "ROOM_NOT_FOUND"
	ErrCodeRoomNotAvailable     = "ROOM_NOT_AVAILABLE"
	ErrCodeNotInWaitingRoom     = "NOT_IN_WAITING_ROOM"
	ErrCodeCannotJoinOwnRoom    = "CANNOT_JOIN_OWN_ROOM"
	ErrCodeNotInReadyUp         = "NOT_IN_READY_UP"
	ErrCodeNotInRoom            = "NOT_IN_ROOM"
	ErrCodeNotInPlacement       = "NOT_IN_PLACEMENT"
	ErrCodeFleetAlreadyLocked   = "FLEET_ALREADY_LOCKED"
	ErrCodeInvalidPlacement     = "INVALID_PLACEMENT"
	ErrCodeNotInBattle          = "NOT_IN_BATTLE"
	ErrCodeNotYourTurn          = "NOT_YOUR_TURN"
	ErrCodeCellAlreadyTargeted  = "CELL_ALREADY_TARGETED"
	ErrCodeCellOutOfBounds      = "INVALID_COORDINATE"
	ErrCodeGameOver             = "GAME_OVER"
	ErrCodeNotInFinished        = "NOT_IN_FINISHED"
	ErrCodeRematchNotAllowed    = "REMATCH_NOT_ALLOWED"
	ErrCodeOpponentNotPresent   = "OPPONENT_NOT_PRESENT"
	ErrCodeRateLimited          = "RATE_LIMIT_EXCEEDED"
)
