package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/azbagas/battleship/backend/internal/lobby"
)

func TestHealthzEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	reg := lobby.NewSessionRegistry()
	mgr := lobby.NewLobbyManager(reg)
	handler := routes(reg, mgr, nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status code %d, got %d", http.StatusOK, rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}

	var resp HealthResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
}

func TestHealthzMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rr := httptest.NewRecorder()

	reg := lobby.NewSessionRegistry()
	mgr := lobby.NewLobbyManager(reg)
	handler := routes(reg, mgr, nil)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status code %d for POST, got %d", http.StatusMethodNotAllowed, rr.Code)
	}
}

func TestWSEndpoint_NonWebSocketRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	rr := httptest.NewRecorder()

	reg := lobby.NewSessionRegistry()
	mgr := lobby.NewLobbyManager(reg)
	handler := routes(reg, mgr, nil)
	handler.ServeHTTP(rr, req)

	// Since it's a plain GET request without WebSocket upgrade headers,
	// coder/websocket rejects it with 400 Bad Request or 426 Upgrade Required.
	if rr.Code != http.StatusBadRequest && rr.Code != http.StatusUpgradeRequired {
		t.Fatalf("expected status code 400 or 426 for non-WS GET /ws, got %d", rr.Code)
	}
}

func TestInitLogger(t *testing.T) {
	// Test default setup
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("LOG_FORMAT", "json")
	initLogger()

	t.Setenv("LOG_LEVEL", "WARN")
	t.Setenv("LOG_FORMAT", "text")
	initLogger()

	t.Setenv("LOG_LEVEL", "ERROR")
	initLogger()

	t.Setenv("LOG_LEVEL", "")
	initLogger()
}

