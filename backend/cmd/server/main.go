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
	"github.com/azbagas/battleship/backend/internal/ws"
)

// HealthResponse represents the payload returned by the health check endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// routes sets up and returns the HTTP mux for the server.
func routes(registry *lobby.SessionRegistry, lobbyMgr *lobby.LobbyManager) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthzHandler)

	wsHandler := ws.NewHandler(registry, lobbyMgr)
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
	sessionRegistry := lobby.NewSessionRegistry()
	lobbyManager := lobby.NewLobbyManager(sessionRegistry)
	srv := &http.Server{
		Addr:         addr,
		Handler:      routes(sessionRegistry, lobbyManager),
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

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	} else {
		slog.Info("server exited cleanly")
	}
}

