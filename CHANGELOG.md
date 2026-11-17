# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.1] - 2026-11-17

### Changed
- **Docker & Reverse Proxy:** Load optional `backend/.env` via `env_file` in `docker-compose.yml`, add `restart: unless-stopped` to both services, bind `frontend` to `127.0.0.1:3000:80`, and preserve client IP headers (`X-Forwarded-For $proxy_add_x_forwarded_for`) in `frontend/nginx.conf`.

## [1.0.0] - 2026-11-17

### Added
- **Session Identity & Reconnection (`F-01`, `F-06`):** Frictionless display-name entry (1–20 chars), cryptographically random session tokens persisted in `localStorage`, single-active-tab enforcement (`SESSION_ALREADY_ACTIVE`), automatic exponential-backoff reconnection, and a 60-second disconnect grace window with live opponent countdown.
- **Live Public Lobby & Ready-Up (`F-02`, `F-03`):** Real-time public waiting room list (oldest-first, 10-room global cap), atomic room join, host cancel, and pre-game mutual ready-up confirmation.
- **Fleet Placement (`F-04`):** Timed 60-second setup phase on a 10×10 grid for the classic 5-ship fleet (Carrier 5, Battleship 4, Cruiser 3, Submarine 3, Destroyer 2) with desktop drag-and-drop, touch tap-to-place, keyboard shortcuts, ship rotation, **Randomize**, draft sync, and automatic placement of unplaced ships on timeout.
- **Turn-Based Battle (`F-05`):** Random starting player, strict alternating single-shot turns, 30-second turn timer with automatic random fallback shots (`auto: true`), hit/miss/sunk resolution, recent combat log, and SVG top-down ship blueprints.
- **Game Over, Rematch & Match Persistence (`F-07`):** Post-match debrief modal/screen with end reasons (`all_sunk`, `forfeit`, `disconnect_timeout`, `abandoned`), mutual rematch flow returning directly to placement, cancellation notifications when an opponent leaves, and SQLite match recording via pure-Go `modernc.org/sqlite` (WAL mode).
- **Responsive UX, Accessibility & Fair Play (`F-08`):** Mobile-first layout down to 360px with target/defense view toggle, color-blind-safe hit/miss/sunk shapes and patterns, keyboard grid navigation, and strict server-authoritative state hiding opponent coordinates until sunk or game end.
- **Operations, Security & Tooling:** Go `net/http` + `coder/websocket` backend (`/healthz`, `/ws`), structured `slog` logging, `TRUST_PROXY`-gated IP extraction, per-IP handshake/connection limits, per-connection message rate limiting, bounded async WebSocket outbox with slow-consumer disconnect, `crypto/rand` RNG, Docker Compose + Nginx deployment with security headers, and Vitest / Go `-race` / Playwright E2E test suites.
