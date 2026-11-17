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
	cfg := loadConfig()

	addr := fmt.Sprintf(":%s", cfg.Port)
	gameStore, err := store.New(cfg.DBPath)
	if err != nil {
		slog.Error("failed to initialize sqlite store", "dbPath", cfg.DBPath, "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := gameStore.Close(); err != nil {
			slog.Error("error closing sqlite store", "error", err)
		}
	}()

	sessionRegistry := lobby.NewSessionRegistry()
	lobbyManager := lobby.NewLobbyManager(sessionRegistry, lobby.WithStore(gameStore))
	wsHandler := ws.NewHandler(sessionRegistry, lobbyManager, buildWSOptions(cfg)...)

	srv := &http.Server{
		Addr:              addr,
		Handler:           routes(sessionRegistry, lobbyManager, wsHandler),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
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

