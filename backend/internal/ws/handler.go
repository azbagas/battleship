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

	"github.com/azbagas/battleship/backend/internal/game"
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
		slog.Error("failed to accept websocket connection", "remoteAddr", r.RemoteAddr, "error", err)
		return
	}
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	slog.Debug("websocket connection accepted", "remoteAddr", r.RemoteAddr)

	sender := &wsSender{conn: conn, ctx: r.Context()}

	// Enforce 5-second deadline for the initial hello handshake.
	handshakeCtx, cancelHandshake := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancelHandshake()

	player, err := h.handleHandshake(handshakeCtx, conn, sender, r.RemoteAddr)
	if err != nil {
		slog.Warn("websocket handshake failed", "remoteAddr", r.RemoteAddr, "error", err)
		return
	}

	// Register connection sender with lobby manager.
	h.lobby.RegisterConnection(player.ID, sender)

	// Detach connection and clean up lobby state when this connection terminates.
	defer func() {
		slog.Info("websocket connection closed", "playerId", player.ID, "remoteAddr", r.RemoteAddr)
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
		slog.Error("failed to write hello.ok", "playerId", player.ID, "error", err)
		return
	}

	// If player is not restored into an active room, place them in the lobby.
	if restoredState == nil {
		h.lobby.SubscribeToLobby(player.ID)
		snapshot := h.lobby.GetLobbySnapshot()
		if err := sender.Send(string(TypeLobbySnapshot), snapshot); err != nil {
			slog.Error("failed to write lobby.snapshot", "playerId", player.ID, "error", err)
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
			slog.Warn("malformed inbound message", "playerId", player.ID, "error", err)
			_ = sendError(sender, ErrCodeBadRequest, "Malformed message JSON")
			continue
		}

		slog.Debug("websocket message received", "playerId", player.ID, "type", inbound.Type)

		switch inbound.Type {
		case TypeLobbyCreate:
			if _, err := h.lobby.CreateRoom(player.ID); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypeLobbyCancel:
			if err := h.lobby.CancelRoom(player.ID); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypeLobbyJoin:
			var joinPayload LobbyJoinPayload
			if err := json.Unmarshal(inbound.Payload, &joinPayload); err != nil || joinPayload.RoomID == "" {
				slog.Warn("invalid join payload", "playerId", player.ID)
				_ = sendError(sender, ErrCodeBadRequest, "Invalid join payload: roomId is required")
				continue
			}
			if _, err := h.lobby.JoinRoom(player.ID, joinPayload.RoomID); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypeRoomReady:
			if err := h.lobby.ReadyRoom(player.ID); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypeRoomUnready:
			if err := h.lobby.UnreadyRoom(player.ID); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypeRoomLeave, TypeGameLeave:
			if err := h.lobby.LeaveRoom(player.ID); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypePlacementSubmit, TypePlacementReady:
			var submitPayload PlacementSubmitPayload
			if len(inbound.Payload) > 0 && string(inbound.Payload) != "null" && string(inbound.Payload) != "{}" {
				if err := json.Unmarshal(inbound.Payload, &submitPayload); err != nil {
					slog.Warn("invalid placement payload", "playerId", player.ID, "error", err)
					_ = sendError(sender, ErrCodeBadRequest, "Invalid placement payload: malformed ships data")
					continue
				}
			}
			if err := h.lobby.SubmitFleet(player.ID, submitPayload.Ships); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypeBattleFire:
			var firePayload BattleFirePayload
			if len(inbound.Payload) > 0 && string(inbound.Payload) != "null" && string(inbound.Payload) != "{}" {
				if err := json.Unmarshal(inbound.Payload, &firePayload); err != nil {
					slog.Warn("invalid battle fire payload", "playerId", player.ID, "error", err)
					_ = sendError(sender, ErrCodeBadRequest, "Invalid battle fire payload: malformed coordinate")
					continue
				}
			}
			if err := h.lobby.FireShot(player.ID, firePayload.Cell); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		default:
			slog.Debug("unhandled message type", "playerId", player.ID, "type", inbound.Type)
		}
	}
}

// handleLobbyError translates lobby errors into protocol error codes and logs the rejection.
func (h *Handler) handleLobbyError(sender *wsSender, playerID string, action MessageType, err error) {
	code := ErrCodeBadRequest
	switch {
	case errors.Is(err, lobby.ErrRoomCapReached):
		code = ErrCodeRoomCapReached
	case errors.Is(err, lobby.ErrAlreadyInRoom):
		code = ErrCodeAlreadyInRoom
	case errors.Is(err, lobby.ErrRoomNotFound):
		code = ErrCodeRoomNotFound
	case errors.Is(err, lobby.ErrRoomNotAvailable):
		code = ErrCodeRoomNotAvailable
	case errors.Is(err, lobby.ErrNotInWaitingRoom):
		code = ErrCodeNotInWaitingRoom
	case errors.Is(err, lobby.ErrCannotJoinOwnRoom):
		code = ErrCodeCannotJoinOwnRoom
	case errors.Is(err, lobby.ErrNotInReadyUp):
		code = ErrCodeNotInReadyUp
	case errors.Is(err, lobby.ErrNotInRoom):
		code = ErrCodeNotInRoom
	case errors.Is(err, lobby.ErrNotInPlacement):
		code = ErrCodeNotInPlacement
	case errors.Is(err, lobby.ErrFleetAlreadyLocked):
		code = ErrCodeFleetAlreadyLocked
	case errors.Is(err, lobby.ErrInvalidPlacement):
		code = ErrCodeInvalidPlacement
	case errors.Is(err, lobby.ErrNotInBattle):
		code = ErrCodeNotInBattle
	case errors.Is(err, lobby.ErrNotYourTurn):
		code = ErrCodeNotYourTurn
	case errors.Is(err, lobby.ErrGameOver):
		code = ErrCodeGameOver
	case errors.Is(err, game.ErrCellAlreadyTargeted):
		code = ErrCodeCellAlreadyTargeted
	case errors.Is(err, game.ErrCellOutOfBounds):
		code = ErrCodeCellOutOfBounds
	}
	slog.Warn("lobby action rejected", "playerId", playerID, "action", string(action), "code", code, "error", err)
	_ = sendError(sender, code, err.Error())
}

// handleHandshake reads and processes the required hello handshake.
func (h *Handler) handleHandshake(ctx context.Context, conn *websocket.Conn, sender *wsSender, remoteAddr string) (*lobby.Player, error) {
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
			slog.Warn("handshake rejected: session already active", "displayName", payload.Name, "remoteAddr", remoteAddr)
			_ = sendError(sender, ErrCodeSessionAlreadyActive, "Session is already active in another connection")
			_ = conn.Close(websocket.StatusPolicyViolation, "session already active")
			return nil, lobby.ErrSessionAlreadyActive
		}

		if errors.Is(attachErr, lobby.ErrSessionNotFound) {
			// Unknown token (e.g., server restart) -> issue fresh session
			newPlayer, createErr := h.registry.CreatePlayer(payload.Name)
			if createErr != nil {
				slog.Warn("handshake rejected: invalid display name", "displayName", payload.Name, "remoteAddr", remoteAddr, "error", createErr)
				_ = sendError(sender, ErrCodeInvalidName, createErr.Error())
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid display name")
				return nil, createErr
			}
			player = newPlayer
			slog.Info("player authenticated", "session", "new (reissued)", "playerId", player.ID, "displayName", player.DisplayName, "remoteAddr", remoteAddr)
		} else if attachErr != nil {
			slog.Warn("handshake rejected: failed to attach session", "remoteAddr", remoteAddr, "error", attachErr)
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
			slog.Info("player authenticated", "session", "resumed", "playerId", player.ID, "displayName", player.DisplayName, "remoteAddr", remoteAddr)
		}
	} else {
		// No session token provided -> create fresh session
		newPlayer, createErr := h.registry.CreatePlayer(payload.Name)
		if createErr != nil {
			slog.Warn("handshake rejected: invalid display name", "displayName", payload.Name, "remoteAddr", remoteAddr, "error", createErr)
			_ = sendError(sender, ErrCodeInvalidName, createErr.Error())
			_ = conn.Close(websocket.StatusPolicyViolation, "invalid display name")
			return nil, createErr
		}
		player = newPlayer
		slog.Info("player authenticated", "session", "new", "playerId", player.ID, "displayName", player.DisplayName, "remoteAddr", remoteAddr)
	}

	return player, nil
}
