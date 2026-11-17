package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/azbagas/battleship/backend/internal/ws"
)

// serverConfig holds runtime configuration parsed from environment variables.
type serverConfig struct {
	Port            string
	DBPath          string
	AllowedOrigins  []string
	MaxPayloadBytes int64
	MaxConnsPerIP   int
	RateLimit       float64
	RateBurst       float64
	TrustProxy      bool
}

// loadConfig reads server configuration from environment variables with safe defaults.
func loadConfig() serverConfig {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = os.Getenv("SQLITE_PATH")
	}
	if dbPath == "" {
		dbPath = "battleship.db"
	}

	cfg := serverConfig{
		Port:      port,
		DBPath:    dbPath,
		RateLimit: 10.0,
		RateBurst: 20.0,
	}

	if allowedOriginsStr := os.Getenv("ALLOWED_ORIGINS"); allowedOriginsStr != "" {
		for _, p := range strings.Split(allowedOriginsStr, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				cfg.AllowedOrigins = append(cfg.AllowedOrigins, trimmed)
			}
		}
	}

	if maxPayloadStr := os.Getenv("MAX_PAYLOAD_BYTES"); maxPayloadStr != "" {
		if bytes, err := strconv.ParseInt(maxPayloadStr, 10, 64); err == nil && bytes > 0 {
			cfg.MaxPayloadBytes = bytes
		}
	}

	if maxConnsIPStr := os.Getenv("MAX_CONNS_PER_IP"); maxConnsIPStr != "" {
		if maxConns, err := strconv.Atoi(maxConnsIPStr); err == nil && maxConns > 0 {
			cfg.MaxConnsPerIP = maxConns
		}
	}

	if rateStr := os.Getenv("RATE_LIMIT_MESSAGES_PER_SEC"); rateStr != "" {
		if r, err := strconv.ParseFloat(rateStr, 64); err == nil && r > 0 {
			cfg.RateLimit = r
		}
	}

	if burstStr := os.Getenv("RATE_LIMIT_BURST"); burstStr != "" {
		if b, err := strconv.ParseFloat(burstStr, 64); err == nil && b > 0 {
			cfg.RateBurst = b
		}
	}

	if trustProxyStr := strings.TrimSpace(os.Getenv("TRUST_PROXY")); strings.EqualFold(trustProxyStr, "true") || trustProxyStr == "1" {
		cfg.TrustProxy = true
	}

	return cfg
}

// buildWSOptions converts serverConfig into ws.HandlerOption values.
func buildWSOptions(cfg serverConfig) []ws.HandlerOption {
	var opts []ws.HandlerOption
	if len(cfg.AllowedOrigins) > 0 {
		opts = append(opts, ws.WithAllowedOrigins(cfg.AllowedOrigins))
	}
	if cfg.MaxPayloadBytes > 0 {
		opts = append(opts, ws.WithMaxPayloadBytes(cfg.MaxPayloadBytes))
	}
	if cfg.MaxConnsPerIP > 0 {
		opts = append(opts, ws.WithIPLimit(cfg.MaxConnsPerIP))
	}
	opts = append(opts, ws.WithRateLimit(cfg.RateLimit, cfg.RateBurst))
	if cfg.TrustProxy {
		opts = append(opts, ws.WithTrustProxy(true))
	}
	return opts
}
