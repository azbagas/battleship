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

func TestLoadConfig_DefaultsAndOverrides(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("DB_PATH", "")
	t.Setenv("SQLITE_PATH", "")
	t.Setenv("ALLOWED_ORIGINS", "")
	t.Setenv("MAX_PAYLOAD_BYTES", "")
	t.Setenv("MAX_CONNS_PER_IP", "")
	t.Setenv("RATE_LIMIT_MESSAGES_PER_SEC", "")
	t.Setenv("RATE_LIMIT_BURST", "")
	t.Setenv("TRUST_PROXY", "")

	def := loadConfig()
	if def.Port != "8080" || def.DBPath != "battleship.db" || def.RateLimit != 10.0 || def.RateBurst != 20.0 || def.TrustProxy {
		t.Fatalf("unexpected defaults: %+v", def)
	}

	t.Setenv("SQLITE_PATH", "fallback.db")
	if cfg := loadConfig(); cfg.DBPath != "fallback.db" {
		t.Fatalf("expected SQLITE_PATH fallback, got %q", cfg.DBPath)
	}

	t.Setenv("PORT", "9090")
	t.Setenv("DB_PATH", "custom.db")
	t.Setenv("ALLOWED_ORIGINS", " https://a.example , , https://b.example ")
	t.Setenv("MAX_PAYLOAD_BYTES", "16384")
	t.Setenv("MAX_CONNS_PER_IP", "25")
	t.Setenv("RATE_LIMIT_MESSAGES_PER_SEC", "15")
	t.Setenv("RATE_LIMIT_BURST", "30")
	t.Setenv("TRUST_PROXY", "true")

	ov := loadConfig()
	if ov.Port != "9090" || ov.DBPath != "custom.db" || ov.MaxPayloadBytes != 16384 || ov.MaxConnsPerIP != 25 || ov.RateLimit != 15 || ov.RateBurst != 30 || !ov.TrustProxy {
		t.Fatalf("unexpected overrides: %+v", ov)
	}
	if len(ov.AllowedOrigins) != 2 || ov.AllowedOrigins[0] != "https://a.example" || ov.AllowedOrigins[1] != "https://b.example" {
		t.Fatalf("unexpected AllowedOrigins: %+v", ov.AllowedOrigins)
	}

	opts := buildWSOptions(ov)
	if len(opts) != 5 {
		t.Fatalf("expected 5 ws options when all overrides active, got %d", len(opts))
	}
}

