package ws

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/azbagas/battleship/backend/internal/lobby"
)

func setupTestServer(t *testing.T) (*httptest.Server, *lobby.SessionRegistry) {
	t.Helper()
	reg := lobby.NewSessionRegistry()
	handler := NewHandler(reg)
	server := httptest.NewServer(handler)
	t.Cleanup(func() {
		server.Close()
	})
	return server, reg
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

func TestHandshake_NewSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _ := setupTestServer(t)
	conn := dialWS(t, ctx, server.URL)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}()

	helloMsg := OutboundMessage{
		Type: TypeHello,
		Payload: HelloPayload{
			Name: "  Captain Kirk  ",
		},
	}
	data, _ := json.Marshal(helloMsg)
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("failed to send hello: %v", err)
	}

	msgType, payloadBytes := readEnvelope(t, ctx, conn)
	if msgType != TypeHelloOK {
		t.Fatalf("expected %s, got %s (payload: %s)", TypeHelloOK, msgType, string(payloadBytes))
	}

	var helloOK HelloOKPayload
	if err := json.Unmarshal(payloadBytes, &helloOK); err != nil {
		t.Fatalf("failed to unmarshal hello.ok: %v", err)
	}

	if helloOK.DisplayName != "Captain Kirk" {
		t.Errorf("expected trimmed display name 'Captain Kirk', got %q", helloOK.DisplayName)
	}
	if !strings.HasPrefix(helloOK.PlayerID, "p_") {
		t.Errorf("expected player ID with prefix 'p_', got %q", helloOK.PlayerID)
	}
	if len(helloOK.SessionToken) == 0 {
		t.Errorf("expected non-empty session token")
	}
}

func TestHandshake_ReconnectRestoresSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _ := setupTestServer(t)

	// 1. Initial connection
	conn1 := dialWS(t, ctx, server.URL)
	helloMsg := OutboundMessage{
		Type: TypeHello,
		Payload: HelloPayload{
			Name: "Spock",
		},
	}
	data, _ := json.Marshal(helloMsg)
	_ = conn1.Write(ctx, websocket.MessageText, data)

	msgType, payloadBytes := readEnvelope(t, ctx, conn1)
	if msgType != TypeHelloOK {
		t.Fatalf("expected %s, got %s", TypeHelloOK, msgType)
	}
	var ok1 HelloOKPayload
	_ = json.Unmarshal(payloadBytes, &ok1)

	// Disconnect client 1
	_ = conn1.Close(websocket.StatusNormalClosure, "closing client 1")

	// Wait briefly for server disconnect hook to run
	time.Sleep(50 * time.Millisecond)

	// 2. Reconnect with the same session token
	conn2 := dialWS(t, ctx, server.URL)
	defer func() {
		_ = conn2.Close(websocket.StatusNormalClosure, "done")
	}()

	reconnectMsg := OutboundMessage{
		Type: TypeHello,
		Payload: HelloPayload{
			Name:         "Spock Updated",
			SessionToken: ok1.SessionToken,
		},
	}
	reconnectData, _ := json.Marshal(reconnectMsg)
	if err := conn2.Write(ctx, websocket.MessageText, reconnectData); err != nil {
		t.Fatalf("failed to write reconnect message: %v", err)
	}

	msgType2, payloadBytes2 := readEnvelope(t, ctx, conn2)
	if msgType2 != TypeHelloOK {
		t.Fatalf("expected %s on reconnect, got %s (payload: %s)", TypeHelloOK, msgType2, string(payloadBytes2))
	}
	var ok2 HelloOKPayload
	_ = json.Unmarshal(payloadBytes2, &ok2)

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

	server, reg := setupTestServer(t)

	// 1. First connection
	conn1 := dialWS(t, ctx, server.URL)
	defer func() {
		_ = conn1.Close(websocket.StatusNormalClosure, "closing 1")
	}()

	helloMsg := OutboundMessage{
		Type: TypeHello,
		Payload: HelloPayload{
			Name: "McCoy",
		},
	}
	data, _ := json.Marshal(helloMsg)
	_ = conn1.Write(ctx, websocket.MessageText, data)

	msgType, payloadBytes := readEnvelope(t, ctx, conn1)
	if msgType != TypeHelloOK {
		t.Fatalf("expected %s, got %s", TypeHelloOK, msgType)
	}
	var ok1 HelloOKPayload
	_ = json.Unmarshal(payloadBytes, &ok1)

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

	// Verify conn1 remains alive and connected in registry and socket is writable
	player, ok := reg.GetByToken(ok1.SessionToken)
	if !ok || player.ConnectionState != lobby.StateConnected {
		t.Errorf("expected player1 to remain connected in registry")
	}

	writeErr := conn1.Write(ctx, websocket.MessageText, []byte(`{"type":"noop","payload":{}}`))
	if writeErr != nil {
		t.Errorf("expected conn1 to remain writable, got: %v", writeErr)
	}
}

func TestHandshake_UnknownTokenCreatesNewSession(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server, _ := setupTestServer(t)
	conn := dialWS(t, ctx, server.URL)
	defer func() {
		_ = conn.Close(websocket.StatusNormalClosure, "done")
	}()

	unknownToken := "invalid-or-restarted-server-token"
	helloMsg := OutboundMessage{
		Type: TypeHello,
		Payload: HelloPayload{
			Name:         "Scotty",
			SessionToken: unknownToken,
		},
	}
	data, _ := json.Marshal(helloMsg)
	_ = conn.Write(ctx, websocket.MessageText, data)

	msgType, payloadBytes := readEnvelope(t, ctx, conn)
	if msgType != TypeHelloOK {
		t.Fatalf("expected %s for unknown token, got %s", TypeHelloOK, msgType)
	}

	var helloOK HelloOKPayload
	_ = json.Unmarshal(payloadBytes, &helloOK)

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

			server, _ := setupTestServer(t)
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

	server, _ := setupTestServer(t)
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

	server, _ := setupTestServer(t)
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
