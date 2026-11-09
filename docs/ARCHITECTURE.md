# Architecture

<!-- The technical counterpart to PRD.md. This is where stack, structure, data, and design decisions live.
     Keep it current: describe the system as it IS, not its history. -->

## Overview
Online Battleship is built as a monorepo with a React + TypeScript single-page application (SPA) frontend and a single-process, server-authoritative Go backend. All live state (sessions, lobby rooms, boards, turns, and timers) lives in memory inside the Go server, while real-time lobby and game events flow over WebSockets. When a match finishes, the backend writes a single summary record to an embedded SQLite database. The system deploys on a single VPS via Docker Compose behind a reverse proxy that terminates TLS and serves the static frontend build.

## Tech Stack
| Layer | Choice | Why |
|-------|--------|-----|
| Language | TypeScript (Frontend), Go (Backend) | Type safety on the client; fast, simple concurrency and single-binary deployment on the server. |
| Framework | React + Vite (Frontend), Go `net/http` + WebSocket library (`coder/websocket` or `gorilla/websocket`) | Lightweight SPA setup driven by server messages (Zustand or React Context + reducer); minimal backend overhead with one goroutine per connection. |
| Database | SQLite via pure-Go driver (`modernc.org/sqlite`) | Zero-ops single-file persistence for finished game records without requiring CGO in Docker builds. |
| Testing | Go `testing` (`-race`, injectable clock), Vitest + React Testing Library, Playwright | Exhaustive unit, timer, concurrency, and two-browser end-to-end verification. |
| Hosting/Deploy | Docker Compose on a single VPS (Go backend container + Caddy/nginx reverse proxy) | Simple operations with TLS termination, static asset serving, and a mounted volume for the SQLite file. |

## Folder Structure
```
battleship/
├── frontend/            # React app (Vite + TypeScript)
├── backend/             # Go module
│   ├── cmd/server/      # main entrypoint, HTTP/WS server, config, graceful shutdown
│   ├── internal/game/   # pure game logic (board, ships, shots, win check, per-player views)
│   ├── internal/lobby/  # rooms, matchmaking state, session registry, room timers
│   ├── internal/ws/     # WebSocket transport, heartbeats, rate limits, JSON encode/decode
│   └── internal/store/  # SQLite persistence for completed games
├── docker-compose.yml   # Single-VPS deployment stack
├── README.md            # Human-facing setup and usage
└── docs/                # PRD.md, ARCHITECTURE.md, TASKS.md, protocol.md
```

## Components & Responsibilities
- **`frontend`:** Renders the Home, Lobby, Waiting Room, Ready-Up, Placement, Battle, and Game Over screens. Persists `sessionToken` and `displayName` in `localStorage`, maintains the WebSocket connection with automatic backoff reconnection, and projects server messages into UI state without computing authoritative game outcomes locally.
- **`backend/cmd/server`:** Application entrypoint. Parses environment configuration (port, allowed origin, SQLite path, configurable timers/caps), exposes `/healthz` and the WebSocket upgrade endpoint, wires `store`, `lobby`, and `ws`, and handles `SIGTERM` for graceful shutdown.
- **`backend/internal/game`:** Pure, deterministic game engine with zero I/O and no timers. Validates ship placements (10×10 bounds, 5-ship fleet, no overlaps, adjacent ships allowed), generates random fleets and partial auto-placements, resolves shots (miss, hit, sunk), selects random untargeted cells for timed-out turns, checks win conditions, and builds per-player board views so unrevealed opponent ships never leak.
- **`backend/internal/lobby`:** Manages the in-memory session registry (`playerId` ↔ `sessionToken`), public lobby list, atomic room joins, the 10-room global cap, and individual room state machines (`waiting` → `ready-up` → `placement` → `battle` → `finished`). Owns room timers (60s placement, 30s turn, 60s disconnect grace) via an injectable clock interface and serializes state mutations per room using a single owner goroutine or mutex.
- **`backend/internal/ws`:** Manages WebSocket upgrades, origin checks, ping/pong heartbeats for dead-connection detection, per-connection message size and rate limits, per-IP concurrent connection caps, and JSON envelope (`{ "type", "payload" }`) encoding/decoding. Rejects a second concurrent connection attempting to use an already-connected `sessionToken`.
- **`backend/internal/store`:** Initializes the SQLite schema and writes a single immutable record to the `games` table whenever a game concludes (`all_sunk`, `forfeit`, `disconnect_timeout`, or `abandoned`).

## Data Model

### In-Memory Entities

#### `Player`
- `playerId`: `string` — internal server-generated unique identifier; sole basis of authority.
- `displayName`: `string` — 1–20 trimmed characters, control characters stripped, non-unique.
- `sessionToken`: `string` — cryptographically random, unguessable token stored by the client in `localStorage`.
- `connectionState`: `enum (connected | disconnected)` — current WebSocket attachment status.
- `currentRoomId`: `string | null` — enforces at most one active room/game per player.
- `disconnectedAt`: `timestamp | null` — set when connection drops to track the 60s grace window.

#### `Room`
- `roomId`: `string` — unique room identifier.
- `hostPlayerId`: `string` — creator of the room.
- `guestPlayerId`: `string | null` — joined opponent (`null` while waiting).
- `phase`: `enum (waiting | ready-up | placement | battle | finished)` — lifecycle stage.
- `hostReady` / `guestReady`: `bool` — ready-up flags during `ready-up` phase and rematch acceptance flags during `finished` phase.
- `createdAt`: `timestamp` — used to sort waiting rooms oldest-first and show wait duration.
- `timers`: room-owned handles for placement deadline (60s), turn deadline (30s), and per-player disconnect grace deadlines (60s).

#### `Game`
- `boards`: `map[playerId]Board` — each player's 10×10 grid containing ship placements (Carrier 5, Battleship 4, Cruiser 3, Submarine 3, Destroyer 2) and incoming shots received.
- `fleetConfirmed`: `map[playerId]bool` — whether each player has locked in their fleet.
- `turnPlayerId`: `string` — player whose turn it currently is.
- `placementDeadline`: `timestamp` — UTC timestamp when the 60s placement timer expires.
- `turnDeadline`: `timestamp` — UTC timestamp when the current 30s turn timer expires.
- `shotLog`: `[]ShotRecord` — ordered history of shots (`shooterSlot`, `cell`, `result`, `sunkShip`, `auto`, `timestamp`).

### SQLite Schema (`games` table)
Written once per finished game (no personal data beyond self-chosen display names):
- `id`: `TEXT PRIMARY KEY` — unique game ID.
- `started_at`: `TIMESTAMP NOT NULL` — when placement/battle started.
- `ended_at`: `TIMESTAMP NOT NULL` — when the game concluded.
- `player1_name`: `TEXT NOT NULL` — host display name at match time.
- `player2_name`: `TEXT NOT NULL` — guest display name at match time.
- `winner_slot`: `INTEGER NULL` — `1`, `2`, or `NULL` if `abandoned`.
- `end_reason`: `TEXT NOT NULL` — one of `all_sunk`, `forfeit`, `disconnect_timeout`, `abandoned`.
- `total_shots`: `INTEGER NOT NULL` — total shots fired in the match.
- `shot_log`: `TEXT NOT NULL` — JSON array of ordered shots and results for future analytics/replay.

### WebSocket Protocol Summary
All messages are JSON envelopes of the form `{ "type": string, "payload": object }` (full wire specification documented in `docs/protocol.md` during implementation):
- **Client → Server:**
  - `hello` (`name`, optional `sessionToken`)
  - `lobby.create`, `lobby.cancel`, `lobby.join` (`roomId`)
  - `room.ready`, `room.unready`
  - `placement.submit` (`ships`), `placement.ready`
  - `battle.fire` (`cell`)
  - `game.leave`
  - `rematch.accept`, `rematch.decline`
- **Server → Client:**
  - `hello.ok` (`playerId`, `sessionToken`, `restoredState`)
  - `lobby.snapshot` (waiting rooms list), `lobby.update` (room added or removed)
  - `room.state` (`phase`, `opponentName`, `deadlines`), `room.readyState` (`hostReady`, `guestReady`)
  - `placement.opponentReady`
  - `battle.start` (`firstPlayerId`), `battle.shotResult` (`by`, `cell`, `result`, `sunkShip?`, `auto?`), `battle.turn` (`turnPlayerId`, `deadline`)
  - `opponent.disconnected` (`graceDeadline`), `opponent.reconnected`
  - `game.over` (`winnerPlayerId`, `reason`)
  - `rematch.offered`, `rematch.cancelled`
  - `error` (`code`, `message`)

## Data Flow
1. **Session Handshake:** Client opens a WebSocket connection and sends `hello` with `name` and any stored `sessionToken`. Server validates/sanitizes the name, either resumes the existing `Player` session or creates a new cryptographically random `sessionToken` and `playerId`, and replies with `hello.ok` containing the player's restored room/game view (or places them in the lobby with `lobby.snapshot`).
2. **Lobby & Ready-Up:** When a player sends `lobby.create` (and total rooms < 10), the server allocates a `waiting` room and broadcasts `lobby.update` to lobby clients. When a guest sends `lobby.join`, the lobby manager atomically transitions the room to `ready-up`, removes it from the public lobby broadcast, and sends `room.state` to both players. Once both send `room.ready`, the room transitions to `placement` and starts the 60s placement timer.
3. **Fleet Placement:** Each player sends `placement.submit` / `placement.ready` with their 5 ship coordinates. The room calls `game.ValidateFleet`; if valid, the fleet is locked and the opponent receives `placement.opponentReady`. If the 60s timer fires first, the room invokes `game.AutoPlaceRemaining` for any unconfirmed player. Once both fleets are locked, the server randomly picks the starting player, transitions to `battle`, sends `battle.start` and `battle.turn`, and starts the 30s turn timer.
4. **Battle Loop:** On their turn, the active player sends `battle.fire` (`cell`). The room verifies turn ownership and phase, invokes `game.FireShot`, and broadcasts `battle.shotResult` (miss, hit, or sunk with ship type and cells) to both players. If the 30s turn timer expires, the room calls `game.FireRandomShot` (`auto: true`). If all 5 ships of a fleet are sunk, the room transitions to `finished`, emits `game.over`, and writes the match record to SQLite via `store`.
5. **Disconnects & Rematches:** WebSocket ping/pong heartbeats detect dropped connections and start a 60s grace timer (`opponent.disconnected`). Reconnecting with the same `sessionToken` within 60s cancels the grace timer, sends a filtered `hello.ok` snapshot, and notifies the opponent (`opponent.reconnected`). Expiry of the grace timer ends the game as `disconnect_timeout` (or `abandoned` if both are disconnected). After a normal `all_sunk` finish, mutual `rematch.accept` messages create a fresh `Game` in the `placement` phase with a re-randomized starting player.

## External Services & Integrations
- None.

## Key Decisions
- **Pure game engine (`internal/game`):** Holds no timers, goroutines, or I/O. Takes `(state, action)` and returns `(newState, events)`. Trade-off: Requires the `lobby`/room layer to translate timer ticks and network intents into engine calls, but makes core game rules 100% deterministic and trivial to unit test.
- **Room-owned timers with injectable clock:** Placement (60s), turn (30s), and disconnect grace (60s) timers live in the room coordinator using an injectable clock interface. Trade-off: Slight boilerplate for clock abstraction, but enables fast, deterministic timeout testing without real sleeps.
- **Per-player state projection:** The server builds tailored views per player and never serializes raw opponent board structs to clients. Trade-off: Two serialization passes per state snapshot, guaranteeing zero information leakage of unrevealed ships.
- **In-memory live state + SQLite only on game end:** Active rooms and boards live exclusively in Go memory; SQLite is written once when a game finishes using pure-Go `modernc.org/sqlite`. Trade-off: A server restart drops in-flight games, which is explicitly acceptable for v1 in exchange for low latency and zero CGO/external DB dependencies.
- **Single active connection per session token:** A second WebSocket connection presenting an already-connected `sessionToken` is rejected so the existing tab stays active. Trade-off: Users cannot mirror the same game across two open tabs simultaneously, avoiding split-brain client state.

## Constraints & Non-Goals
- **Fairness & Validation (NFR-1 to NFR-4):** Server is the sole source of truth; every client message is validated for schema, size, game phase, and turn ownership. Malformed or out-of-turn messages return an `error` event and never crash the server.
- **Abuse & Resource Limits (NFR-5 to NFR-7):** Global cap of 10 concurrent rooms (waiting + active); per-connection message rate limit and max payload size; per-IP concurrent connection cap; strict WebSocket `Origin` header check; plain-text rendering of display names (1–20 chars, control chars stripped).
- **Scale & Latency Targets (NFR-8 to NFR-10):** Single VPS instance supports ~100 concurrent connections (tens of concurrent games) with < 300 ms typical shot turnaround and prompt dead-peer detection via WebSocket ping/pong heartbeats.
- **Operability (NFR-12, NFR-13):** `/healthz` HTTP endpoint for container health checks; graceful `SIGTERM` handling that stops new room creation and closes connections cleanly; timeouts and caps configurable via environment variables.
- **Deliberate Non-Goals:** No horizontal scaling, no multi-node state synchronization, no persistent accounts, and no audio in v1.

## Known Technical Debt
- **Volatile in-flight state on restart:** Restarting the Go server clears the lobby and terminates in-progress games (reconnecting clients receive a clean notice and return to the home/lobby screen).
- **No AFK forfeit on repeated turn timeouts:** Players who stay connected (or whose turn expires before disconnect timeout) will have random shots fired automatically every 30s until the match concludes naturally or they disconnect.
