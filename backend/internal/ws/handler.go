package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/azbagas/battleship/backend/internal/lobby"
)

// wsSender serializes outgoing WebSocket writes for a single client connection.
type wsSender struct {
	mu   sync.Mutex
	conn *websocket.Conn
	ctx  context.Context
}

// Send formats and transmits an outbound message envelope.
func (s *wsSender) Send(msgType string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return writeJSON(s.ctx, s.conn, OutboundMessage{
		Type:    MessageType(msgType),
		Payload: payload,
	})
}

// Handler manages WebSocket upgrades, handshake, and message dispatch.
type Handler struct {
	registry *lobby.SessionRegistry
	lobby    *lobby.LobbyManager
}

// NewHandler constructs a WebSocket Handler backed by SessionRegistry and LobbyManager.
func NewHandler(registry *lobby.SessionRegistry, lobbyMgr *lobby.LobbyManager) *Handler {
	return &Handler{
		registry: registry,
		lobby:    lobbyMgr,
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
func sendError(sender *wsSender, code string, message string) error {
	return sender.Send(string(TypeError), ErrorPayload{
		Code:    code,
		Message: message,
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

	sender := &wsSender{conn: conn, ctx: r.Context()}

	// Enforce 5-second deadline for the initial hello handshake.
	handshakeCtx, cancelHandshake := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancelHandshake()

	player, err := h.handleHandshake(handshakeCtx, conn, sender)
	if err != nil {
		// Handshake failed; error already written to client if possible, close and exit.
		return
	}

	// Register connection sender with lobby manager.
	h.lobby.RegisterConnection(player.ID, sender)

	// Detach connection and clean up lobby state when this connection terminates.
	defer func() {
		h.lobby.UnsubscribeFromLobby(player.ID)
		h.lobby.UnregisterConnection(player.ID)
		h.lobby.HandleDisconnect(player.ID)
		h.registry.DetachConnection(player.SessionToken)
	}()

	// Check if player has an active room to restore.
	currentRoomID, _ := h.registry.GetPlayerRoom(player.ID)
	var restoredState any
	if currentRoomID != "" {
		if room, _ := h.lobby.HandleReconnect(player.ID); room != nil {
			restoredState = room
		}
	}

	// Send hello.ok with player identity and restoredState.
	helloOK := OutboundMessage{
		Type: TypeHelloOK,
		Payload: HelloOKPayload{
			PlayerID:      player.ID,
			SessionToken:  player.SessionToken,
			DisplayName:   player.DisplayName,
			RestoredState: restoredState,
		},
	}
	if err := sender.Send(string(TypeHelloOK), helloOK.Payload); err != nil {
		slog.Error("failed to write hello.ok", "error", err)
		return
	}

	// If player is not restored into an active room, place them in the lobby.
	if restoredState == nil {
		h.lobby.SubscribeToLobby(player.ID)
		snapshot := h.lobby.GetLobbySnapshot()
		if err := sender.Send(string(TypeLobbySnapshot), snapshot); err != nil {
			slog.Error("failed to write lobby.snapshot", "error", err)
			return
		}
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
			_ = sendError(sender, ErrCodeBadRequest, "Malformed message JSON")
			continue
		}

		switch inbound.Type {
		case TypeLobbyCreate:
			if _, err := h.lobby.CreateRoom(player.ID); err != nil {
				h.handleLobbyError(sender, err)
			}
		case TypeLobbyCancel:
			if err := h.lobby.CancelRoom(player.ID); err != nil {
				h.handleLobbyError(sender, err)
			}
		case TypeLobbyJoin:
			var joinPayload LobbyJoinPayload
			if err := json.Unmarshal(inbound.Payload, &joinPayload); err != nil || joinPayload.RoomID == "" {
				_ = sendError(sender, ErrCodeBadRequest, "Invalid join payload: roomId is required")
				continue
			}
			if _, err := h.lobby.JoinRoom(player.ID, joinPayload.RoomID); err != nil {
				h.handleLobbyError(sender, err)
			}
		default:
			// Dispatched in future tasks (T-06 ready-up, T-09 placement, T-12 battle).
		}
	}
}

// handleLobbyError translates lobby errors into protocol error codes.
func (h *Handler) handleLobbyError(sender *wsSender, err error) {
	switch {
	case errors.Is(err, lobby.ErrRoomCapReached):
		_ = sendError(sender, ErrCodeRoomCapReached, err.Error())
	case errors.Is(err, lobby.ErrAlreadyInRoom):
		_ = sendError(sender, ErrCodeAlreadyInRoom, err.Error())
	case errors.Is(err, lobby.ErrRoomNotFound):
		_ = sendError(sender, ErrCodeRoomNotFound, err.Error())
	case errors.Is(err, lobby.ErrRoomNotAvailable):
		_ = sendError(sender, ErrCodeRoomNotAvailable, err.Error())
	case errors.Is(err, lobby.ErrNotInWaitingRoom):
		_ = sendError(sender, ErrCodeNotInWaitingRoom, err.Error())
	case errors.Is(err, lobby.ErrCannotJoinOwnRoom):
		_ = sendError(sender, ErrCodeCannotJoinOwnRoom, err.Error())
	default:
		_ = sendError(sender, ErrCodeBadRequest, err.Error())
	}
}

// handleHandshake reads and processes the required hello handshake.
func (h *Handler) handleHandshake(ctx context.Context, conn *websocket.Conn, sender *wsSender) (*lobby.Player, error) {
	typ, msgBytes, err := conn.Read(ctx)
	if err != nil {
		_ = sendError(sender, ErrCodeInvalidHandshake, "Handshake timed out or failed to read")
		_ = conn.Close(websocket.StatusPolicyViolation, "handshake timeout")
		return nil, fmt.Errorf("read handshake: %w", err)
	}

	if typ != websocket.MessageText {
		_ = sendError(sender, ErrCodeInvalidHandshake, "Expected text frame")
		_ = conn.Close(websocket.StatusPolicyViolation, "expected text frame")
		return nil, errors.New("expected text frame")
	}

	var inbound InboundMessage
	if err := json.Unmarshal(msgBytes, &inbound); err != nil {
		_ = sendError(sender, ErrCodeBadRequest, "Malformed message JSON")
		_ = conn.Close(websocket.StatusPolicyViolation, "malformed JSON")
		return nil, fmt.Errorf("unmarshal handshake: %w", err)
	}

	if inbound.Type != TypeHello {
		_ = sendError(sender, ErrCodeInvalidHandshake, "First message must be hello")
		_ = conn.Close(websocket.StatusPolicyViolation, "expected hello")
		return nil, errors.New("expected hello message")
	}

	var payload HelloPayload
	if err := json.Unmarshal(inbound.Payload, &payload); err != nil {
		_ = sendError(sender, ErrCodeBadRequest, "Malformed hello payload")
		_ = conn.Close(websocket.StatusPolicyViolation, "malformed hello payload")
		return nil, fmt.Errorf("unmarshal hello payload: %w", err)
	}

	var player *lobby.Player

	if payload.SessionToken != "" {
		// Attempt to attach to existing session
		attached, attachErr := h.registry.AttachConnection(payload.SessionToken)
		if errors.Is(attachErr, lobby.ErrSessionAlreadyActive) {
			_ = sendError(sender, ErrCodeSessionAlreadyActive, "Session is already active in another connection")
			_ = conn.Close(websocket.StatusPolicyViolation, "session already active")
			return nil, lobby.ErrSessionAlreadyActive
		}

		if errors.Is(attachErr, lobby.ErrSessionNotFound) {
			// Unknown token (e.g., server restart) -> issue fresh session
			newPlayer, createErr := h.registry.CreatePlayer(payload.Name)
			if createErr != nil {
				_ = sendError(sender, ErrCodeInvalidName, createErr.Error())
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid display name")
				return nil, createErr
			}
			player = newPlayer
		} else if attachErr != nil {
			_ = sendError(sender, ErrCodeBadRequest, attachErr.Error())
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
			_ = sendError(sender, ErrCodeInvalidName, createErr.Error())
			_ = conn.Close(websocket.StatusPolicyViolation, "invalid display name")
			return nil, createErr
		}
		player = newPlayer
	}

	return player, nil
}
