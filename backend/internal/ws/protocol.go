package ws

import (
	"encoding/json"

	"github.com/azbagas/battleship/backend/internal/game"
)

// MessageType represents the domain-action identifier of a WebSocket message.
type MessageType string

const (
	TypeHello          MessageType = "hello"
	TypeHelloOK        MessageType = "hello.ok"
	TypeError          MessageType = "error"
	TypeLobbyCreate    MessageType = "lobby.create"
	TypeLobbyCancel    MessageType = "lobby.cancel"
	TypeLobbyJoin      MessageType = "lobby.join"
	TypeLobbySnapshot  MessageType = "lobby.snapshot"
	TypeLobbyUpdate    MessageType = "lobby.update"
	TypeRoomState      MessageType = "room.state"
	TypeRoomReady      MessageType = "room.ready"
	TypeRoomUnready    MessageType = "room.unready"
	TypeRoomLeave              MessageType = "room.leave"
	TypeGameLeave              MessageType = "game.leave"
	TypeRoomReadyState         MessageType = "room.readyState"
	TypePlacementSubmit        MessageType = "placement.submit"
	TypePlacementReady         MessageType = "placement.ready"
	TypePlacementOpponentReady MessageType = "placement.opponentReady"
	TypePlacementConfirmed     MessageType = "placement.confirmed"
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
type PlacementSubmitPayload struct {
	Ships []game.PlacedShip `json:"ships"`
}

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
)
