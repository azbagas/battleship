package ws

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/azbagas/battleship/backend/internal/lobby"
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

	server, _, _ := setupTestServer(t)
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

	server, _, _ := setupTestServer(t)

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

	server, _, _ := setupTestServer(t)
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

			server, _, _ := setupTestServer(t)
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

	server, _, _ := setupTestServer(t)
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

	server, _, _ := setupTestServer(t)
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

	server, _, _ := setupTestServer(t)

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

	server, _, _ := setupTestServer(t)

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

	server, _, _ := setupTestServer(t)

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

	go func() {
		defer wg.Done()
		<-barrier
		_ = connJ1.Write(ctx, websocket.MessageText, joinPayload)
		mType, p := readEnvelope(t, ctx, connJ1)
		j1Result = mType
		if mType == TypeError {
			var ep ErrorPayload
			_ = json.Unmarshal(p, &ep)
			j1ErrCode = ep.Code
		}
	}()

	go func() {
		defer wg.Done()
		<-barrier
		_ = connJ2.Write(ctx, websocket.MessageText, joinPayload)
		mType, p := readEnvelope(t, ctx, connJ2)
		j2Result = mType
		if mType == TypeError {
			var ep ErrorPayload
			_ = json.Unmarshal(p, &ep)
			j2ErrCode = ep.Code
		}
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
