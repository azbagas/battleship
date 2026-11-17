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

const (
	defaultWriteTimeout = 5 * time.Second
	defaultOutboxSize   = 64
)

type outboundFrame struct {
	msgType string
	payload any
}

// wsSender serializes outgoing WebSocket writes for a single client connection.
// Pre-handshake frames write synchronously with a write timeout; post-handshake
// frames enqueue onto a bounded outbox so slow consumers cannot block broadcasts.
type wsSender struct {
	mu     sync.Mutex
	conn   *websocket.Conn
	ctx    context.Context
	outbox chan outboundFrame
	async  bool
	closed bool
	wg     sync.WaitGroup
}

// StartAsync transitions the sender to a bounded non-blocking outbox drained by a
// dedicated writer goroutine tied to ctx.
func (s *wsSender) StartAsync(ctx context.Context) {
	s.mu.Lock()
	if s.closed || s.async {
		s.mu.Unlock()
		return
	}
	s.async = true
	s.outbox = make(chan outboundFrame, defaultOutboxSize)
	s.wg.Add(1)
	s.mu.Unlock()

	go s.writeLoop(ctx)
}

func (s *wsSender) writeLoop(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case frame, ok := <-s.outbox:
			if !ok {
				return
			}
			writeCtx, cancel := context.WithTimeout(ctx, defaultWriteTimeout)
			err := writeJSON(writeCtx, s.conn, OutboundMessage{
				Type:    MessageType(frame.msgType),
				Payload: frame.payload,
			})
			cancel()
			if err != nil {
				s.closeOnSlowOrBroken("write failed")
				return
			}
		}
	}
}

func (s *wsSender) closeOnSlowOrBroken(reason string) {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		if s.async && s.outbox != nil {
			close(s.outbox)
		}
	}
	s.mu.Unlock()
	_ = s.conn.Close(websocket.StatusGoingAway, reason)
}

// Stop closes the outbox (if active) and waits for the writer goroutine to exit.
func (s *wsSender) Stop() {
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		if s.async && s.outbox != nil {
			close(s.outbox)
		}
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// Send formats and transmits an outbound message envelope.
func (s *wsSender) Send(msgType string, payload any) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("websocket sender closed")
	}
	if !s.async {
		defer s.mu.Unlock()
		writeCtx, cancel := context.WithTimeout(s.ctx, defaultWriteTimeout)
		defer cancel()
		return writeJSON(writeCtx, s.conn, OutboundMessage{
			Type:    MessageType(msgType),
			Payload: payload,
		})
	}

	select {
	case s.outbox <- outboundFrame{msgType: msgType, payload: payload}:
		s.mu.Unlock()
		return nil
	default:
		// Outbox is full: client is a slow consumer; detach it without blocking callers.
		s.closed = true
		close(s.outbox)
		s.mu.Unlock()
		_ = s.conn.Close(websocket.StatusGoingAway, "slow consumer")
		return errors.New("outbound buffer full: slow consumer")
	}
}

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
			if submitPayload.Draft {
				if err := h.lobby.UpdateDraftFleet(player.ID, submitPayload.Ships); err != nil {
					h.handleLobbyError(sender, player.ID, inbound.Type, err)
				}
				continue
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
		case TypeRematchAccept:
			if err := h.lobby.AcceptRematch(player.ID); err != nil {
				h.handleLobbyError(sender, player.ID, inbound.Type, err)
			}
		case TypeRematchDecline:
			if err := h.lobby.DeclineRematch(player.ID); err != nil {
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
	case errors.Is(err, lobby.ErrNotInFinished):
		code = ErrCodeNotInFinished
	case errors.Is(err, lobby.ErrRematchNotAllowed):
		code = ErrCodeRematchNotAllowed
	case errors.Is(err, lobby.ErrOpponentNotPresent):
		code = ErrCodeOpponentNotPresent
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
			// Successfully reconnected to existing session.
			// Only allow display name updates when the player is not in an active room/match.
			player = attached
			if payload.Name != "" && player.CurrentRoomID == "" {
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
