package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/azbagas/battleship/backend/internal/game"
	"github.com/azbagas/battleship/backend/internal/lobby"
	"github.com/azbagas/battleship/backend/internal/store"
)

func setupTestServer(t *testing.T, opts ...lobby.Option) (*httptest.Server, *lobby.SessionRegistry, *lobby.LobbyManager) {
	t.Helper()
	reg := lobby.NewSessionRegistry()
	mgr := lobby.NewLobbyManager(reg, opts...)
	handler := NewHandler(reg, mgr)
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.Close()
	})
	return server, reg, mgr
}

func setupTestServerWithHandlerOpts(t *testing.T, handlerOpts []HandlerOption, lobbyOpts ...lobby.Option) (*httptest.Server, *lobby.SessionRegistry, *lobby.LobbyManager, *Handler) {
	t.Helper()
	reg := lobby.NewSessionRegistry()
	mgr := lobby.NewLobbyManager(reg, lobbyOpts...)
	handler := NewHandler(reg, mgr, handlerOpts...)
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.Close()
	})
	return server, reg, mgr, handler
}

func dialWS(t *testing.T, ctx context.Context, serverURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http")
	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	return conn
}

func readEnvelope(t *testing.T, ctx context.Context, conn *websocket.Conn) (MessageType, json.RawMessage) {
	t.Helper()
	typ, msgBytes, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read websocket message: %v", err)
	}
	if typ != websocket.MessageText {
		t.Fatalf("expected MessageText, got %v", typ)
	}

	var env InboundMessage
	if err := json.Unmarshal(msgBytes, &env); err != nil {
		t.Fatalf("failed to unmarshal message envelope: %v (raw: %s)", err, string(msgBytes))
	}
	return env.Type, env.Payload
}

func connectAndHandshake(t *testing.T, ctx context.Context, serverURL string, name string, sessionToken string) (*websocket.Conn, HelloOKPayload, lobby.LobbySnapshotPayload) {
	t.Helper()
	conn := dialWS(t, ctx, serverURL)

	helloMsg := OutboundMessage{
		Type: TypeHello,
		Payload: HelloPayload{
			Name:         name,
			SessionToken: sessionToken,
		},
	}
	data, _ := json.Marshal(helloMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to write hello: %v", err)
	}

	msgType, payloadBytes := readEnvelope(t, ctx, conn)
	if msgType != TypeHelloOK {
		t.Fatalf("expected %s, got %s (payload: %s)", TypeHelloOK, msgType, string(payloadBytes))
	}

	var helloOK HelloOKPayload
	_ = json.Unmarshal(payloadBytes, &helloOK)

	var snap lobby.LobbySnapshotPayload
	if helloOK.RestoredState == nil {
		snapType, snapBytes := readEnvelope(t, ctx, conn)
		if snapType != TypeLobbySnapshot {
			t.Fatalf("expected %s after hello.ok, got %s", TypeLobbySnapshot, snapType)
		}
		_ = json.Unmarshal(snapBytes, &snap)
	}

	return conn, helloOK, snap
}

func TestHandshake_NewSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})
	conn, helloOK, snap := connectAndHandshake(t, ctx, server.URL, "  Captain Kirk  ", "")
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}()

	if helloOK.DisplayName != "Captain Kirk" {
		t.Errorf("expected trimmed display name 'Captain Kirk', got %q", helloOK.DisplayName)
	}
	if !strings.HasPrefix(helloOK.PlayerID, "p_") {
		t.Errorf("expected player ID with prefix 'p_', got %q", helloOK.PlayerID)
	}
	if len(helloOK.SessionToken) == 0 {
		t.Errorf("expected non-empty session token")
	}
	if snap.TotalRooms != 0 || snap.MaxRooms != 10 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestHandshake_ReconnectRestoresSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	// 1. Initial connection
	conn1, ok1, _ := connectAndHandshake(t, ctx, server.URL, "Spock", "")
	_ = conn1.Close(websocket.StatusNormalClosure, "closing client 1")

	// Wait briefly for server disconnect hook to run
	time.Sleep(50 * time.Millisecond)

	// 2. Reconnect with the same session token
	conn2, ok2, _ := connectAndHandshake(t, ctx, server.URL, "Spock Updated", ok1.SessionToken)
	defer func() {
		_ = conn2.Close(websocket.StatusNormalClosure, "done")
	}()

	if ok2.PlayerID != ok1.PlayerID {
		t.Errorf("expected restored player ID %q, got %q", ok1.PlayerID, ok2.PlayerID)
	}
	if ok2.SessionToken != ok1.SessionToken {
		t.Errorf("expected same session token %q, got %q", ok1.SessionToken, ok2.SessionToken)
	}
	if ok2.DisplayName != "Spock Updated" {
		t.Errorf("expected updated display name 'Spock Updated', got %q", ok2.DisplayName)
	}
}

func TestHandshake_ConcurrentConnectionRejected(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, reg, _ := setupTestServer(t)

	// 1. First connection
	conn1, ok1, _ := connectAndHandshake(t, ctx, server.URL, "McCoy", "")
	defer func() {
		_ = conn1.Close(websocket.StatusNormalClosure, "closing 1")
	}()

	// 2. Second connection while first is still active
	conn2 := dialWS(t, ctx, server.URL)
	defer func() {
		_ = conn2.Close(websocket.StatusNormalClosure, "closing 2")
	}()

	conn2Hello := OutboundMessage{
		Type: TypeHello,
		Payload: HelloPayload{
			Name:         "McCoy Tab 2",
			SessionToken: ok1.SessionToken,
		},
	}
	data2, _ := json.Marshal(conn2Hello)
	_ = conn2.Write(ctx, websocket.MessageText, data2)

	// Second connection must receive error with SESSION_ALREADY_ACTIVE
	msgType2, payloadBytes2 := readEnvelope(t, ctx, conn2)
	if msgType2 != TypeError {
		t.Fatalf("expected %s on concurrent connection, got %s (payload: %s)", TypeError, msgType2, string(payloadBytes2))
	}

	var errPayload ErrorPayload
	if err := json.Unmarshal(payloadBytes2, &errPayload); err != nil {
		t.Fatalf("failed to unmarshal error payload: %v", err)
	}
	if errPayload.Code != ErrCodeSessionAlreadyActive {
		t.Errorf("expected code %s, got %s", ErrCodeSessionAlreadyActive, errPayload.Code)
	}

	// Read on conn2 should now result in connection closure
	_, _, err := conn2.Read(ctx)
	if err == nil {
		t.Errorf("expected conn2 to be closed after rejection")
	}

	// Verify conn1 remains alive and connected in registry
	player, ok := reg.GetByToken(ok1.SessionToken)
	if !ok || player.ConnectionState != lobby.StateConnected {
		t.Errorf("expected player1 to remain connected in registry")
	}
}

func TestHandshake_UnknownTokenCreatesNewSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})
	unknownToken := "invalid-or-restarted-server-token"
	conn, helloOK, _ := connectAndHandshake(t, ctx, server.URL, "Scotty", unknownToken)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}()

	if helloOK.SessionToken == unknownToken {
		t.Errorf("expected new session token to be generated, got unchanged %q", helloOK.SessionToken)
	}
	if helloOK.DisplayName != "Scotty" {
		t.Errorf("expected display name 'Scotty', got %q", helloOK.DisplayName)
	}
}

func TestHandshake_InvalidNameRejected(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
	}{
		{name: "empty name", displayName: ""},
		{name: "whitespace only", displayName: "    \t\n  "},
		{name: "exceeds 20 runes", displayName: "123456789012345678901"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})
			conn := dialWS(t, ctx, server.URL)
			defer func() {
				_ = conn.Close(websocket.StatusNormalClosure, "done")
			}()

			helloMsg := OutboundMessage{
				Type: TypeHello,
				Payload: HelloPayload{
					Name: tc.displayName,
				},
			}
			data, _ := json.Marshal(helloMsg)
			_ = conn.Write(ctx, websocket.MessageText, data)

			msgType, payloadBytes := readEnvelope(t, ctx, conn)
			if msgType != TypeError {
				t.Fatalf("expected %s, got %s", TypeError, msgType)
			}

			var errPayload ErrorPayload
			_ = json.Unmarshal(payloadBytes, &errPayload)
			if errPayload.Code != ErrCodeInvalidName {
				t.Errorf("expected error code %s, got %s", ErrCodeInvalidName, errPayload.Code)
			}
		})
	}
}

func TestHandshake_InvalidFirstMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})
	conn := dialWS(t, ctx, server.URL)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}()

	invalidMsg := OutboundMessage{
		Type:    "lobby.create",
		Payload: map[string]string{},
	}
	data, _ := json.Marshal(invalidMsg)
	_ = conn.Write(ctx, websocket.MessageText, data)

	msgType, payloadBytes := readEnvelope(t, ctx, conn)
	if msgType != TypeError {
		t.Fatalf("expected %s, got %s", TypeError, msgType)
	}

	var errPayload ErrorPayload
	_ = json.Unmarshal(payloadBytes, &errPayload)
	if errPayload.Code != ErrCodeInvalidHandshake {
		t.Errorf("expected error code %s, got %s", ErrCodeInvalidHandshake, errPayload.Code)
	}
}

func TestHandshake_MalformedJSON(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})
	conn := dialWS(t, ctx, server.URL)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}()

	_ = conn.Write(ctx, websocket.MessageText, []byte("{invalid json syntax"))

	msgType, payloadBytes := readEnvelope(t, ctx, conn)
	if msgType != TypeError {
		t.Fatalf("expected %s, got %s", TypeError, msgType)
	}

	var errPayload ErrorPayload
	_ = json.Unmarshal(payloadBytes, &errPayload)
	if errPayload.Code != ErrCodeBadRequest {
		t.Errorf("expected error code %s, got %s", ErrCodeBadRequest, errPayload.Code)
	}
}

func TestWebSocket_LobbyLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	// Client 1 connects (Alice)
	conn1, _, snap1 := connectAndHandshake(t, ctx, server.URL, "Alice", "")
	defer func() { _ = conn1.Close(websocket.StatusNormalClosure, "done") }()
	if len(snap1.Rooms) != 0 {
		t.Fatalf("expected empty snapshot for Alice, got: %+v", snap1)
	}

	// Client 2 connects (Bob)
	conn2, _, snap2 := connectAndHandshake(t, ctx, server.URL, "Bob", "")
	defer func() { _ = conn2.Close(websocket.StatusNormalClosure, "done") }()
	if len(snap2.Rooms) != 0 {
		t.Fatalf("expected empty snapshot for Bob, got: %+v", snap2)
	}

	// Alice creates room
	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})
	_ = conn1.Write(ctx, websocket.MessageText, createMsg)

	// Alice receives room.state (waiting)
	msgType1, payload1 := readEnvelope(t, ctx, conn1)
	if msgType1 != TypeRoomState {
		t.Fatalf("expected %s for host Alice, got %s (payload: %s)", TypeRoomState, msgType1, string(payload1))
	}
	var aliceState lobby.RoomStateView
	_ = json.Unmarshal(payload1, &aliceState)
	if aliceState.Phase != lobby.PhaseWaiting || aliceState.Role != "host" {
		t.Errorf("unexpected alice room state: %+v", aliceState)
	}

	// Bob receives lobby.update (action: upsert)
	msgType2, payload2 := readEnvelope(t, ctx, conn2)
	if msgType2 != TypeLobbyUpdate {
		t.Fatalf("expected %s for Bob, got %s (payload: %s)", TypeLobbyUpdate, msgType2, string(payload2))
	}
	var bobUpdate lobby.LobbyUpdatePayload
	_ = json.Unmarshal(payload2, &bobUpdate)
	if bobUpdate.Action != "upsert" || bobUpdate.Room.ID != aliceState.RoomID {
		t.Errorf("unexpected bob update: %+v", bobUpdate)
	}

	// Bob joins Alice's room
	joinMsg, _ := json.Marshal(OutboundMessage{
		Type:    TypeLobbyJoin,
		Payload: LobbyJoinPayload{RoomID: aliceState.RoomID},
	})
	_ = conn2.Write(ctx, websocket.MessageText, joinMsg)

	// Both should receive room.state (ready-up)
	mTypeA, pA := readEnvelope(t, ctx, conn1)
	if mTypeA != TypeRoomState {
		t.Fatalf("expected room.state on Alice after join, got %s", mTypeA)
	}
	var aReadyState lobby.RoomStateView
	_ = json.Unmarshal(pA, &aReadyState)
	if aReadyState.Phase != lobby.PhaseReadyUp || aReadyState.OpponentName != "Bob" {
		t.Errorf("unexpected Alice ready state: %+v", aReadyState)
	}

	mTypeB, pB := readEnvelope(t, ctx, conn2)
	if mTypeB != TypeRoomState {
		t.Fatalf("expected room.state on Bob after join, got %s", mTypeB)
	}
	var bReadyState lobby.RoomStateView
	_ = json.Unmarshal(pB, &bReadyState)
	if bReadyState.Phase != lobby.PhaseReadyUp || bReadyState.OpponentName != "Alice" {
		t.Errorf("unexpected Bob ready state: %+v", bReadyState)
	}
}

func TestWebSocket_HostCancel(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, _, _ := connectAndHandshake(t, ctx, server.URL, "Host", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	connBrowser, _, _ := connectAndHandshake(t, ctx, server.URL, "Browser", "")
	defer func() { _ = connBrowser.Close(websocket.StatusNormalClosure, "done") }()

	// Host creates room
	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)

	_, pHost := readEnvelope(t, ctx, connHost) // room.state
	var roomState lobby.RoomStateView
	_ = json.Unmarshal(pHost, &roomState)

	_, _ = readEnvelope(t, ctx, connBrowser) // lobby.update (upsert)

	// Host cancels
	cancelMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCancel, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, cancelMsg)

	// Host receives lobby.snapshot
	mTypeH, pH := readEnvelope(t, ctx, connHost)
	if mTypeH != TypeLobbySnapshot {
		t.Fatalf("expected %s for host after cancel, got %s", TypeLobbySnapshot, mTypeH)
	}
	var hostSnap lobby.LobbySnapshotPayload
	_ = json.Unmarshal(pH, &hostSnap)
	if len(hostSnap.Rooms) != 0 {
		t.Errorf("expected 0 rooms in host snapshot after cancel, got %d", len(hostSnap.Rooms))
	}

	// Browser receives lobby.update (action: remove)
	mTypeB, pB := readEnvelope(t, ctx, connBrowser)
	if mTypeB != TypeLobbyUpdate {
		t.Fatalf("expected %s for browser, got %s", TypeLobbyUpdate, mTypeB)
	}
	var removeUpdate lobby.LobbyUpdatePayload
	_ = json.Unmarshal(pB, &removeUpdate)
	if removeUpdate.Action != "remove" || removeUpdate.RoomID != roomState.RoomID {
		t.Errorf("unexpected remove update: %+v", removeUpdate)
	}
}

func TestWebSocket_SimultaneousJoinRace(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	// Host creates room
	connHost, _, _ := connectAndHandshake(t, ctx, server.URL, "Host", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)

	_, pHost := readEnvelope(t, ctx, connHost)
	var roomState lobby.RoomStateView
	_ = json.Unmarshal(pHost, &roomState)
	roomID := roomState.RoomID

	// Two joiners connect
	connJ1, _, _ := connectAndHandshake(t, ctx, server.URL, "Joiner1", "")
	defer func() { _ = connJ1.Close(websocket.StatusNormalClosure, "done") }()

	connJ2, _, _ := connectAndHandshake(t, ctx, server.URL, "Joiner2", "")
	defer func() { _ = connJ2.Close(websocket.StatusNormalClosure, "done") }()

	var wg sync.WaitGroup
	wg.Add(2)
	barrier := make(chan struct{})

	var j1Result MessageType
	var j2Result MessageType
	var j1ErrCode string
	var j2ErrCode string

	joinPayload, _ := json.Marshal(OutboundMessage{
		Type:    TypeLobbyJoin,
		Payload: LobbyJoinPayload{RoomID: roomID},
	})

	readJoinResponse := func(conn *websocket.Conn) (MessageType, string) {
		for {
			mType, p := readEnvelope(t, ctx, conn)
			if mType == TypeRoomState {
				return mType, ""
			}
			if mType == TypeError {
				var ep ErrorPayload
				_ = json.Unmarshal(p, &ep)
				return mType, ep.Code
			}
		}
	}

	go func() {
		defer wg.Done()
		<-barrier
		_ = connJ1.Write(ctx, websocket.MessageText, joinPayload)
		j1Result, j1ErrCode = readJoinResponse(connJ1)
	}()

	go func() {
		defer wg.Done()
		<-barrier
		_ = connJ2.Write(ctx, websocket.MessageText, joinPayload)
		j2Result, j2ErrCode = readJoinResponse(connJ2)
	}()

	close(barrier)
	wg.Wait()

	// Exactly one joiner must get room.state and the other must get error ROOM_NOT_AVAILABLE
	if j1Result == TypeRoomState {
		if j2Result != TypeError || j2ErrCode != ErrCodeRoomNotAvailable {
			t.Errorf("j1 succeeded, expected j2 to get ROOM_NOT_AVAILABLE, got %s (%s)", j2Result, j2ErrCode)
		}
	} else if j2Result == TypeRoomState {
		if j1Result != TypeError || j1ErrCode != ErrCodeRoomNotAvailable {
			t.Errorf("j2 succeeded, expected j1 to get ROOM_NOT_AVAILABLE, got %s (%s)", j1Result, j1ErrCode)
		}
	} else {
		t.Fatalf("neither joiner succeeded: j1=%s, j2=%s", j1Result, j2Result)
	}
}

func TestWebSocket_RoomCap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _ := setupTestServer(t, lobby.WithMaxRooms(2))

	c1, _, _ := connectAndHandshake(t, ctx, server.URL, "P1", "")
	defer func() { _ = c1.Close(websocket.StatusNormalClosure, "done") }()

	c2, _, _ := connectAndHandshake(t, ctx, server.URL, "P2", "")
	defer func() { _ = c2.Close(websocket.StatusNormalClosure, "done") }()

	c3, _, _ := connectAndHandshake(t, ctx, server.URL, "P3", "")
	defer func() { _ = c3.Close(websocket.StatusNormalClosure, "done") }()

	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})

	_ = c1.Write(ctx, websocket.MessageText, createMsg)
	t1, _ := readEnvelope(t, ctx, c1)
	if t1 != TypeRoomState {
		t.Fatalf("p1 expected room.state, got %s", t1)
	}

	_ = c2.Write(ctx, websocket.MessageText, createMsg)
	t2, _ := readEnvelope(t, ctx, c2)
	// c2 might have received lobby.update for c1's creation first!
	if t2 == TypeLobbyUpdate {
		t2, _ = readEnvelope(t, ctx, c2)
	}
	if t2 != TypeRoomState {
		t.Fatalf("p2 expected room.state, got %s", t2)
	}

	// 3rd room should hit ROOM_CAP_REACHED
	_ = c3.Write(ctx, websocket.MessageText, createMsg)
	t3, p3 := readEnvelope(t, ctx, c3)
	// Drain any lobby.updates
	for t3 == TypeLobbyUpdate {
		t3, p3 = readEnvelope(t, ctx, c3)
	}

	if t3 != TypeError {
		t.Fatalf("p3 expected error, got %s", t3)
	}
	var ep ErrorPayload
	_ = json.Unmarshal(p3, &ep)
	if ep.Code != ErrCodeRoomCapReached {
		t.Errorf("expected error code %s, got %s", ErrCodeRoomCapReached, ep.Code)
	}
}

func TestWebSocket_ReadyUpFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, _, _ := connectAndHandshake(t, ctx, server.URL, "Host", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	connGuest, _, _ := connectAndHandshake(t, ctx, server.URL, "Guest", "")
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	// Host creates room
	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)

	typ, pBytes := readEnvelope(t, ctx, connHost)
	if typ != TypeRoomState {
		t.Fatalf("expected room.state for host, got %s", typ)
	}
	var hostRoomState lobby.RoomStateView
	_ = json.Unmarshal(pBytes, &hostRoomState)
	roomID := hostRoomState.RoomID

	// Guest receives lobby.update upsert
	typG, _ := readEnvelope(t, ctx, connGuest)
	if typG != TypeLobbyUpdate {
		t.Fatalf("expected lobby.update for guest, got %s", typG)
	}

	// Guest joins room
	joinMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyJoin, Payload: LobbyJoinPayload{RoomID: roomID}})
	_ = connGuest.Write(ctx, websocket.MessageText, joinMsg)

	// Both receive room.state and room.readyState
	typG1, _ := readEnvelope(t, ctx, connGuest)
	if typG1 != TypeRoomState {
		t.Fatalf("guest expected room.state, got %s", typG1)
	}
	typG2, pG2 := readEnvelope(t, ctx, connGuest)
	if typG2 != TypeRoomReadyState {
		t.Fatalf("guest expected room.readyState, got %s", typG2)
	}
	var gReady lobby.RoomReadyStateView
	_ = json.Unmarshal(pG2, &gReady)
	if gReady.HostReady || gReady.GuestReady {
		t.Fatalf("expected both unready initially")
	}

	typH1, _ := readEnvelope(t, ctx, connHost)
	if typH1 != TypeRoomState {
		t.Fatalf("host expected room.state, got %s", typH1)
	}
	typH2, _ := readEnvelope(t, ctx, connHost)
	if typH2 != TypeRoomReadyState {
		t.Fatalf("host expected room.readyState, got %s", typH2)
	}

	// Guest readies up
	readyMsg, _ := json.Marshal(OutboundMessage{Type: TypeRoomReady, Payload: struct{}{}})
	_ = connGuest.Write(ctx, websocket.MessageText, readyMsg)

	// Both should receive room.readyState with guestReady: true
	_, pGuestR := readEnvelope(t, ctx, connGuest)
	_ = json.Unmarshal(pGuestR, &gReady)
	if !gReady.GuestReady || gReady.HostReady {
		t.Errorf("guest expected guestReady=true, hostReady=false, got %+v", gReady)
	}

	_, pHostR := readEnvelope(t, ctx, connHost)
	var hReady lobby.RoomReadyStateView
	_ = json.Unmarshal(pHostR, &hReady)
	if !hReady.GuestReady || hReady.HostReady {
		t.Errorf("host expected guestReady=true, hostReady=false, got %+v", hReady)
	}

	// Host readies up -> both ready -> transition to placement
	_ = connHost.Write(ctx, websocket.MessageText, readyMsg)

	// Host receives readyState then placement room.state
	tHostR, _ := readEnvelope(t, ctx, connHost)
	if tHostR != TypeRoomReadyState {
		t.Fatalf("host expected room.readyState, got %s", tHostR)
	}
	tHostState, pHostState := readEnvelope(t, ctx, connHost)
	if tHostState != TypeRoomState {
		t.Fatalf("host expected room.state, got %s", tHostState)
	}
	var hostPlacement lobby.RoomStateView
	_ = json.Unmarshal(pHostState, &hostPlacement)
	if hostPlacement.Phase != lobby.PhasePlacement {
		t.Errorf("host expected phase placement, got %s", hostPlacement.Phase)
	}

	// Guest receives readyState then placement room.state
	tGuestR, _ := readEnvelope(t, ctx, connGuest)
	if tGuestR != TypeRoomReadyState {
		t.Fatalf("guest expected room.readyState, got %s", tGuestR)
	}
	tGuestState, pGuestState := readEnvelope(t, ctx, connGuest)
	if tGuestState != TypeRoomState {
		t.Fatalf("guest expected room.state, got %s", tGuestState)
	}
	var guestPlacement lobby.RoomStateView
	_ = json.Unmarshal(pGuestState, &guestPlacement)
	if guestPlacement.Phase != lobby.PhasePlacement {
		t.Errorf("guest expected phase placement, got %s", guestPlacement.Phase)
	}
}

func TestWebSocket_ReadyUp_GuestLeave(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, _, _ := connectAndHandshake(t, ctx, server.URL, "Host", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	connGuest, _, _ := connectAndHandshake(t, ctx, server.URL, "Guest", "")
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)
	_, pBytes := readEnvelope(t, ctx, connHost)
	var hostRoomState lobby.RoomStateView
	_ = json.Unmarshal(pBytes, &hostRoomState)
	roomID := hostRoomState.RoomID

	// Drain guest lobby.update
	_, _ = readEnvelope(t, ctx, connGuest)

	joinMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyJoin, Payload: LobbyJoinPayload{RoomID: roomID}})
	_ = connGuest.Write(ctx, websocket.MessageText, joinMsg)

	// Drain room.state and room.readyState for both
	_, _ = readEnvelope(t, ctx, connGuest)
	_, _ = readEnvelope(t, ctx, connGuest)
	_, _ = readEnvelope(t, ctx, connHost)
	_, _ = readEnvelope(t, ctx, connHost)

	// Guest leaves room
	leaveMsg, _ := json.Marshal(OutboundMessage{Type: TypeRoomLeave, Payload: struct{}{}})
	_ = connGuest.Write(ctx, websocket.MessageText, leaveMsg)

	// Host receives waiting room.state
	tHost, pHost := readEnvelope(t, ctx, connHost)
	if tHost != TypeRoomState {
		t.Fatalf("host expected room.state, got %s", tHost)
	}
	var hState lobby.RoomStateView
	_ = json.Unmarshal(pHost, &hState)
	if hState.Phase != lobby.PhaseWaiting {
		t.Errorf("host expected phase waiting, got %s", hState.Phase)
	}

	// Guest receives lobby.snapshot (or lobby.update first)
	tGuest, _ := readEnvelope(t, ctx, connGuest)
	switch tGuest {
	case TypeLobbySnapshot:
		// all good
	case TypeLobbyUpdate:
		tGuest, _ = readEnvelope(t, ctx, connGuest)
		if tGuest != TypeLobbySnapshot {
			t.Errorf("guest expected lobby.snapshot, got %s", tGuest)
		}
	default:
		t.Errorf("guest expected lobby.snapshot, got %s", tGuest)
	}
}

func TestWebSocket_ReadyUp_HostLeave(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, _, _ := connectAndHandshake(t, ctx, server.URL, "Host", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	connGuest, _, _ := connectAndHandshake(t, ctx, server.URL, "Guest", "")
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)
	_, pBytes := readEnvelope(t, ctx, connHost)
	var hostRoomState lobby.RoomStateView
	_ = json.Unmarshal(pBytes, &hostRoomState)
	roomID := hostRoomState.RoomID

	// Drain guest lobby.update
	_, _ = readEnvelope(t, ctx, connGuest)

	joinMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyJoin, Payload: LobbyJoinPayload{RoomID: roomID}})
	_ = connGuest.Write(ctx, websocket.MessageText, joinMsg)

	// Drain room.state and room.readyState for both
	_, _ = readEnvelope(t, ctx, connGuest)
	_, _ = readEnvelope(t, ctx, connGuest)
	_, _ = readEnvelope(t, ctx, connHost)
	_, _ = readEnvelope(t, ctx, connHost)

	// Host leaves room
	leaveMsg, _ := json.Marshal(OutboundMessage{Type: TypeRoomLeave, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, leaveMsg)

	// Host receives lobby.snapshot
	tHost, _ := readEnvelope(t, ctx, connHost)
	if tHost != TypeLobbySnapshot {
		t.Fatalf("host expected lobby.snapshot, got %s", tHost)
	}

	// Guest receives lobby.snapshot
	tGuest, _ := readEnvelope(t, ctx, connGuest)
	if tGuest != TypeLobbySnapshot {
		t.Fatalf("guest expected lobby.snapshot, got %s", tGuest)
	}
}

func TestWebSocket_PlacementSubmitAndMutualConfirmation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, _, _ := connectAndHandshake(t, ctx, server.URL, "HostAdmiral", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	connGuest, _, _ := connectAndHandshake(t, ctx, server.URL, "GuestCaptain", "")
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	// Host creates room
	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)
	_, pBytes := readEnvelope(t, ctx, connHost)
	var hostRoomState lobby.RoomStateView
	_ = json.Unmarshal(pBytes, &hostRoomState)
	roomID := hostRoomState.RoomID

	// Drain guest lobby.update
	_, _ = readEnvelope(t, ctx, connGuest)

	// Guest joins
	joinMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyJoin, Payload: LobbyJoinPayload{RoomID: roomID}})
	_ = connGuest.Write(ctx, websocket.MessageText, joinMsg)

	// Drain room.state and room.readyState for both
	_, _ = readEnvelope(t, ctx, connGuest)
	_, _ = readEnvelope(t, ctx, connGuest)
	_, _ = readEnvelope(t, ctx, connHost)
	_, _ = readEnvelope(t, ctx, connHost)

	// Both ready up
	readyMsg, _ := json.Marshal(OutboundMessage{Type: TypeRoomReady, Payload: struct{}{}})
	_ = connHost.Write(ctx, websocket.MessageText, readyMsg)
	// Drain readyStates
	_, _ = readEnvelope(t, ctx, connHost)
	_, _ = readEnvelope(t, ctx, connGuest)

	_ = connGuest.Write(ctx, websocket.MessageText, readyMsg)
	// Drain readyStates
	_, _ = readEnvelope(t, ctx, connHost)
	_, _ = readEnvelope(t, ctx, connGuest)

	// Both receive room.state with phase: placement
	tHostPhase, pHState := readEnvelope(t, ctx, connHost)
	if tHostPhase != TypeRoomState {
		t.Fatalf("host expected room.state, got %s", tHostPhase)
	}
	var hPlacementState lobby.RoomStateView
	_ = json.Unmarshal(pHState, &hPlacementState)
	if hPlacementState.Phase != lobby.PhasePlacement || hPlacementState.PlacementDeadline == nil {
		t.Fatalf("expected placement phase with deadline, got phase=%s, deadline=%v", hPlacementState.Phase, hPlacementState.PlacementDeadline)
	}

	tGuestPhase, pGState := readEnvelope(t, ctx, connGuest)
	if tGuestPhase != TypeRoomState {
		t.Fatalf("guest expected room.state, got %s", tGuestPhase)
	}
	var gPlacementState lobby.RoomStateView
	_ = json.Unmarshal(pGState, &gPlacementState)
	if gPlacementState.Phase != lobby.PhasePlacement {
		t.Fatalf("expected placement phase for guest, got %s", gPlacementState.Phase)
	}

	// Host submits valid fleet
	hostFleet := []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 0, Y: 0}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipBattleship, Origin: game.Coord{X: 0, Y: 1}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipCruiser, Origin: game.Coord{X: 0, Y: 2}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 0, Y: 3}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 0, Y: 4}, Orientation: game.OrientationHorizontal},
	}
	submitMsg, _ := json.Marshal(OutboundMessage{
		Type:    TypePlacementSubmit,
		Payload: PlacementSubmitPayload{Ships: hostFleet},
	})
	_ = connHost.Write(ctx, websocket.MessageText, submitMsg)

	// Host receives placement.confirmed
	tHConf, pHConf := readEnvelope(t, ctx, connHost)
	if tHConf != TypePlacementConfirmed {
		t.Fatalf("host expected placement.confirmed, got %s", tHConf)
	}
	var hConf PlacementConfirmedPayload
	_ = json.Unmarshal(pHConf, &hConf)
	if !hConf.Confirmed || len(hConf.Ships) != 5 {
		t.Fatalf("expected 5 confirmed ships for host, got %+v", hConf)
	}

	// Guest receives placement.opponentReady (and NO ship coordinates!)
	tGOpp, pGOpp := readEnvelope(t, ctx, connGuest)
	if tGOpp != TypePlacementOpponentReady {
		t.Fatalf("guest expected placement.opponentReady, got %s", tGOpp)
	}
	var oppReadyMap map[string]any
	_ = json.Unmarshal(pGOpp, &oppReadyMap)
	if _, leaked := oppReadyMap["ships"]; leaked {
		t.Fatal("SECURITY LEAK: guest received host ship coordinates in placement.opponentReady!")
	}

	// Guest submits valid fleet
	guestFleet := []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 5, Y: 0}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipBattleship, Origin: game.Coord{X: 5, Y: 1}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipCruiser, Origin: game.Coord{X: 5, Y: 2}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 5, Y: 3}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 5, Y: 4}, Orientation: game.OrientationHorizontal},
	}
	submitMsgGuest, _ := json.Marshal(OutboundMessage{
		Type:    TypePlacementSubmit,
		Payload: PlacementSubmitPayload{Ships: guestFleet},
	})
	_ = connGuest.Write(ctx, websocket.MessageText, submitMsgGuest)

	// Guest receives placement.confirmed
	tGConf, _ := readEnvelope(t, ctx, connGuest)
	if tGConf != TypePlacementConfirmed {
		t.Fatalf("guest expected placement.confirmed, got %s", tGConf)
	}

	// Host receives placement.opponentReady
	tHOpp, _ := readEnvelope(t, ctx, connHost)
	if tHOpp != TypePlacementOpponentReady {
		t.Fatalf("host expected placement.opponentReady, got %s", tHOpp)
	}

	// Both receive battle.start, battle.turn, and room.state transitioning to PhaseBattle
	tHStart, _ := readEnvelope(t, ctx, connHost)
	if tHStart != TypeBattleStart {
		t.Fatalf("host expected battle.start, got %s", tHStart)
	}
	tGStart, _ := readEnvelope(t, ctx, connGuest)
	if tGStart != TypeBattleStart {
		t.Fatalf("guest expected battle.start, got %s", tGStart)
	}

	tHTurn, _ := readEnvelope(t, ctx, connHost)
	if tHTurn != TypeBattleTurn {
		t.Fatalf("host expected battle.turn, got %s", tHTurn)
	}
	tGTurn, _ := readEnvelope(t, ctx, connGuest)
	if tGTurn != TypeBattleTurn {
		t.Fatalf("guest expected battle.turn, got %s", tGTurn)
	}

	tHBattle, pHBattle := readEnvelope(t, ctx, connHost)
	if tHBattle != TypeRoomState {
		t.Fatalf("host expected room.state battle transition, got %s", tHBattle)
	}
	var hBattleState lobby.RoomStateView
	_ = json.Unmarshal(pHBattle, &hBattleState)
	if hBattleState.Phase != lobby.PhaseBattle {
		t.Fatalf("expected host battle phase, got %s", hBattleState.Phase)
	}

	tGBattle, pGBattle := readEnvelope(t, ctx, connGuest)
	if tGBattle != TypeRoomState {
		t.Fatalf("guest expected room.state battle transition, got %s", tGBattle)
	}
	var gBattleState lobby.RoomStateView
	_ = json.Unmarshal(pGBattle, &gBattleState)
	if gBattleState.Phase != lobby.PhaseBattle {
		t.Fatalf("expected guest battle phase, got %s", gBattleState.Phase)
	}
}

func testFleetForWS() []game.PlacedShip {
	return []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 0, Y: 0}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipBattleship, Origin: game.Coord{X: 0, Y: 1}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipCruiser, Origin: game.Coord{X: 0, Y: 2}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 0, Y: 3}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 0, Y: 4}, Orientation: game.OrientationHorizontal},
	}
}

func testGuestFleetForWS() []game.PlacedShip {
	return []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 5, Y: 0}, Orientation: game.OrientationVertical},
		{Type: game.ShipBattleship, Origin: game.Coord{X: 6, Y: 0}, Orientation: game.OrientationVertical},
		{Type: game.ShipCruiser, Origin: game.Coord{X: 7, Y: 0}, Orientation: game.OrientationVertical},
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 8, Y: 0}, Orientation: game.OrientationVertical},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 9, Y: 0}, Orientation: game.OrientationVertical},
	}
}

func TestWebSocket_BattleFireAndTurnLoop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, playerHost, _ := connectAndHandshake(t, ctx, server.URL, "HostShooter", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	// Host creates room
	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: map[string]any{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)
	_, pHostState := readEnvelope(t, ctx, connHost)
	var hostRoomState lobby.RoomStateView
	_ = json.Unmarshal(pHostState, &hostRoomState)

	// Guest connects and joins
	connGuest, playerGuest, _ := connectAndHandshake(t, ctx, server.URL, "GuestShooter", "")
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	joinMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyJoin, Payload: LobbyJoinPayload{RoomID: hostRoomState.RoomID}})
	_ = connGuest.Write(ctx, websocket.MessageText, joinMsg)
	_, _ = readEnvelope(t, ctx, connGuest) // guest room.state
	_, _ = readEnvelope(t, ctx, connGuest) // guest room.readyState
	_, _ = readEnvelope(t, ctx, connHost)  // host room.state
	_, _ = readEnvelope(t, ctx, connHost)  // host room.readyState

	// Ready-up
	readyMsg, _ := json.Marshal(OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})
	_ = connHost.Write(ctx, websocket.MessageText, readyMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // host readyState
	_, _ = readEnvelope(t, ctx, connGuest) // guest readyState

	_ = connGuest.Write(ctx, websocket.MessageText, readyMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // host readyState
	_, _ = readEnvelope(t, ctx, connGuest) // guest readyState
	_, _ = readEnvelope(t, ctx, connHost)  // host room.state (placement)
	_, _ = readEnvelope(t, ctx, connGuest) // guest room.state (placement)

	// Both submit fleets
	fleetHost := testFleetForWS()
	fleetGuest := testGuestFleetForWS()

	subHost, _ := json.Marshal(OutboundMessage{Type: TypePlacementSubmit, Payload: PlacementSubmitPayload{Ships: fleetHost}})
	_ = connHost.Write(ctx, websocket.MessageText, subHost)
	_, _ = readEnvelope(t, ctx, connHost)  // placement.confirmed
	_, _ = readEnvelope(t, ctx, connGuest) // placement.opponentReady

	subGuest, _ := json.Marshal(OutboundMessage{Type: TypePlacementSubmit, Payload: PlacementSubmitPayload{Ships: fleetGuest}})
	_ = connGuest.Write(ctx, websocket.MessageText, subGuest)
	_, _ = readEnvelope(t, ctx, connGuest) // placement.confirmed
	_, _ = readEnvelope(t, ctx, connHost)  // placement.opponentReady

	// Both receive battle.start and battle.turn
	tStartH, _ := readEnvelope(t, ctx, connHost)
	if tStartH != TypeBattleStart {
		t.Fatalf("expected battle.start on host, got %s", tStartH)
	}
	tStartG, _ := readEnvelope(t, ctx, connGuest)
	if tStartG != TypeBattleStart {
		t.Fatalf("expected battle.start on guest, got %s", tStartG)
	}

	tTurnH, pTurnH := readEnvelope(t, ctx, connHost)
	if tTurnH != TypeBattleTurn {
		t.Fatalf("expected battle.turn on host, got %s", tTurnH)
	}
	tTurnG, _ := readEnvelope(t, ctx, connGuest)
	if tTurnG != TypeBattleTurn {
		t.Fatalf("expected battle.turn on guest, got %s", tTurnG)
	}

	tStateH, _ := readEnvelope(t, ctx, connHost)
	if tStateH != TypeRoomState {
		t.Fatalf("expected room.state on host, got %s", tStateH)
	}
	tStateG, _ := readEnvelope(t, ctx, connGuest)
	if tStateG != TypeRoomState {
		t.Fatalf("expected room.state on guest, got %s", tStateG)
	}

	var turnPayload BattleTurnPayload
	if err := json.Unmarshal(pTurnH, &turnPayload); err != nil {
		t.Fatalf("failed to unmarshal turnPayload: %v", err)
	}
	if turnPayload.TurnPlayerID == "" {
		t.Fatalf("turnPlayerId is empty in turnPayload: %s", string(pTurnH))
	}

	firstConn := connHost
	firstID := playerHost.PlayerID
	secondConn := connGuest
	secondID := playerGuest.PlayerID
	if turnPayload.TurnPlayerID == playerGuest.PlayerID {
		firstConn = connGuest
		firstID = playerGuest.PlayerID
		secondConn = connHost
		secondID = playerHost.PlayerID
	}

	// 1. Inactive player fires out of turn -> returns NOT_YOUR_TURN error
	badFire, _ := json.Marshal(OutboundMessage{Type: TypeBattleFire, Payload: BattleFirePayload{Cell: game.Coord{X: 0, Y: 0}}})
	_ = secondConn.Write(ctx, websocket.MessageText, badFire)
	tErr, pErr := readEnvelope(t, ctx, secondConn)
	if tErr != TypeError {
		t.Fatalf("expected error message on out-of-turn fire, got %s", tErr)
	}
	var errPayload ErrorPayload
	_ = json.Unmarshal(pErr, &errPayload)
	if errPayload.Code != ErrCodeNotYourTurn {
		t.Errorf("expected NOT_YOUR_TURN code, got %s", errPayload.Code)
	}

	// 2. Active player fires valid shot at (0, 9)
	validFire, _ := json.Marshal(OutboundMessage{Type: TypeBattleFire, Payload: BattleFirePayload{Cell: game.Coord{X: 0, Y: 9}}})
	_ = firstConn.Write(ctx, websocket.MessageText, validFire)

	// Both receive battle.shotResult
	tRes1, pRes1 := readEnvelope(t, ctx, firstConn)
	if tRes1 != TypeBattleShotResult {
		t.Fatalf("expected battle.shotResult on first conn, got %s", tRes1)
	}
	var res1 BattleShotResultPayload
	_ = json.Unmarshal(pRes1, &res1)
	if res1.By != firstID {
		t.Errorf("expected shooter %s, got %s", firstID, res1.By)
	}
	if res1.Result != game.ShotResultMiss {
		t.Errorf("expected miss at (0,9), got %s", res1.Result)
	}

	tRes2, _ := readEnvelope(t, ctx, secondConn)
	if tRes2 != TypeBattleShotResult {
		t.Fatalf("expected battle.shotResult on second conn, got %s", tRes2)
	}

	// Both receive battle.turn for second player
	tNextTurn1, pNextTurn1 := readEnvelope(t, ctx, firstConn)
	if tNextTurn1 != TypeBattleTurn {
		t.Fatalf("expected battle.turn on first conn, got %s", tNextTurn1)
	}
	var nextTurn1 BattleTurnPayload
	_ = json.Unmarshal(pNextTurn1, &nextTurn1)
	if nextTurn1.TurnPlayerID != secondID {
		t.Errorf("expected turn to switch to %s, got %s", secondID, nextTurn1.TurnPlayerID)
	}

	_, _ = readEnvelope(t, ctx, secondConn) // battle.turn
	_, _ = readEnvelope(t, ctx, firstConn)  // room.state
	_, _ = readEnvelope(t, ctx, secondConn) // room.state
}

func validFleet() []game.PlacedShip {
	return []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 0, Y: 0}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipBattleship, Origin: game.Coord{X: 0, Y: 1}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipCruiser, Origin: game.Coord{X: 0, Y: 2}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 0, Y: 3}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 0, Y: 4}, Orientation: game.OrientationHorizontal},
	}
}

func TestHandler_GameLeave_ForfeitInBattle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, _, mgr := setupTestServer(t)

	connHost, playerHost, _ := connectAndHandshake(t, ctx, server.URL, "HostPlayer", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: map[string]any{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)
	_, pRoomState := readEnvelope(t, ctx, connHost)
	var roomState lobby.RoomStateView
	_ = json.Unmarshal(pRoomState, &roomState)
	roomID := roomState.RoomID

	connGuest, playerGuest, _ := connectAndHandshake(t, ctx, server.URL, "GuestPlayer", "")
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	joinMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyJoin, Payload: LobbyJoinPayload{RoomID: roomID}})
	_ = connGuest.Write(ctx, websocket.MessageText, joinMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // room.state
	_, _ = readEnvelope(t, ctx, connHost)  // room.readyState
	_, _ = readEnvelope(t, ctx, connGuest) // room.state
	_, _ = readEnvelope(t, ctx, connGuest) // room.readyState

	// Ready up
	readyMsg, _ := json.Marshal(OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})
	_ = connHost.Write(ctx, websocket.MessageText, readyMsg)
	_, _ = readEnvelope(t, ctx, connHost)
	_, _ = readEnvelope(t, ctx, connGuest)

	_ = connGuest.Write(ctx, websocket.MessageText, readyMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // room.readyState
	_, _ = readEnvelope(t, ctx, connGuest) // room.readyState
	_, _ = readEnvelope(t, ctx, connHost)  // room.state (placement)
	_, _ = readEnvelope(t, ctx, connGuest) // room.state (placement)

	// Both submit fleets
	fleet := validFleet()
	submitMsg, _ := json.Marshal(OutboundMessage{Type: TypePlacementSubmit, Payload: PlacementSubmitPayload{Ships: fleet}})
	_ = connHost.Write(ctx, websocket.MessageText, submitMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // placement.confirmed
	_, _ = readEnvelope(t, ctx, connGuest) // placement.opponentReady

	_ = connGuest.Write(ctx, websocket.MessageText, submitMsg)
	_, _ = readEnvelope(t, ctx, connGuest) // placement.confirmed
	_, _ = readEnvelope(t, ctx, connHost)  // placement.opponentReady

	// Both advance to battle
	_, _ = readEnvelope(t, ctx, connHost)  // battle.start
	_, _ = readEnvelope(t, ctx, connGuest) // battle.start
	_, _ = readEnvelope(t, ctx, connHost)  // battle.turn
	_, _ = readEnvelope(t, ctx, connGuest) // battle.turn
	_, _ = readEnvelope(t, ctx, connHost)  // room.state
	_, _ = readEnvelope(t, ctx, connGuest) // room.state

	// Guest sends game.leave (forfeits match)
	leaveMsg, _ := json.Marshal(OutboundMessage{Type: TypeGameLeave, Payload: map[string]any{}})
	_ = connGuest.Write(ctx, websocket.MessageText, leaveMsg)

	// Guest receives lobby.snapshot
	tGuestLeave, _ := readEnvelope(t, ctx, connGuest)
	if tGuestLeave != TypeLobbySnapshot {
		t.Fatalf("expected guest to receive lobby.snapshot after game.leave, got %s", tGuestLeave)
	}

	// Host receives game.over with reason forfeit
	tHostGameOver, pHostGameOver := readEnvelope(t, ctx, connHost)
	if tHostGameOver != TypeGameOver {
		t.Fatalf("expected host to receive game.over, got %s", tHostGameOver)
	}
	var gameOver GameOverPayload
	_ = json.Unmarshal(pHostGameOver, &gameOver)
	if gameOver.WinnerPlayerID != playerHost.PlayerID {
		t.Errorf("expected winner to be host (%s), got %s", playerHost.PlayerID, gameOver.WinnerPlayerID)
	}
	if gameOver.Reason != "forfeit" {
		t.Errorf("expected reason forfeit, got %s", gameOver.Reason)
	}

	// Host receives updated room.state
	tHostState, _ := readEnvelope(t, ctx, connHost)
	if tHostState != TypeRoomState {
		t.Fatalf("expected host to receive room.state, got %s", tHostState)
	}

	_ = playerGuest
	_ = mgr
}

func TestHandler_HeartbeatPing(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reg := lobby.NewSessionRegistry()
	mgr := lobby.NewLobbyManager(reg)
	// Fast ping interval to verify heartbeat loop runs cleanly
	handler := NewHandler(reg, mgr, WithPingConfig(50*time.Millisecond, 200*time.Millisecond))
	server := httptest.NewServer(handler)
	defer server.Close()

	conn, player, _ := connectAndHandshake(t, ctx, server.URL, "PingTester", "")
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	// Sleep for a short while so multiple pings are exchanged
	time.Sleep(200 * time.Millisecond)

	// Connection should still be functional and connected
	p, ok := reg.GetByToken(player.SessionToken)
	if !ok || p.ConnectionState != lobby.StateConnected {
		t.Fatalf("expected player to be connected, got %v", p)
	}
}

func TestHandler_ReconnectInPlacementRestoresState(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reg := lobby.NewSessionRegistry()
	mgr := lobby.NewLobbyManager(reg)
	handler := NewHandler(reg, mgr)
	server := httptest.NewServer(handler)
	defer server.Close()

	// 1. Connect Host and Guest
	connHost, playerHost, _ := connectAndHandshake(t, ctx, server.URL, "HostAdmiral", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	connGuest, playerGuest, _ := connectAndHandshake(t, ctx, server.URL, "GuestCommander", "")

	// 2. Host creates room
	createMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyCreate, Payload: map[string]any{}})
	_ = connHost.Write(ctx, websocket.MessageText, createMsg)
	_, pCreate := readEnvelope(t, ctx, connHost)
	var roomStateHost lobby.RoomStateView
	_ = json.Unmarshal(pCreate, &roomStateHost)
	roomID := roomStateHost.RoomID
	_, _ = readEnvelope(t, ctx, connGuest) // guest receives lobby.update upsert

	// 3. Guest joins room
	joinMsg, _ := json.Marshal(OutboundMessage{Type: TypeLobbyJoin, Payload: LobbyJoinPayload{RoomID: roomID}})
	_ = connGuest.Write(ctx, websocket.MessageText, joinMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // host receives room.state
	_, _ = readEnvelope(t, ctx, connHost)  // host receives room.readyState
	_, _ = readEnvelope(t, ctx, connGuest) // guest receives room.state
	_, _ = readEnvelope(t, ctx, connGuest) // guest receives room.readyState

	// 4. Both ready up -> transitions to placement
	readyMsg, _ := json.Marshal(OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})
	_ = connHost.Write(ctx, websocket.MessageText, readyMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // room.readyState
	_, _ = readEnvelope(t, ctx, connGuest) // room.readyState

	_ = connGuest.Write(ctx, websocket.MessageText, readyMsg)
	_, _ = readEnvelope(t, ctx, connHost)  // room.readyState
	_, _ = readEnvelope(t, ctx, connGuest) // room.readyState
	_, _ = readEnvelope(t, ctx, connHost)  // room.state (placement)
	_, _ = readEnvelope(t, ctx, connGuest) // room.state (placement)

	// 5. Guest closes tab / drops connection
	_ = connGuest.Close(websocket.StatusNormalClosure, "leaving tab")

	// Host should receive opponent.disconnected
	tHostDisconnect, _ := readEnvelope(t, ctx, connHost)
	if tHostDisconnect != TypeOpponentDisconnected {
		t.Fatalf("expected host to receive opponent.disconnected, got %s", tHostDisconnect)
	}

	// 6. Guest reconnects with their existing session token
	connGuestNew, helloOKGuest, _ := connectAndHandshake(t, ctx, server.URL, "GuestCommander", playerGuest.SessionToken)
	defer func() { _ = connGuestNew.Close(websocket.StatusNormalClosure, "done") }()

	if helloOKGuest.RestoredState == nil {
		t.Fatalf("expected guest to have restoredState in hello.ok, got nil")
	}

	// Verify restoredState marshals cleanly without channel/timer errors and has correct phase
	restoredBytes, err := json.Marshal(helloOKGuest.RestoredState)
	if err != nil {
		t.Fatalf("failed to marshal restoredState: %v", err)
	}

	var restoredView lobby.RoomStateView
	if err := json.Unmarshal(restoredBytes, &restoredView); err != nil {
		t.Fatalf("failed to unmarshal restoredState to RoomStateView: %v", err)
	}

	if restoredView.RoomID != roomID {
		t.Errorf("expected restored roomId %s, got %s", roomID, restoredView.RoomID)
	}
	if restoredView.Phase != lobby.PhasePlacement {
		t.Errorf("expected restored phase %s, got %s", lobby.PhasePlacement, restoredView.Phase)
	}
	if restoredView.Role != "guest" {
		t.Errorf("expected restored role guest, got %s", restoredView.Role)
	}

	// Host should receive opponent.reconnected
	tHostReconnect, _ := readEnvelope(t, ctx, connHost)
	if tHostReconnect != TypeOpponentReconnected {
		t.Fatalf("expected host to receive opponent.reconnected, got %s", tHostReconnect)
	}

	// Guest should also receive room.state over newly established connection
	tGuestState, _ := readEnvelope(t, ctx, connGuestNew)
	if tGuestState != TypeRoomState {
		t.Fatalf("expected guest to receive room.state, got %s", tGuestState)
	}

	_ = playerHost
}

func mustWriteJSON(t *testing.T, ctx context.Context, conn *websocket.Conn, msg OutboundMessage) {
	t.Helper()
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message failed: %v", err)
	}
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write message failed: %v", err)
	}
}

func drainUntilType(t *testing.T, ctx context.Context, conn *websocket.Conn, target MessageType) (MessageType, json.RawMessage) {
	t.Helper()
	for {
		typ, payload := readEnvelope(t, ctx, conn)
		if typ == target {
			return typ, payload
		}
	}
}

func TestHandler_RematchFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, _, mgr := setupTestServer(t)

	connHost, playerHost, _ := connectAndHandshake(t, ctx, server.URL, "HostRematch", "")
	defer func() {
		_ = connHost.Close(websocket.StatusNormalClosure, "done")
	}()

	connGuest, playerGuest, _ := connectAndHandshake(t, ctx, server.URL, "GuestRematch", "")
	defer func() {
		_ = connGuest.Close(websocket.StatusNormalClosure, "done")
	}()

	// Host creates room
	mustWriteJSON(t, ctx, connHost, OutboundMessage{
		Type:    TypeLobbyCreate,
		Payload: map[string]any{},
	})

	var roomStateHost lobby.RoomStateView
	for {
		tMsg, payload := readEnvelope(t, ctx, connHost)
		if tMsg == TypeRoomState {
			if err := json.Unmarshal(payload, &roomStateHost); err != nil {
				t.Fatalf("unmarshal room.state: %v", err)
			}
			break
		}
	}
	roomID := roomStateHost.RoomID

	// Guest joins
	mustWriteJSON(t, ctx, connGuest, OutboundMessage{
		Type:    TypeLobbyJoin,
		Payload: LobbyJoinPayload{RoomID: roomID},
	})
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

	// Both ready up
	mustWriteJSON(t, ctx, connHost, OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})
	mustWriteJSON(t, ctx, connGuest, OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})

	// Drain into placement
	_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

	// Simulate match finish via manager directly to test WebSocket rematch flow cleanly
	mgr.SetRoomFinishedForTesting(roomID, playerHost.PlayerID, "all_sunk")

	// Host sends rematch.accept
	mustWriteJSON(t, ctx, connHost, OutboundMessage{
		Type:    TypeRematchAccept,
		Payload: map[string]any{},
	})

	// Guest receives rematch.offered
	tMsg, pBytes := drainUntilType(t, ctx, connGuest, TypeRematchOffered)
	if tMsg != TypeRematchOffered {
		t.Fatalf("expected guest to receive rematch.offered, got %s", tMsg)
	}
	var offered RematchOfferedPayload
	if err := json.Unmarshal(pBytes, &offered); err != nil {
		t.Fatalf("unmarshal rematch.offered: %v", err)
	}
	if offered.By != playerHost.PlayerID {
		t.Errorf("expected offer by host %s, got %s", playerHost.PlayerID, offered.By)
	}

	// Guest also sends rematch.accept -> mutual rematch restarts into placement
	mustWriteJSON(t, ctx, connGuest, OutboundMessage{
		Type:    TypeRematchAccept,
		Payload: map[string]any{},
	})

	// Both should receive room.state with phase placement
	_, pHostState := drainUntilType(t, ctx, connHost, TypeRoomState)
	var newHostView lobby.RoomStateView
	if err := json.Unmarshal(pHostState, &newHostView); err != nil {
		t.Fatalf("unmarshal host room.state: %v", err)
	}
	if newHostView.Phase != lobby.PhasePlacement {
		t.Errorf("expected host view phase placement on rematch, got %v", newHostView.Phase)
	}

	_, pGuestState := drainUntilType(t, ctx, connGuest, TypeRoomState)
	var newGuestView lobby.RoomStateView
	if err := json.Unmarshal(pGuestState, &newGuestView); err != nil {
		t.Fatalf("unmarshal guest room.state: %v", err)
	}
	if newGuestView.Phase != lobby.PhasePlacement {
		t.Errorf("expected guest view phase placement on rematch, got %v", newGuestView.Phase)
	}

	_ = playerGuest
}

func TestHandler_OriginCheck(t *testing.T) {
	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithAllowedOrigins([]string{"authorized.example.com"}),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// 1. Connection with unauthorized origin should be rejected with 403
	_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"http://unauthorized.evil.com"},
		},
	})
	if err == nil {
		t.Fatal("expected unauthorized origin to be rejected, but dial succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 Forbidden for unauthorized origin, got %d", resp.StatusCode)
	}

	// 2. Connection with authorized origin should succeed
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"http://authorized.example.com"},
		},
	})
	if err != nil {
		t.Fatalf("expected authorized origin dial to succeed, got %v", err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "done")

	// 3. Connection with wildcard origin configured
	serverWildcard, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithAllowedOrigins([]string{"*"}),
	})
	wsURLWildcard := "ws" + strings.TrimPrefix(serverWildcard.URL, "http")

	connW, _, err := websocket.Dial(ctx, wsURLWildcard, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Origin": []string{"http://any-domain.com"},
		},
	})
	if err != nil {
		t.Fatalf("expected wildcard origin dial to succeed, got %v", err)
	}
	_ = connW.Close(websocket.StatusNormalClosure, "done")
}

func TestHandler_MaxPayloadSize(t *testing.T) {
	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithAllowedOrigins([]string{"*"}),
		WithMaxPayloadBytes(128),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Handshake with a short name fits within 128 bytes
	conn, _, _ := connectAndHandshake(t, ctx, server.URL, "Short", "")
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	// Send an oversized payload (> 128 bytes)
	hugeData := strings.Repeat("X", 256)
	hugeMsg := OutboundMessage{
		Type: TypeLobbyJoin,
		Payload: map[string]string{
			"roomId": hugeData,
		},
	}
	err := writeJSON(ctx, conn, hugeMsg)
	if err != nil {
		t.Fatalf("failed to send huge message: %v", err)
	}

	// The next read should return an error because the server closed with StatusMessageTooBig
	_, _, readErr := conn.Read(ctx)
	if readErr == nil {
		t.Fatal("expected read to fail after sending oversized message, but it succeeded")
	}
	t.Logf("connection closed as expected on oversized message: %v", readErr)
}

func TestHandler_RateLimiting(t *testing.T) {
	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithAllowedOrigins([]string{"*"}),
		WithRateLimit(1.0, 1.0), // 1 msg/sec with burst 1
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, _ := connectAndHandshake(t, ctx, server.URL, "RateTest", "")
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	// Send 5 rapid messages
	for i := 0; i < 5; i++ {
		_ = writeJSON(ctx, conn, OutboundMessage{
			Type:    TypeLobbyCreate,
			Payload: map[string]any{},
		})
	}

	// Read responses: at least one should be RATE_LIMIT_EXCEEDED
	sawRateLimitError := false
	for i := 0; i < 5; i++ {
		readCtx, readCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		typ, payload := readEnvelope(t, readCtx, conn)
		readCancel()

		if typ == TypeError {
			var errPayload ErrorPayload
			if err := json.Unmarshal(payload, &errPayload); err == nil {
				if errPayload.Code == ErrCodeRateLimited {
					sawRateLimitError = true
					break
				}
			}
		}
	}

	if !sawRateLimitError {
		t.Error("expected at least one RATE_LIMIT_EXCEEDED error envelope, but none was received")
	}
}

func TestHandler_PerIPConnectionCap(t *testing.T) {
	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithAllowedOrigins([]string{"*"}),
		WithIPLimit(2),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect 1st client
	conn1, _, _ := connectAndHandshake(t, ctx, server.URL, "Player1", "")
	defer func() { _ = conn1.Close(websocket.StatusNormalClosure, "done") }()

	// Connect 2nd client
	conn2, _, _ := connectAndHandshake(t, ctx, server.URL, "Player2", "")
	defer func() { _ = conn2.Close(websocket.StatusNormalClosure, "done") }()

	// Attempt 3rd client from same IP -> should be rejected with 429 Too Many Requests
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected 3rd connection to be rejected by per-IP cap, but dial succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected status 429 Too Many Requests, got %d", resp.StatusCode)
	}

	// Close conn1 and verify capacity frees up
	_ = conn1.Close(websocket.StatusNormalClosure, "done")
	time.Sleep(50 * time.Millisecond) // allow handler cleanup

	// Now 3rd client should succeed
	conn3, _, _ := connectAndHandshake(t, ctx, server.URL, "Player3", "")
	defer func() { _ = conn3.Close(websocket.StatusNormalClosure, "done") }()
}

func TestHandler_GracefulShutdown(t *testing.T) {
	server, _, _, handler := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithAllowedOrigins([]string{"*"}),
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, _ := connectAndHandshake(t, ctx, server.URL, "ShutdownPlayer", "")
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	// Initiate handler shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	if err := handler.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("handler.Shutdown returned error: %v", err)
	}

	// Connected client should see connection closed with StatusGoingAway
	_, _, readErr := conn.Read(ctx)
	if readErr == nil {
		t.Fatal("expected read to fail after server shutdown, but it succeeded")
	}
	t.Logf("connection closed on shutdown with error: %v (status: %v)", readErr, websocket.CloseStatus(readErr))

	// New connection attempt while shutting down should be rejected with 503
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected new connection to be rejected during shutdown, but dial succeeded")
	}
	if resp != nil && resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected status 503 Service Unavailable, got %d", resp.StatusCode)
	}
}

// advanceTwoClientsToBattle connects Host and Guest, creates/joins a room, readies both,
// submits their fleets, and returns both connections and the starting turn player ID.
func advanceTwoClientsToBattle(
	t *testing.T,
	ctx context.Context,
	serverURL string,
	hostName string,
	guestName string,
) (*websocket.Conn, HelloOKPayload, *websocket.Conn, HelloOKPayload, string, string) {
	t.Helper()

	connHost, playerHost, _ := connectAndHandshake(t, ctx, serverURL, hostName, "")
	connGuest, playerGuest, _ := connectAndHandshake(t, ctx, serverURL, guestName, "")

	mustWriteJSON(t, ctx, connHost, OutboundMessage{Type: TypeLobbyCreate, Payload: map[string]any{}})
	_, pCreate := drainUntilType(t, ctx, connHost, TypeRoomState)
	var hostRoom lobby.RoomStateView
	if err := json.Unmarshal(pCreate, &hostRoom); err != nil {
		t.Fatalf("unmarshal host room.state: %v", err)
	}
	roomID := hostRoom.RoomID

	mustWriteJSON(t, ctx, connGuest, OutboundMessage{
		Type:    TypeLobbyJoin,
		Payload: LobbyJoinPayload{RoomID: roomID},
	})
	_, _ = drainUntilType(t, ctx, connHost, TypeRoomReadyState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomReadyState)

	mustWriteJSON(t, ctx, connHost, OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})
	mustWriteJSON(t, ctx, connGuest, OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})

	_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

	mustWriteJSON(t, ctx, connHost, OutboundMessage{
		Type:    TypePlacementSubmit,
		Payload: PlacementSubmitPayload{Ships: testFleetForWS()},
	})
	_, _ = drainUntilType(t, ctx, connHost, TypePlacementConfirmed)
	_, _ = drainUntilType(t, ctx, connGuest, TypePlacementOpponentReady)

	mustWriteJSON(t, ctx, connGuest, OutboundMessage{
		Type:    TypePlacementSubmit,
		Payload: PlacementSubmitPayload{Ships: testGuestFleetForWS()},
	})

	_, pTurnH := drainUntilType(t, ctx, connHost, TypeBattleTurn)
	_, _ = drainUntilType(t, ctx, connGuest, TypeBattleTurn)
	_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

	var turnPayload BattleTurnPayload
	if err := json.Unmarshal(pTurnH, &turnPayload); err != nil {
		t.Fatalf("unmarshal battle.turn: %v", err)
	}

	return connHost, playerHost, connGuest, playerGuest, roomID, turnPayload.TurnPlayerID
}

func TestWebSocket_TwoClientFullGameAndPersistence(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	dbPath := filepath.Join(t.TempDir(), "integration_full_game.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create sqlite store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
	})

	server, _, mgr, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)}, lobby.WithStore(st))

	connHost, playerHost, connGuest, playerGuest, roomID, currentTurnID := advanceTwoClientsToBattle(
		t, ctx, server.URL, "AdmiralNimitz", "CaptainHalsey",
	)
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	// Guest fleet occupies:
	// Carrier (5): (5,0)..(5,4)
	// Battleship (4): (6,0)..(6,3)
	// Cruiser (3): (7,0)..(7,2)
	// Submarine (3): (8,0)..(8,2)
	// Destroyer (2): (9,0)..(9,1)
	guestShipCells := []game.Coord{
		{X: 5, Y: 0}, {X: 5, Y: 1}, {X: 5, Y: 2}, {X: 5, Y: 3}, {X: 5, Y: 4},
		{X: 6, Y: 0}, {X: 6, Y: 1}, {X: 6, Y: 2}, {X: 6, Y: 3},
		{X: 7, Y: 0}, {X: 7, Y: 1}, {X: 7, Y: 2},
		{X: 8, Y: 0}, {X: 8, Y: 1}, {X: 8, Y: 2},
		{X: 9, Y: 0}, {X: 9, Y: 1},
	}

	// If Guest goes first, have Guest fire an initial miss at (0,9) so Host takes the offensive lead
	guestMissIdx := 0
	guestMissCoords := func() game.Coord {
		c := game.Coord{X: guestMissIdx % 10, Y: 7 + (guestMissIdx / 10)}
		guestMissIdx++
		return c
	}

	if currentTurnID == playerGuest.PlayerID {
		missCell := guestMissCoords()
		mustWriteJSON(t, ctx, connGuest, OutboundMessage{
			Type:    TypeBattleFire,
			Payload: BattleFirePayload{Cell: missCell},
		})
		_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
		_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)
	}

	sunkCount := 0
	var hostGameOver, guestGameOver GameOverPayload

	for i, targetCell := range guestShipCells {
		mustWriteJSON(t, ctx, connHost, OutboundMessage{
			Type:    TypeBattleFire,
			Payload: BattleFirePayload{Cell: targetCell},
		})

		_, pShotH := drainUntilType(t, ctx, connHost, TypeBattleShotResult)
		var shotRes BattleShotResultPayload
		if err := json.Unmarshal(pShotH, &shotRes); err != nil {
			t.Fatalf("unmarshal shotResult at step %d: %v", i, err)
		}
		if shotRes.By != playerHost.PlayerID {
			t.Fatalf("expected shot by host %s, got %s", playerHost.PlayerID, shotRes.By)
		}
		if shotRes.Result == game.ShotResultSunk {
			sunkCount++
			if shotRes.SunkShip == nil || len(shotRes.SunkShip.Cells) == 0 {
				t.Fatalf("expected sunkShip details on sunk shot at step %d", i)
			}
		} else if shotRes.Result != game.ShotResultHit {
			t.Fatalf("expected hit or sunk at %v, got %s", targetCell, shotRes.Result)
		}

		isFinalShot := i == len(guestShipCells)-1
		if isFinalShot {
			_, pOverH := drainUntilType(t, ctx, connHost, TypeGameOver)
			_, pOverG := drainUntilType(t, ctx, connGuest, TypeGameOver)
			_ = json.Unmarshal(pOverH, &hostGameOver)
			_ = json.Unmarshal(pOverG, &guestGameOver)
			_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
			_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)
		} else {
			_, pStateH := drainUntilType(t, ctx, connHost, TypeRoomState)
			_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

			// Verify zero coordinate leakage of unrevealed opponent ships in raw JSON
			var rawView map[string]any
			_ = json.Unmarshal(pStateH, &rawView)
			if bv, ok := rawView["battleView"].(map[string]any); ok {
				if tb, ok := bv["targetBoard"].(map[string]any); ok {
					if _, leaked := tb["ships"]; leaked {
						t.Fatal("SECURITY LEAK: targetBoard leaked opponent ships array during battle")
					}
				}
			}

			// Guest fires a miss on their turn
			missCell := guestMissCoords()
			mustWriteJSON(t, ctx, connGuest, OutboundMessage{
				Type:    TypeBattleFire,
				Payload: BattleFirePayload{Cell: missCell},
			})
			_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
			_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)
		}
	}

	if sunkCount != 5 {
		t.Errorf("expected 5 sunk ships across full game, got %d", sunkCount)
	}
	if hostGameOver.WinnerPlayerID != playerHost.PlayerID || hostGameOver.Reason != "all_sunk" {
		t.Errorf("unexpected host game.over payload: %+v", hostGameOver)
	}
	if guestGameOver.WinnerPlayerID != playerHost.PlayerID || guestGameOver.Reason != "all_sunk" {
		t.Errorf("unexpected guest game.over payload: %+v", guestGameOver)
	}
	expectedTotalShots := len(guestShipCells) + guestMissIdx
	if hostGameOver.TotalShots != expectedTotalShots {
		t.Errorf("expected TotalShots %d, got %d", expectedTotalShots, hostGameOver.TotalShots)
	}

	// Verify SQLite persistence wrote the finished match record
	gameID := mgr.GetRoomGameIDForTesting(roomID)
	if gameID == "" {
		t.Fatal("expected non-empty gameID on finished room")
	}

	var savedRecord *store.GameRecord
	for attempt := 0; attempt < 25; attempt++ {
		rec, getErr := st.GetGame(ctx, gameID)
		if getErr == nil && rec != nil {
			savedRecord = rec
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if savedRecord == nil {
		t.Fatalf("expected finished game %s to be persisted in SQLite store", gameID)
	}
	if savedRecord.EndReason != "all_sunk" {
		t.Errorf("expected persisted EndReason 'all_sunk', got %q", savedRecord.EndReason)
	}
	if savedRecord.Player1Name != "AdmiralNimitz" || savedRecord.Player2Name != "CaptainHalsey" {
		t.Errorf("unexpected persisted player names: %q vs %q", savedRecord.Player1Name, savedRecord.Player2Name)
	}
	if savedRecord.WinnerSlot == nil || *savedRecord.WinnerSlot != 1 {
		t.Errorf("expected WinnerSlot 1 (host), got %v", savedRecord.WinnerSlot)
	}
	if savedRecord.TotalShots != expectedTotalShots {
		t.Errorf("expected persisted TotalShots %d, got %d", expectedTotalShots, savedRecord.TotalShots)
	}
}

func TestWebSocket_TwoClientBattleReconnectAndResume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, playerHost, connGuest, playerGuest, roomID, currentTurnID := advanceTwoClientsToBattle(
		t, ctx, server.URL, "HostReconnect", "GuestReconnect",
	)
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	// Fire 2 alternating shots so shotLog and board state are populated
	firstConn := connHost
	secondConn := connGuest
	if currentTurnID == playerGuest.PlayerID {
		firstConn = connGuest
		secondConn = connHost
	}

	mustWriteJSON(t, ctx, firstConn, OutboundMessage{
		Type:    TypeBattleFire,
		Payload: BattleFirePayload{Cell: game.Coord{X: 0, Y: 9}},
	})
	_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

	mustWriteJSON(t, ctx, secondConn, OutboundMessage{
		Type:    TypeBattleFire,
		Payload: BattleFirePayload{Cell: game.Coord{X: 1, Y: 9}},
	})
	_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

	// Guest drops WebSocket connection mid-battle
	_ = connGuest.Close(websocket.StatusNormalClosure, "simulated network drop")

	// Host receives opponent.disconnected with graceDeadline
	_, pDisc := drainUntilType(t, ctx, connHost, TypeOpponentDisconnected)
	var discPayload OpponentDisconnectedPayload
	if err := json.Unmarshal(pDisc, &discPayload); err != nil {
		t.Fatalf("unmarshal opponent.disconnected: %v", err)
	}
	if discPayload.GraceDeadline.IsZero() {
		t.Error("expected non-zero graceDeadline on opponent.disconnected")
	}

	// Guest reconnects within 60s grace period using sessionToken
	connGuestReconnected, helloOKGuest, _ := connectAndHandshake(
		t, ctx, server.URL, "GuestReconnect", playerGuest.SessionToken,
	)
	defer func() { _ = connGuestReconnected.Close(websocket.StatusNormalClosure, "done") }()

	if helloOKGuest.RestoredState == nil {
		t.Fatal("expected non-nil restoredState in hello.ok upon battle reconnect")
	}

	restoredBytes, _ := json.Marshal(helloOKGuest.RestoredState)
	var restoredView lobby.RoomStateView
	if err := json.Unmarshal(restoredBytes, &restoredView); err != nil {
		t.Fatalf("unmarshal restoredState: %v", err)
	}
	if restoredView.RoomID != roomID || restoredView.Phase != lobby.PhaseBattle {
		t.Fatalf("expected restored room %s in phase battle, got room=%s phase=%s", roomID, restoredView.RoomID, restoredView.Phase)
	}
	if len(restoredView.ShotLog) != 2 {
		t.Errorf("expected 2 shots in restored ShotLog, got %d", len(restoredView.ShotLog))
	}
	if restoredView.BattleView == nil {
		t.Fatal("expected BattleView in restoredState")
	}

	// Host receives opponent.reconnected
	_, _ = drainUntilType(t, ctx, connHost, TypeOpponentReconnected)
	// Guest also receives room.state after hello.ok
	_, _ = drainUntilType(t, ctx, connGuestReconnected, TypeRoomState)

	// Verify battle resumes cleanly: active player fires another shot
	activeConn := connHost
	if restoredView.TurnPlayerID == playerGuest.PlayerID {
		activeConn = connGuestReconnected
	}
	mustWriteJSON(t, ctx, activeConn, OutboundMessage{
		Type:    TypeBattleFire,
		Payload: BattleFirePayload{Cell: game.Coord{X: 2, Y: 9}},
	})

	_, pShotHost := drainUntilType(t, ctx, connHost, TypeBattleShotResult)
	_, pShotGuest := drainUntilType(t, ctx, connGuestReconnected, TypeBattleShotResult)
	var shotHost, shotGuest BattleShotResultPayload
	_ = json.Unmarshal(pShotHost, &shotHost)
	_ = json.Unmarshal(pShotGuest, &shotGuest)

	if shotHost.Cell != (game.Coord{X: 2, Y: 9}) || shotGuest.Cell != (game.Coord{X: 2, Y: 9}) {
		t.Errorf("expected resumed shot at (2,9), got host=%v guest=%v", shotHost.Cell, shotGuest.Cell)
	}
	_ = playerHost
}

func TestWebSocket_TwoClientFullGameAndRematch(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, playerHost, connGuest, playerGuest, _, currentTurnID := advanceTwoClientsToBattle(
		t, ctx, server.URL, "HostRematchFull", "GuestRematchFull",
	)
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	guestShipCells := []game.Coord{
		{X: 5, Y: 0}, {X: 5, Y: 1}, {X: 5, Y: 2}, {X: 5, Y: 3}, {X: 5, Y: 4},
		{X: 6, Y: 0}, {X: 6, Y: 1}, {X: 6, Y: 2}, {X: 6, Y: 3},
		{X: 7, Y: 0}, {X: 7, Y: 1}, {X: 7, Y: 2},
		{X: 8, Y: 0}, {X: 8, Y: 1}, {X: 8, Y: 2},
		{X: 9, Y: 0}, {X: 9, Y: 1},
	}

	missIdx := 0
	if currentTurnID == playerGuest.PlayerID {
		mustWriteJSON(t, ctx, connGuest, OutboundMessage{
			Type:    TypeBattleFire,
			Payload: BattleFirePayload{Cell: game.Coord{X: missIdx % 10, Y: 7 + (missIdx / 10)}},
		})
		missIdx++
		_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
		_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)
	}

	for i, target := range guestShipCells {
		mustWriteJSON(t, ctx, connHost, OutboundMessage{
			Type:    TypeBattleFire,
			Payload: BattleFirePayload{Cell: target},
		})
		if i == len(guestShipCells)-1 {
			_, _ = drainUntilType(t, ctx, connHost, TypeGameOver)
			_, _ = drainUntilType(t, ctx, connGuest, TypeGameOver)
			_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
			_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)
		} else {
			_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
			_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

			mustWriteJSON(t, ctx, connGuest, OutboundMessage{
				Type:    TypeBattleFire,
				Payload: BattleFirePayload{Cell: game.Coord{X: missIdx % 10, Y: 7 + (missIdx / 10)}},
			})
			missIdx++
			_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
			_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)
		}
	}

	// Game 1 finished via all_sunk. Now both players accept a rematch over WebSockets.
	mustWriteJSON(t, ctx, connHost, OutboundMessage{Type: TypeRematchAccept, Payload: map[string]any{}})
	_, pOffer := drainUntilType(t, ctx, connGuest, TypeRematchOffered)
	var offered RematchOfferedPayload
	_ = json.Unmarshal(pOffer, &offered)
	if offered.By != playerHost.PlayerID {
		t.Fatalf("expected rematch.offered by host %s, got %s", playerHost.PlayerID, offered.By)
	}

	mustWriteJSON(t, ctx, connGuest, OutboundMessage{Type: TypeRematchAccept, Payload: map[string]any{}})

	// Both transition directly to placement for Game 2
	_, pHostRematchState := drainUntilType(t, ctx, connHost, TypeRoomState)
	_, pGuestRematchState := drainUntilType(t, ctx, connGuest, TypeRoomState)

	var hostPlacementView, guestPlacementView lobby.RoomStateView
	_ = json.Unmarshal(pHostRematchState, &hostPlacementView)
	_ = json.Unmarshal(pGuestRematchState, &guestPlacementView)

	if hostPlacementView.Phase != lobby.PhasePlacement || guestPlacementView.Phase != lobby.PhasePlacement {
		t.Fatalf("expected both players in placement phase after mutual rematch, got host=%s guest=%s",
			hostPlacementView.Phase, guestPlacementView.Phase)
	}
	if len(hostPlacementView.ShotLog) != 0 {
		t.Errorf("expected ShotLog reset on rematch, got %d entries", len(hostPlacementView.ShotLog))
	}

	// Both submit fleets in Game 2 and verify battle.start fires
	mustWriteJSON(t, ctx, connHost, OutboundMessage{
		Type:    TypePlacementSubmit,
		Payload: PlacementSubmitPayload{Ships: testFleetForWS()},
	})
	mustWriteJSON(t, ctx, connGuest, OutboundMessage{
		Type:    TypePlacementSubmit,
		Payload: PlacementSubmitPayload{Ships: testGuestFleetForWS()},
	})

	_, _ = drainUntilType(t, ctx, connHost, TypeBattleStart)
	_, _ = drainUntilType(t, ctx, connGuest, TypeBattleStart)
}

func TestWebSocket_DraftPlacementPreservedOnTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clock := lobby.NewMockClock(time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC))
	server, _, _ := setupTestServer(t, lobby.WithClock(clock))

	connHost, _, _ := connectAndHandshake(t, ctx, server.URL, "DraftHost", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	connGuest, _, _ := connectAndHandshake(t, ctx, server.URL, "DraftGuest", "")
	defer func() { _ = connGuest.Close(websocket.StatusNormalClosure, "done") }()

	mustWriteJSON(t, ctx, connHost, OutboundMessage{Type: TypeLobbyCreate, Payload: map[string]any{}})
	_, pCreate := drainUntilType(t, ctx, connHost, TypeRoomState)
	var hostRoom lobby.RoomStateView
	_ = json.Unmarshal(pCreate, &hostRoom)

	mustWriteJSON(t, ctx, connGuest, OutboundMessage{
		Type:    TypeLobbyJoin,
		Payload: LobbyJoinPayload{RoomID: hostRoom.RoomID},
	})
	_, _ = drainUntilType(t, ctx, connHost, TypeRoomReadyState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomReadyState)

	mustWriteJSON(t, ctx, connHost, OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})
	mustWriteJSON(t, ctx, connGuest, OutboundMessage{Type: TypeRoomReady, Payload: map[string]any{}})

	_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)
	_, _ = drainUntilType(t, ctx, connGuest, TypeRoomState)

	// Host places 2 ships as a draft (unconfirmed)
	draftShips := []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 2, Y: 2}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 7, Y: 8}, Orientation: game.OrientationVertical},
	}
	mustWriteJSON(t, ctx, connHost, OutboundMessage{
		Type: TypePlacementSubmit,
		Payload: PlacementSubmitPayload{
			Ships: draftShips,
			Draft: true,
		},
	})

	// Allow the draft message to be processed before advancing the clock
	time.Sleep(50 * time.Millisecond)

	// Advance clock by 60s to trigger placement timeout auto-completion
	clock.Advance(60 * time.Second)

	_, pConf := drainUntilType(t, ctx, connHost, TypePlacementConfirmed)
	var confPayload PlacementConfirmedPayload
	if err := json.Unmarshal(pConf, &confPayload); err != nil {
		t.Fatalf("unmarshal placement.confirmed: %v", err)
	}
	if !confPayload.Confirmed || !confPayload.Auto || len(confPayload.Ships) != 5 {
		t.Fatalf("expected auto-confirmed 5-ship fleet, got %+v", confPayload)
	}

	// Verify the 2 draft ships were preserved untouched
	sawCarrier := false
	sawDestroyer := false
	for _, s := range confPayload.Ships {
		if s.Type == game.ShipCarrier && s.Origin == (game.Coord{X: 2, Y: 2}) && s.Orientation == game.OrientationHorizontal {
			sawCarrier = true
		}
		if s.Type == game.ShipDestroyer && s.Origin == (game.Coord{X: 7, Y: 8}) && s.Orientation == game.OrientationVertical {
			sawDestroyer = true
		}
	}
	if !sawCarrier || !sawDestroyer {
		t.Errorf("expected draft ships to be preserved on 60s placement timeout, got fleet: %+v", confPayload.Ships)
	}
}

func TestHandshake_ReconnectInActiveRoomPreservesDisplayName(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})

	connHost, playerHost, _ := connectAndHandshake(t, ctx, server.URL, "OriginalHost", "")
	defer func() { _ = connHost.Close(websocket.StatusNormalClosure, "done") }()

	// Create room so host has an active CurrentRoomID
	mustWriteJSON(t, ctx, connHost, OutboundMessage{Type: TypeLobbyCreate, Payload: map[string]any{}})
	_, _ = drainUntilType(t, ctx, connHost, TypeRoomState)

	// Simulate drop and reconnect with a different display name (e.g. spoofing attempt)
	_ = connHost.Close(websocket.StatusNormalClosure, "drop")
	time.Sleep(50 * time.Millisecond)

	connReconnected, helloOK, _ := connectAndHandshake(t, ctx, server.URL, "SpoofedName", playerHost.SessionToken)
	defer func() { _ = connReconnected.Close(websocket.StatusNormalClosure, "done") }()

	if helloOK.DisplayName != "OriginalHost" {
		t.Errorf("expected display name to remain 'OriginalHost' when reconnecting to active room, got %q", helloOK.DisplayName)
	}
}

func TestWebSocket_SpoofedForwardedHeaderCannotBypassIPLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithIPLimit(1),
		WithTrustProxy(false),
	})

	conn1, _, _ := connectAndHandshake(t, ctx, server.URL, "LegitUser", "")
	defer func() { _ = conn1.Close(websocket.StatusNormalClosure, "done") }()

	// Attempt second connection from same RemoteAddr with spoofed X-Forwarded-For
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	_, resp, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"X-Forwarded-For": []string{"198.51.100.99"},
			"X-Real-IP":       []string{"198.51.100.99"},
		},
	})
	if err == nil {
		t.Fatal("expected second connection from same IP to be rejected despite spoofed forwarded headers")
	}
	if resp != nil && resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected HTTP 429 Too Many Requests, got %d", resp.StatusCode)
	}
}

func TestWebSocket_HandshakeRateLimited(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{
		WithIPLimit(0),
		WithHandshakeRateLimit(0.5, 1.0),
	})

	conn1, _, _ := connectAndHandshake(t, ctx, server.URL, "Fast1", "")
	_ = conn1.Close(websocket.StatusNormalClosure, "done")

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	_, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected rapid second upgrade to be rate limited")
	}
	if resp != nil && resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected HTTP 429 Too Many Requests, got %d", resp.StatusCode)
	}
}

func TestWSSender_SlowConsumerOutboxOverflowClosesWithoutBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _, _, _ := setupTestServerWithHandlerOpts(t, []HandlerOption{WithRateLimit(100, 100)})
	conn := dialWS(t, ctx, server.URL)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "done") }()

	// Unit-test wsSender directly against an unread connection: fill outbox > defaultOutboxSize
	// without starting read loop on the peer, verifying Send returns promptly.
	sender := &wsSender{
		conn:   conn,
		ctx:    ctx,
		async:  true,
		outbox: make(chan outboundFrame, 2),
	}

	if err := sender.Send("test.1", map[string]any{"n": 1}); err != nil {
		t.Fatalf("unexpected error on slot 1: %v", err)
	}
	if err := sender.Send("test.2", map[string]any{"n": 2}); err != nil {
		t.Fatalf("unexpected error on slot 2: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- sender.Send("test.3", map[string]any{"n": 3})
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected slow consumer overflow error when outbox full")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Send blocked on full outbox instead of dropping slow consumer immediately")
	}
}
