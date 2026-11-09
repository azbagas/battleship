package ws

import "encoding/json"

// MessageType represents the domain-action identifier of a WebSocket message.
type MessageType string

const (
	TypeHello   MessageType = "hello"
	TypeHelloOK MessageType = "hello.ok"
	TypeError   MessageType = "error"
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

// Standard protocol error codes.
const (
	ErrCodeInvalidHandshake     = "INVALID_HANDSHAKE"
	ErrCodeInvalidName          = "INVALID_NAME"
	ErrCodeSessionAlreadyActive = "SESSION_ALREADY_ACTIVE"
	ErrCodeBadRequest           = "BAD_REQUEST"
)

