package ws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/azbagas/battleship/backend/internal/game"
	"github.com/azbagas/battleship/backend/internal/lobby"
)

// HandlerOption configures Handler instances.
type HandlerOption func(*Handler)

// WithPingConfig sets custom ping interval and timeout for heartbeats.
func WithPingConfig(interval, timeout time.Duration) HandlerOption {
	return func(h *Handler) {
		h.pingInterval = interval
		h.pingTimeout = timeout
	}
}

// WithAllowedOrigins configures authorized Origin patterns for WebSocket handshakes.
// Passing []string{"*"} allows any origin (dev/testing mode).
func WithAllowedOrigins(origins []string) HandlerOption {
	return func(h *Handler) {
		if len(origins) == 1 && origins[0] == "*" {
			h.allowAllOrigins = true
			h.originPatterns = nil
			return
		}
		h.allowAllOrigins = false
		h.originPatterns = origins
	}
}

// WithMaxPayloadBytes configures the maximum allowed WebSocket message payload size in bytes.
func WithMaxPayloadBytes(bytes int64) HandlerOption {
	return func(h *Handler) {
		if bytes > 0 {
			h.maxPayloadBytes = bytes
		}
	}
}

// WithIPLimit configures the maximum concurrent connections allowed per IP address.
func WithIPLimit(maxPerIP int) HandlerOption {
	return func(h *Handler) {
		h.ipTracker = NewIPTracker(maxPerIP)
	}
}

// WithRateLimit configures the per-connection message rate limit (replenishment rate and burst capacity).
func WithRateLimit(rate, burst float64) HandlerOption {
	return func(h *Handler) {
		h.rateLimit = rate
		h.rateBurst = burst
	}
}

// WithTrustProxy configures whether X-Real-IP / X-Forwarded-For headers are trusted
// when identifying client IPs for rate and connection limits.
func WithTrustProxy(trust bool) HandlerOption {
	return func(h *Handler) {
		h.trustProxy = trust
	}
}

// WithHandshakeRateLimit configures the per-IP WebSocket upgrade rate limit.
func WithHandshakeRateLimit(rate, burst float64) HandlerOption {
	return func(h *Handler) {
		h.handshakeLimiter = NewHandshakeLimiter(rate, burst)
	}
}

// Handler manages WebSocket upgrades, handshake, and message dispatch.
type Handler struct {
	registry         *lobby.SessionRegistry
	lobby            *lobby.LobbyManager
	pingInterval     time.Duration
	pingTimeout      time.Duration
	originPatterns   []string
	allowAllOrigins  bool
	trustProxy       bool
	maxPayloadBytes  int64
	rateLimit        float64
	rateBurst        float64
	ipTracker        *IPTracker
	handshakeLimiter *HandshakeLimiter
	connsMu          sync.Mutex
	activeConns      map[*websocket.Conn]struct{}
	shuttingDown     atomic.Bool
}

// NewHandler constructs a WebSocket Handler backed by SessionRegistry and LobbyManager.
func NewHandler(registry *lobby.SessionRegistry, lobbyMgr *lobby.LobbyManager, opts ...HandlerOption) *Handler {
	h := &Handler{
		registry:         registry,
		lobby:            lobbyMgr,
		pingInterval:     15 * time.Second,
		pingTimeout:      5 * time.Second,
		originPatterns:   []string{"localhost:*", "127.0.0.1:*"},
		allowAllOrigins:  false,
		trustProxy:       false,
		maxPayloadBytes:  4096,
		rateLimit:        10.0,
		rateBurst:        20.0,
		ipTracker:        NewIPTracker(10),
		handshakeLimiter: NewHandshakeLimiter(5.0, 20.0),
		activeConns:      make(map[*websocket.Conn]struct{}),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Shutdown initiates graceful shutdown: halts new upgrades and closes active connections.
func (h *Handler) Shutdown(ctx context.Context) error {
	h.shuttingDown.Store(true)

	h.connsMu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.activeConns))
	for c := range h.activeConns {
		conns = append(conns, c)
	}
	h.connsMu.Unlock()

	for _, c := range conns {
		_ = c.Close(websocket.StatusGoingAway, "server shutting down")
	}

	return nil
}

func (h *Handler) registerConn(c *websocket.Conn) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	h.activeConns[c] = struct{}{}
}

func (h *Handler) unregisterConn(c *websocket.Conn) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	delete(h.activeConns, c)
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
	if h.shuttingDown.Load() {
		http.Error(w, "server is shutting down", http.StatusServiceUnavailable)
		return
	}

	clientIP := ExtractClientIP(r, h.trustProxy)
	if !h.handshakeLimiter.Allow(clientIP) {
		slog.Warn("connection rejected: handshake rate limit exceeded", "remoteAddr", r.RemoteAddr, "ip", clientIP)
		http.Error(w, "Too many connection attempts from your IP", http.StatusTooManyRequests)
		return
	}
	if !h.ipTracker.Acquire(clientIP) {
		slog.Warn("connection rejected: per-ip connection cap reached", "remoteAddr", r.RemoteAddr, "ip", clientIP)
		http.Error(w, "Too many concurrent connections from your IP", http.StatusTooManyRequests)
		return
	}
	defer h.ipTracker.Release(clientIP)

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:     h.originPatterns,
		InsecureSkipVerify: h.allowAllOrigins,
	})
	if err != nil {
		slog.Warn("failed to accept websocket connection", "remoteAddr", r.RemoteAddr, "error", err)
		return
	}
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	h.registerConn(conn)
	defer h.unregisterConn(conn)

	conn.SetReadLimit(h.maxPayloadBytes)

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

	sessionCtx, cancelSession := context.WithCancel(r.Context())
	defer func() {
		cancelSession()
		sender.Stop()
	}()

	if h.pingInterval > 0 {
		go h.heartbeatLoop(sessionCtx, conn, player.ID)
	}

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
		if roomState, _ := h.lobby.HandleReconnect(player.ID); roomState != nil {
			restoredState = roomState
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

	// Handshake complete: enable non-blocking outbound writer and register with lobby manager.
	sender.StartAsync(sessionCtx)
	h.lobby.RegisterConnection(player.ID, sender)

	// If player is restored into an active room, also send room.state after hello.ok.
	if restoredState != nil {
		if err := sender.Send(string(TypeRoomState), restoredState); err != nil {
			slog.Error("failed to write room.state on reconnect", "playerId", player.ID, "error", err)
			return
		}
	} else {
		// If player is not restored into an active room, place them in the lobby.
		h.lobby.SubscribeToLobby(player.ID)
		snapshot := h.lobby.GetLobbySnapshot()
		if err := sender.Send(string(TypeLobbySnapshot), snapshot); err != nil {
			slog.Error("failed to write lobby.snapshot", "playerId", player.ID, "error", err)
			return
		}
	}

	limiter := NewRateLimiter(h.rateLimit, h.rateBurst)

	// Message pump loop: read messages until disconnection.
	for {
		typ, msgBytes, err := conn.Read(r.Context())
		if err != nil {
			// Normal closure, network drop, or read limit exceeded
			break
		}
		if typ != websocket.MessageText {
			continue
		}

		if !limiter.Allow() {
			slog.Warn("rate limit exceeded", "playerId", player.ID, "remoteAddr", r.RemoteAddr)
			_ = sendError(sender, ErrCodeRateLimited, "Message rate limit exceeded")
			continue
		}

		var inbound InboundMessage
		if err := json.Unmarshal(msgBytes, &inbound); err != nil {
			slog.Warn("malformed inbound message", "playerId", player.ID, "error", err)
			_ = sendError(sender, ErrCodeBadRequest, "Malformed message JSON")
			continue
		}

		slog.Debug("websocket message received", "playerId", player.ID, "type", inbound.Type)
		h.dispatchMessage(sender, player.ID, inbound)
	}
}

func (h *Handler) dispatchMessage(sender *wsSender, playerID string, inbound InboundMessage) {
	switch inbound.Type {
	case TypeLobbyCreate:
		if _, err := h.lobby.CreateRoom(playerID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeLobbyCancel:
		if err := h.lobby.CancelRoom(playerID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeLobbyJoin:
		var joinPayload LobbyJoinPayload
		if err := json.Unmarshal(inbound.Payload, &joinPayload); err != nil || joinPayload.RoomID == "" {
			slog.Warn("invalid join payload", "playerId", playerID)
			_ = sendError(sender, ErrCodeBadRequest, "Invalid join payload: roomId is required")
			return
		}
		if _, err := h.lobby.JoinRoom(playerID, joinPayload.RoomID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeRoomReady:
		if err := h.lobby.ReadyRoom(playerID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeRoomUnready:
		if err := h.lobby.UnreadyRoom(playerID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeRoomLeave, TypeGameLeave:
		if err := h.lobby.LeaveRoom(playerID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypePlacementSubmit, TypePlacementReady:
		var submitPayload PlacementSubmitPayload
		if err := unmarshalOptionalPayload(inbound.Payload, &submitPayload); err != nil {
			slog.Warn("invalid placement payload", "playerId", playerID, "error", err)
			_ = sendError(sender, ErrCodeBadRequest, "Invalid placement payload: malformed ships data")
			return
		}
		if submitPayload.Draft {
			if err := h.lobby.UpdateDraftFleet(playerID, submitPayload.Ships); err != nil {
				h.handleLobbyError(sender, playerID, inbound.Type, err)
			}
			return
		}
		if err := h.lobby.SubmitFleet(playerID, submitPayload.Ships); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeBattleFire:
		var firePayload BattleFirePayload
		if err := unmarshalOptionalPayload(inbound.Payload, &firePayload); err != nil {
			slog.Warn("invalid battle fire payload", "playerId", playerID, "error", err)
			_ = sendError(sender, ErrCodeBadRequest, "Invalid battle fire payload: malformed coordinate")
			return
		}
		if err := h.lobby.FireShot(playerID, firePayload.Cell); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeRematchAccept:
		if err := h.lobby.AcceptRematch(playerID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	case TypeRematchDecline:
		if err := h.lobby.DeclineRematch(playerID); err != nil {
			h.handleLobbyError(sender, playerID, inbound.Type, err)
		}
	default:
		slog.Debug("unhandled message type", "playerId", playerID, "type", inbound.Type)
	}
}

func unmarshalOptionalPayload(raw json.RawMessage, dst any) error {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "{}" {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

var lobbyErrorCodes = []struct {
	target error
	code   string
}{
	{lobby.ErrRoomCapReached, ErrCodeRoomCapReached},
	{lobby.ErrAlreadyInRoom, ErrCodeAlreadyInRoom},
	{lobby.ErrRoomNotFound, ErrCodeRoomNotFound},
	{lobby.ErrRoomNotAvailable, ErrCodeRoomNotAvailable},
	{lobby.ErrNotInWaitingRoom, ErrCodeNotInWaitingRoom},
	{lobby.ErrCannotJoinOwnRoom, ErrCodeCannotJoinOwnRoom},
	{lobby.ErrNotInReadyUp, ErrCodeNotInReadyUp},
	{lobby.ErrNotInRoom, ErrCodeNotInRoom},
	{lobby.ErrNotInPlacement, ErrCodeNotInPlacement},
	{lobby.ErrFleetAlreadyLocked, ErrCodeFleetAlreadyLocked},
	{lobby.ErrInvalidPlacement, ErrCodeInvalidPlacement},
	{lobby.ErrNotInBattle, ErrCodeNotInBattle},
	{lobby.ErrNotYourTurn, ErrCodeNotYourTurn},
	{lobby.ErrGameOver, ErrCodeGameOver},
	{lobby.ErrNotInFinished, ErrCodeNotInFinished},
	{lobby.ErrRematchNotAllowed, ErrCodeRematchNotAllowed},
	{lobby.ErrOpponentNotPresent, ErrCodeOpponentNotPresent},
	{game.ErrCellAlreadyTargeted, ErrCodeCellAlreadyTargeted},
	{game.ErrCellOutOfBounds, ErrCodeCellOutOfBounds},
}

// handleLobbyError translates lobby errors into protocol error codes and logs the rejection.
func (h *Handler) handleLobbyError(sender *wsSender, playerID string, action MessageType, err error) {
	code := ErrCodeBadRequest
	for _, mapping := range lobbyErrorCodes {
		if errors.Is(err, mapping.target) {
			code = mapping.code
			break
		}
	}
	slog.Warn("lobby action rejected", "playerId", playerID, "action", string(action), "code", code, "error", err)
	_ = sendError(sender, code, err.Error())
}

func rejectHandshake(conn *websocket.Conn, sender *wsSender, code, msg, closeReason string) {
	_ = sendError(sender, code, msg)
	_ = conn.Close(websocket.StatusPolicyViolation, closeReason)
}

func (h *Handler) createHandshakePlayer(conn *websocket.Conn, sender *wsSender, name, remoteAddr, sessionKind string) (*lobby.Player, error) {
	newPlayer, createErr := h.registry.CreatePlayer(name)
	if createErr != nil {
		slog.Warn("handshake rejected: invalid display name", "displayName", name, "remoteAddr", remoteAddr, "error", createErr)
		rejectHandshake(conn, sender, ErrCodeInvalidName, createErr.Error(), "invalid display name")
		return nil, createErr
	}
	slog.Info("player authenticated", "session", sessionKind, "playerId", newPlayer.ID, "displayName", newPlayer.DisplayName, "remoteAddr", remoteAddr)
	return newPlayer, nil
}

// handleHandshake reads and processes the required hello handshake.
func (h *Handler) handleHandshake(ctx context.Context, conn *websocket.Conn, sender *wsSender, remoteAddr string) (*lobby.Player, error) {
	typ, msgBytes, err := conn.Read(ctx)
	if err != nil {
		rejectHandshake(conn, sender, ErrCodeInvalidHandshake, "Handshake timed out or failed to read", "handshake timeout")
		return nil, fmt.Errorf("read handshake: %w", err)
	}

	if typ != websocket.MessageText {
		rejectHandshake(conn, sender, ErrCodeInvalidHandshake, "Expected text frame", "expected text frame")
		return nil, errors.New("expected text frame")
	}

	var inbound InboundMessage
	if err := json.Unmarshal(msgBytes, &inbound); err != nil {
		rejectHandshake(conn, sender, ErrCodeBadRequest, "Malformed message JSON", "malformed JSON")
		return nil, fmt.Errorf("unmarshal handshake: %w", err)
	}

	if inbound.Type != TypeHello {
		rejectHandshake(conn, sender, ErrCodeInvalidHandshake, "First message must be hello", "expected hello")
		return nil, errors.New("expected hello message")
	}

	var payload HelloPayload
	if err := json.Unmarshal(inbound.Payload, &payload); err != nil {
		rejectHandshake(conn, sender, ErrCodeBadRequest, "Malformed hello payload", "malformed hello payload")
		return nil, fmt.Errorf("unmarshal hello payload: %w", err)
	}

	if payload.SessionToken == "" {
		return h.createHandshakePlayer(conn, sender, payload.Name, remoteAddr, "new")
	}

	attached, attachErr := h.registry.AttachConnection(payload.SessionToken)
	switch {
	case errors.Is(attachErr, lobby.ErrSessionAlreadyActive):
		slog.Warn("handshake rejected: session already active", "displayName", payload.Name, "remoteAddr", remoteAddr)
		rejectHandshake(conn, sender, ErrCodeSessionAlreadyActive, "Session is already active in another connection", "session already active")
		return nil, lobby.ErrSessionAlreadyActive

	case errors.Is(attachErr, lobby.ErrSessionNotFound):
		return h.createHandshakePlayer(conn, sender, payload.Name, remoteAddr, "new (reissued)")

	case attachErr != nil:
		slog.Warn("handshake rejected: failed to attach session", "remoteAddr", remoteAddr, "error", attachErr)
		rejectHandshake(conn, sender, ErrCodeBadRequest, attachErr.Error(), "failed to attach session")
		return nil, attachErr

	default:
		// Only allow display name updates when the player is not in an active room/match.
		player := attached
		if payload.Name != "" && player.CurrentRoomID == "" {
			if updateErr := h.registry.UpdateDisplayName(player.SessionToken, payload.Name); updateErr == nil {
				if updated, ok := h.registry.GetByToken(player.SessionToken); ok {
					player = updated
				}
			}
		}
		slog.Info("player authenticated", "session", "resumed", "playerId", player.ID, "displayName", player.DisplayName, "remoteAddr", remoteAddr)
		return player, nil
	}
}

// heartbeatLoop periodically pings the client to detect dead connections.
func (h *Handler) heartbeatLoop(ctx context.Context, conn *websocket.Conn, playerID string) {
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancelPing := context.WithTimeout(ctx, h.pingTimeout)
			err := conn.Ping(pingCtx)
			cancelPing()
			if err != nil {
				slog.Debug("heartbeat ping failed, closing connection", "playerId", playerID, "error", err)
				_ = conn.Close(websocket.StatusPolicyViolation, "ping timeout")
				return
			}
		}
	}
}
