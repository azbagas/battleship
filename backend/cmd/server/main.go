package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/azbagas/battleship/backend/internal/lobby"
	"github.com/azbagas/battleship/backend/internal/store"
	"github.com/azbagas/battleship/backend/internal/ws"
)

// HealthResponse represents the payload returned by the health check endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// routes sets up and returns the HTTP mux for the server.
func routes(registry *lobby.SessionRegistry, lobbyMgr *lobby.LobbyManager, wsHandler *ws.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)

	if wsHandler == nil {
		wsHandler = ws.NewHandler(registry, lobbyMgr)
	}
	mux.Handle("/ws", wsHandler)

	return mux
}

// healthzHandler handles GET requests for container and service health checks.
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(HealthResponse{Status: "ok"}); err != nil {
		slog.Error("failed to encode health check response", "error", err)
	}
}

// initLogger configures the default structured logger using environment variables.
func initLogger() {
	level := slog.LevelInfo
	if lvlStr := os.Getenv("LOG_LEVEL"); lvlStr != "" {
		switch strings.ToUpper(lvlStr) {
		case "DEBUG":
			level = slog.LevelDebug
		case "INFO":
			level = slog.LevelInfo
		case "WARN":
			level = slog.LevelWarn
		case "ERROR":
			level = slog.LevelError
		}
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if strings.ToLower(os.Getenv("LOG_FORMAT")) == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func main() {
	initLogger()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf(":%s", port)
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = os.Getenv("SQLITE_PATH")
	}
	if dbPath == "" {
		dbPath = "battleship.db"
	}

	gameStore, err := store.New(dbPath)
	if err != nil {
		slog.Error("failed to initialize sqlite store", "dbPath", dbPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := gameStore.Close(); err != nil {
			slog.Error("error closing sqlite store", "error", err)
		}
	}()

	sessionRegistry := lobby.NewSessionRegistry()
	lobbyManager := lobby.NewLobbyManager(sessionRegistry, lobby.WithStore(gameStore))

	var wsOpts []ws.HandlerOption
	if allowedOriginsStr := os.Getenv("ALLOWED_ORIGINS"); allowedOriginsStr != "" {
		parts := strings.Split(allowedOriginsStr, ",")
		var origins []string
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
		if len(origins) > 0 {
			wsOpts = append(wsOpts, ws.WithAllowedOrigins(origins))
		}
	}

	if maxPayloadStr := os.Getenv("MAX_PAYLOAD_BYTES"); maxPayloadStr != "" {
		if bytes, err := strconv.ParseInt(maxPayloadStr, 10, 64); err == nil && bytes > 0 {
			wsOpts = append(wsOpts, ws.WithMaxPayloadBytes(bytes))
		}
	}

	if maxConnsIPStr := os.Getenv("MAX_CONNS_PER_IP"); maxConnsIPStr != "" {
		if maxConns, err := strconv.Atoi(maxConnsIPStr); err == nil && maxConns > 0 {
			wsOpts = append(wsOpts, ws.WithIPLimit(maxConns))
		}
	}

	rateLimit := 10.0
	rateBurst := 20.0
	if rateStr := os.Getenv("RATE_LIMIT_MESSAGES_PER_SEC"); rateStr != "" {
		if r, err := strconv.ParseFloat(rateStr, 64); err == nil && r > 0 {
			rateLimit = r
		}
	}
	if burstStr := os.Getenv("RATE_LIMIT_BURST"); burstStr != "" {
		if b, err := strconv.ParseFloat(burstStr, 64); err == nil && b > 0 {
			rateBurst = b
		}
	}
	wsOpts = append(wsOpts, ws.WithRateLimit(rateLimit, rateBurst))

	wsHandler := ws.NewHandler(sessionRegistry, lobbyManager, wsOpts...)

	srv := &http.Server{
		Addr:         addr,
		Handler:      routes(sessionRegistry, lobbyManager, wsHandler),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := wsHandler.Shutdown(shutdownCtx); err != nil {
		slog.Error("error shutting down websocket handler", "error", err)
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	} else {
		slog.Info("server exited cleanly")
	}
}

