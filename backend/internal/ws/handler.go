package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/azbagas/battleship/backend/internal/lobby"
)

// Handler manages WebSocket upgrades, handshake, and message dispatch.
type Handler struct {
	registry *lobby.SessionRegistry
}

// NewHandler constructs a WebSocket Handler backed by the given SessionRegistry.
func NewHandler(registry *lobby.SessionRegistry) *Handler {
	return &Handler{
		registry: registry,
	}
}

// writeJSON serializes v as a JSON text frame and writes it to the WebSocket connection.
func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON message: %w", err)
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

// sendError sends a structured error envelope to the client.
func sendError(ctx context.Context, conn *websocket.Conn, code string, message string) error {
	return writeJSON(ctx, conn, OutboundMessage{
		Type: TypeError,
		Payload: ErrorPayload{
			Code:    code,
			Message: message,
		},
	})
}

// ServeHTTP handles the WebSocket upgrade and runs the connection lifecycle.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Origin verification is hardened in T-19.
	})
	if err != nil {
		slog.Error("failed to accept websocket connection", "error", err)
		return
	}
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	// Enforce 5-second deadline for the initial hello handshake.
	handshakeCtx, cancelHandshake := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancelHandshake()

	player, err := h.handleHandshake(handshakeCtx, conn)
	if err != nil {
		// Handshake failed; error already written to client if possible, close and exit.
		return
	}

	// Handshake succeeded: detach session when this connection terminates.
	defer h.registry.DetachConnection(player.SessionToken)

	// Send hello.ok with player identity and restoredState.
	helloOK := OutboundMessage{
		Type: TypeHelloOK,
		Payload: HelloOKPayload{
			PlayerID:      player.ID,
			SessionToken:  player.SessionToken,
			DisplayName:   player.DisplayName,
			RestoredState: nil,
		},
	}
	if err := writeJSON(r.Context(), conn, helloOK); err != nil {
		slog.Error("failed to write hello.ok", "error", err)
		return
	}

	// Message pump loop: read messages until disconnection.
	for {
		typ, msgBytes, err := conn.Read(r.Context())
		if err != nil {
			// Normal closure or network drop
			break
		}
		if typ != websocket.MessageText {
			continue
		}

		var inbound InboundMessage
		if err := json.Unmarshal(msgBytes, &inbound); err != nil {
			_ = sendError(r.Context(), conn, ErrCodeBadRequest, "Malformed message JSON")
			continue
		}

		// Dispatched in future tasks (T-04 lobby, T-06 ready-up, T-09 placement, T-12 battle).
	}
}

// handleHandshake reads and processes the required hello handshake.
func (h *Handler) handleHandshake(ctx context.Context, conn *websocket.Conn) (*lobby.Player, error) {
	typ, msgBytes, err := conn.Read(ctx)
	if err != nil {
		_ = sendError(ctx, conn, ErrCodeInvalidHandshake, "Handshake timed out or failed to read")
		_ = conn.Close(websocket.StatusPolicyViolation, "handshake timeout")
		return nil, fmt.Errorf("read handshake: %w", err)
	}

	if typ != websocket.MessageText {
		_ = sendError(ctx, conn, ErrCodeInvalidHandshake, "Expected text frame")
		_ = conn.Close(websocket.StatusPolicyViolation, "expected text frame")
		return nil, errors.New("expected text frame")
	}

	var inbound InboundMessage
	if err := json.Unmarshal(msgBytes, &inbound); err != nil {
		_ = sendError(ctx, conn, ErrCodeBadRequest, "Malformed message JSON")
		_ = conn.Close(websocket.StatusPolicyViolation, "malformed JSON")
		return nil, fmt.Errorf("unmarshal handshake: %w", err)
	}

	if inbound.Type != TypeHello {
		_ = sendError(ctx, conn, ErrCodeInvalidHandshake, "First message must be hello")
		_ = conn.Close(websocket.StatusPolicyViolation, "expected hello")
		return nil, errors.New("expected hello message")
	}

	var payload HelloPayload
	if err := json.Unmarshal(inbound.Payload, &payload); err != nil {
		_ = sendError(ctx, conn, ErrCodeBadRequest, "Malformed hello payload")
		_ = conn.Close(websocket.StatusPolicyViolation, "malformed hello payload")
		return nil, fmt.Errorf("unmarshal hello payload: %w", err)
	}

	var player *lobby.Player

	if payload.SessionToken != "" {
		// Attempt to attach to existing session
		attached, attachErr := h.registry.AttachConnection(payload.SessionToken)
		if errors.Is(attachErr, lobby.ErrSessionAlreadyActive) {
			_ = sendError(ctx, conn, ErrCodeSessionAlreadyActive, "Session is already active in another connection")
			_ = conn.Close(websocket.StatusPolicyViolation, "session already active")
			return nil, lobby.ErrSessionAlreadyActive
		}

		if errors.Is(attachErr, lobby.ErrSessionNotFound) {
			// Unknown token (e.g., server restart) -> issue fresh session
			newPlayer, createErr := h.registry.CreatePlayer(payload.Name)
			if createErr != nil {
				_ = sendError(ctx, conn, ErrCodeInvalidName, createErr.Error())
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid display name")
				return nil, createErr
			}
			player = newPlayer
		} else if attachErr != nil {
			_ = sendError(ctx, conn, ErrCodeBadRequest, attachErr.Error())
			_ = conn.Close(websocket.StatusPolicyViolation, "failed to attach session")
			return nil, attachErr
		} else {
			// Successfully reconnected to existing session
			player = attached
			// If name supplied, attempt updating display name
			if payload.Name != "" {
				if updateErr := h.registry.UpdateDisplayName(player.SessionToken, payload.Name); updateErr == nil {
					if updated, ok := h.registry.GetByToken(player.SessionToken); ok {
						player = updated
					}
				}
			}
		}
	} else {
		// No session token provided -> create fresh session
		newPlayer, createErr := h.registry.CreatePlayer(payload.Name)
		if createErr != nil {
			_ = sendError(ctx, conn, ErrCodeInvalidName, createErr.Error())
			_ = conn.Close(websocket.StatusPolicyViolation, "invalid display name")
			return nil, createErr
		}
		player = newPlayer
	}

	return player, nil
}

