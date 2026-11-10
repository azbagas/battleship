# TASKS

<!-- The plan and the progress tracker. Every PRD feature must have at least one task.
     Tag tasks with feature IDs. Each task has a "Done when" line. -->

## Current focus
In progress: none

Blocked: nothing

Known issues: none

Next up: T-08 [F-04] Implement pure `internal/game` board & fleet placement logic (10×10 grid, 5 standard ships, bounds/overlap validation, adjacent ships allowed, full random fleet generation, and partial auto-placement for unplaced ships).

<!-- Update this block at the end of every session. If it passes ~15 lines, move it to STATUS.md. -->

## v1

### Setup
- [x] T-01 [setup] Scaffold monorepo (`backend/` Go module, `frontend/` Vite + React + TypeScript, `docker-compose.yml`), test runners, linters, and `/healthz` endpoint.
  Done when: `go test ./...`, frontend test/lint/build commands, and `GET /healthz` all succeed locally and via Docker Compose.
- [x] T-01b [setup] Add root dev orchestrator script with `concurrently` to run frontend and backend simultaneously via a single `npm run dev` command.
  Done when: `npm run dev` starts both backend and frontend concurrently in one terminal with color-coded labels and clean shutdown, and README.md and AGENTS.md are updated.

### F-01 Display Name & Session Identity
- [x] T-02 [F-01] Implement backend session registry, WebSocket handshake (`hello` / `hello.ok`), display name sanitization (1–20 trimmed chars, control chars stripped), and single-active-connection rule per session token.
  Done when: unit and WebSocket tests verify token issuance, session restoration on reconnect, name validation bounds, and rejection of a second concurrent connection with the same token.
- [x] T-03 [F-01] Build frontend Home screen with display name input, `localStorage` persistence (`sessionToken`, `displayName`), WebSocket client wrapper, and automatic state restoration across page refreshes.
  Done when: a player can enter a name, connect, refresh the browser, and remain recognized with the same session identity.

### F-02 Live Public Lobby
- [x] T-04 [F-02] Implement backend in-memory lobby manager (`lobby.create`, `lobby.cancel`, atomic `lobby.join`, `lobby.snapshot`, `lobby.update`) enforcing one active game per player and the 10-room global cap.
  Done when: unit and `-race` concurrency tests confirm real-time lobby broadcasts, atomic simultaneous joins (exactly one succeeds), host cancel, and 10-room cap enforcement.
- [x] T-05 [F-02] Build frontend Lobby screen and Host Waiting Room UI showing live waiting rooms (oldest first with host name and wait duration), empty state, **Create game**, **Join**, **Cancel**, and disabled state when the 10-room cap is reached.
  Done when: creating, cancelling, and joining rooms update live across two connected browsers without manual refresh.
- [x] T-05b [F-02] Add structured backend logging with slog across session, lobby, and WebSocket connection lifecycles.
  Done when: server startup configures slog with env-driven level/format, room and connection lifecycle events emit structured logs without exposing secret tokens, and backend tests pass with `-race`.

### F-03 Pre-Game Ready-Up
- [x] T-06 [F-03] Implement backend `ready-up` room phase (`room.ready`, `room.unready`, `room.readyState`) and leave/disconnect handling during ready-up (returning room to lobby if guest leaves, or closing room if host leaves).
  Done when: room state machine tests verify ready/un-ready toggling, transition to placement only when both players are ready, and guest/host departure rules.
- [x] T-07 [F-03] Build frontend Ready-Up screen displaying opponent name, both players' ready status, **Ready** / **Un-ready** toggle button, and **Leave** action.
  Done when: two joined players can toggle ready states and only advance to placement once both have clicked **Ready**.

### F-04 Fleet Placement Phase
- [ ] T-08 [F-04] Implement pure `internal/game` board & fleet placement logic (10×10 grid, 5 standard ships, bounds/overlap validation, adjacent ships allowed, full random fleet generation, and partial auto-placement for unplaced ships).
  Done when: unit tests exhaustively cover valid/invalid placements, random fleet validity, and preserving already-placed ships when auto-placing remaining ships.
- [ ] T-09 [F-04] Implement backend placement phase orchestration (`placement.submit`, `placement.ready`, `placement.opponentReady`) and the 60-second placement timer with automatic completion on timeout using an injectable clock.
  Done when: room timer tests verify valid fleet lock-in, rejection of invalid placements, opponent ready notification without leaking coordinates, and auto-placement when the 60s clock expires.
- [ ] T-10 [F-04] Build frontend Placement screen with 10×10 grid, unplaced ship dock, desktop drag-and-drop, touch tap-to-select/tap-to-place, ship rotation, **Randomize**, **Confirm fleet**, 60s countdown, and opponent confirmed indicator.
  Done when: players on both desktop and touch viewports can place, move, rotate, randomize, and confirm fleets (or let the 60s timer auto-confirm) to enter battle.

### F-05 Turn-Based Battle Phase
- [ ] T-11 [F-05] Implement pure `internal/game` shot resolution (`miss`, `hit`, `sunk` with sunk ship type and cells), random first-player selection, random untargeted cell selector for auto-shots, win detection, and per-player filtered board views.
  Done when: unit tests verify hit/miss/sunk transitions, win detection when all 5 ships sink, random untargeted shot selection, and zero leakage of unrevealed opponent ships in player views.
- [ ] T-12 [F-05] Implement backend battle phase loop (`battle.start`, `battle.fire`, `battle.shotResult`, `battle.turn`, `game.over`) and the 30-second turn timer that fires an automatic random shot (`auto: true`) on timeout.
  Done when: room tests with an injectable clock verify turn alternation, out-of-turn/duplicate-cell rejection, 30s auto-shot execution, and transition to `finished` on win.
- [ ] T-13 [F-05] Build frontend Battle screen with own and target grids, turn banner, 30s countdown, remaining ships indicator, hit/miss/sunk feedback, and recent events log.
  Done when: two players can play a complete battle from first shot to sinking all 5 ships and see accurate turn timers, shot markers, sunk ship reveals, and event log entries.

### F-06 Disconnect Handling, Reconnection & Forfeit
- [ ] T-14 [F-06] Implement backend WebSocket ping/pong heartbeats, 60-second disconnect grace timer (`opponent.disconnected`, `opponent.reconnected`), timeout forfeit/abandonment resolution, and explicit `game.leave` forfeit.
  Done when: room and timer tests verify seamless state restoration within 60s, opponent win on single-player disconnect timeout or `game.leave`, and `abandoned` outcome when both players time out.
- [ ] T-15 [F-06] Build frontend automatic reconnection with backoff, connection status indicator, opponent disconnect countdown banner, **Leave game** confirmation prompt, and server-restart lost-game notification.
  Done when: refreshing or dropping connection briefly resumes the match, opponent sees the 60s disconnect banner, confirming **Leave game** forfeits immediately, and reconnecting after a server restart returns cleanly to the lobby with a notice.

### F-07 Game Over, Rematch & Match History Recording
- [ ] T-16 [F-07] Implement `internal/store` SQLite persistence (`modernc.org/sqlite`) to write a `games` record once per finished match (`all_sunk`, `forfeit`, `disconnect_timeout`, `abandoned`) including shot count and JSON `shot_log`.
  Done when: store unit tests verify schema creation and accurate row insertion for all four game-end reasons without CGO.
- [ ] T-17 [F-07] Implement backend rematch flow (`rematch.accept`, `rematch.decline`, `rematch.offered`, `rematch.cancelled`) after normal game ends, starting a new placement phase with re-randomized first turn when both accept.
  Done when: room tests verify mutual rematch starts placement with the same two players, while decline/leave/disconnect cancels the offer and notifies the remaining player.
- [ ] T-18 [F-07] Build frontend Game Over screen showing match outcome and reason, **Rematch** button (hidden on forfeit/abandonment) with waiting/cancelled states, and **Back to lobby** button.
  Done when: two players finishing a normal game can mutually accept a rematch or return to the lobby with clear status feedback.

### F-08 Responsive UX, Accessibility & Fair Play
- [ ] T-19 [F-08] Add backend security and operational hardening: WebSocket `Origin` check, per-connection message rate limiting and max payload size, per-IP connection cap, graceful `SIGTERM` shutdown, and `docs/protocol.md` specification.
  Done when: tests verify rate-limit/size/origin/IP rejections and graceful shutdown, and `docs/protocol.md` documents every client and server message schema.
- [ ] T-20 [F-08] Polish frontend responsive layout (360 px mobile with primary target grid + own-fleet toggle up to desktop side-by-side grids), non-color hit/miss/sunk symbols/patterns, and desktop keyboard navigation (cell selection, rotation, confirmation).
  Done when: UI works cleanly at 360 px mobile and desktop widths, hit/miss/sunk states are distinguishable without color, and keyboard controls operate placement and firing.

### Wrap-up
- [ ] T-21 [all] Write two-client WebSocket integration tests (full game, reconnect, rematch, simultaneous join race) and a Playwright E2E test of two browsers completing a game.
  Done when: all backend integration tests (`go test -race ./...`) and Playwright E2E tests pass reliably.
- [ ] T-22 [all] Compare `PRD.md` to the code: confirm every "Done when" is met and list gaps.
  Done when: gap list is empty or turned into new tasks.
- [ ] T-23 [all] Manual walkthrough of every user flow on desktop and mobile viewports.
  Done when: all flows work end to end.

## v1.1
<!-- Add new feature groups here AFTER the PRD has been updated and approved. -->

## Notes / Decisions
<!-- Non-obvious choices made during implementation, with a date.
     Format: YYYY-MM-DD: what was decided and why. -->
- 2026-11-09: Initial v1 defaults locked from PRD: 60s placement timer, 30s turn timer (auto-fires random untargeted cell on timeout; no AFK forfeit on repeated timeouts), 60s disconnect grace period, 10-room global cap (waiting + active), no timeout on pre-game ready-up, and second tab with same session token rejected.
- 2026-11-09: T-02 implemented using `github.com/coder/websocket`; unrecognized session token in `hello` gracefully issues a new session with fresh tokens to handle server restarts seamlessly.
- 2026-11-09: T-03 frontend implemented with React 19 context + WebSocket client wrapper; handles automatic session restoration from `localStorage`, reverse proxies `/ws` in Vite dev and Nginx, and halts reconnection on `SESSION_ALREADY_ACTIVE` with an explicit resolution CTA.
- 2026-11-09: T-04 implemented with in-memory LobbyManager, thread-safe Clock abstraction, and mutex-serialized room transitions outside network I/O; snapshot & update broadcasts carry totalRooms & maxRooms to enforce and expose the 10-room global cap to clients.
- 2026-11-09: T-05 frontend implemented with LobbyProvider, useElapsedTime ticker, live LobbyScreen with room capacity meter, and Host WaitingRoomScreen with animated sonar radar; room cap reaches disable Create Game with explanatory tooltip and alert.
- 2026-11-10: T-05b implemented with structured `log/slog` logging across WebSocket transport, session authentication, and lobby/room lifecycles. Default log level is `INFO` (configurable via `LOG_LEVEL`) and text/json format (configurable via `LOG_FORMAT`); session tokens are redacted to protect security.
- 2026-11-10: Hardened `WebSocketClient.connect` to be idempotent for identical connection options and cleanly detach/close obsolete socket instances before opening new ones, preventing ghost socket timeouts and false session collision triggers under React StrictMode.
- 2026-11-10: T-01b added root package.json orchestrator with `concurrently` to run backend and frontend simultaneously with color-coded logs and graceful SIGINT cleanup via `npm run dev`.
- 2026-11-10: T-06 & T-07 implemented ready-up room phase with `room.ready`, `room.unready`, `room.readyState`, and `room.leave` / `game.leave`. Enforces departure rules (guest leaves/disconnect timeout returns room to waiting lobby with public lobby upsert; host leaves/disconnect timeout destroys room and returns guest to lobby; 60s disconnect grace period applies before timeout resolution). Mutual readiness automatically transitions the room to the placement phase and renders `PlacementPlaceholder`.

## Archive
<!-- Move finished version groups here as one-line summaries, e.g. "v1 shipped: F-01 to F-03." -->
