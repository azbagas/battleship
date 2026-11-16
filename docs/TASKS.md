# TASKS

<!-- The plan and the progress tracker. Every PRD feature must have at least one task.
     Tag tasks with feature IDs. Each task has a "Done when" line. -->

## Current focus
In progress: none (v1 complete)

Blocked: nothing

Known issues: none

Next up: v1.1 planning (Single-player AI opponent & Match history/replay UI per PRD)

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
- [x] T-08 [F-04] Implement pure `internal/game` board & fleet placement logic (10×10 grid, 5 standard ships, bounds/overlap validation, adjacent ships allowed, full random fleet generation, and partial auto-placement for unplaced ships).
  Done when: unit tests exhaustively cover valid/invalid placements, random fleet validity, and preserving already-placed ships when auto-placing remaining ships.
- [x] T-09 [F-04] Implement backend placement phase orchestration (`placement.submit`, `placement.ready`, `placement.opponentReady`) and the 60-second placement timer with automatic completion on timeout using an injectable clock.
  Done when: room timer tests verify valid fleet lock-in, rejection of invalid placements, opponent ready notification without leaking coordinates, and auto-placement when the 60s clock expires.
- [x] T-10 [F-04] Build frontend Placement screen with 10×10 grid, unplaced ship dock, desktop drag-and-drop, touch tap-to-select/tap-to-place, ship rotation, **Randomize**, **Confirm fleet**, 60s countdown, and opponent confirmed indicator.
  Done when: players on both desktop and touch viewports can place, move, rotate, randomize, and confirm fleets (or let the 60s timer auto-confirm) to enter battle.

### F-05 Turn-Based Battle Phase
- [x] T-11 [F-05] Implement pure `internal/game` shot resolution (`miss`, `hit`, `sunk` with sunk ship type and cells), random first-player selection, random untargeted cell selector for auto-shots, win detection, and per-player filtered board views.
  Done when: unit tests verify hit/miss/sunk transitions, win detection when all 5 ships sink, random untargeted shot selection, and zero leakage of unrevealed opponent ships in player views.
- [x] T-12 [F-05] Implement backend battle phase loop (`battle.start`, `battle.fire`, `battle.shotResult`, `battle.turn`, `game.over`) and the 30-second turn timer that fires an automatic random shot (`auto: true`) on timeout.
  Done when: room tests with an injectable clock verify turn alternation, out-of-turn/duplicate-cell rejection, 30s auto-shot execution, and transition to `finished` on win.
- [x] T-13 [F-05] Build frontend Battle screen with own and target grids, turn banner, 30s countdown, remaining ships indicator, hit/miss/sunk feedback, and recent events log.
  Done when: two players can play a complete battle from first shot to sinking all 5 ships and see accurate turn timers, shot markers, sunk ship reveals, and event log entries.

### F-06 Disconnect Handling, Reconnection & Forfeit
- [x] T-14 [F-06] Implement backend WebSocket ping/pong heartbeats, 60-second disconnect grace timer (`opponent.disconnected`, `opponent.reconnected`), timeout forfeit/abandonment resolution, and explicit `game.leave` forfeit.
  Done when: room and timer tests verify seamless state restoration within 60s, opponent win on single-player disconnect timeout or `game.leave`, and `abandoned` outcome when both players time out.
- [x] T-15 [F-06] Build frontend automatic reconnection with backoff, connection status indicator, opponent disconnect countdown banner, **Leave game** confirmation prompt, and server-restart lost-game notification.
  Done when: refreshing or dropping connection briefly resumes the match, opponent sees the 60s disconnect banner, confirming **Leave game** forfeits immediately, and reconnecting after a server restart returns cleanly to the lobby with a notice.

### F-07 Game Over, Rematch & Match History Recording
- [x] T-16 [F-07] Implement `internal/store` SQLite persistence (`modernc.org/sqlite`) to write a `games` record once per finished match (`all_sunk`, `forfeit`, `disconnect_timeout`, `abandoned`) including shot count and JSON `shot_log`.
  Done when: store unit tests verify schema creation and accurate row insertion for all four game-end reasons without CGO.
- [x] T-17 [F-07] Implement backend rematch flow (`rematch.accept`, `rematch.decline`, `rematch.offered`, `rematch.cancelled`) after normal game ends, starting a new placement phase with re-randomized first turn when both accept.
  Done when: room tests verify mutual rematch starts placement with the same two players, while decline/leave/disconnect cancels the offer and notifies the remaining player.
- [x] T-18 [F-07] Build frontend Game Over screen showing match outcome and reason, **Rematch** button (hidden on forfeit/abandonment) with waiting/cancelled states, and **Back to lobby** button.
  Done when: two players finishing a normal game can mutually accept a rematch or return to the lobby with clear status feedback.

### F-08 Responsive UX, Accessibility & Fair Play
- [x] T-19 [F-08] Add backend security and operational hardening: WebSocket `Origin` check, per-connection message rate limiting and max payload size, per-IP connection cap, graceful `SIGTERM` shutdown, and `docs/PROTOCOL.md` specification.
  Done when: tests verify rate-limit/size/origin/IP rejections and graceful shutdown, and `docs/PROTOCOL.md` documents every client and server message schema.
- [x] T-20 [F-08] Polish frontend responsive layout (360 px mobile with primary target grid + own-fleet toggle up to desktop side-by-side grids), non-color hit/miss/sunk symbols/patterns, and desktop keyboard navigation (cell selection, rotation, confirmation).
  Done when: UI works cleanly at 360 px mobile and desktop widths, hit/miss/sunk states are distinguishable without color, and keyboard controls operate placement and firing.
- [x] T-20b [F-04, F-05, F-08] Design and integrate top-down tactical naval SVG ship visuals across Placement and Battle screens (continuous multi-cell grid vessels, ghost placement preview, Fleet Dock blueprints, Battle Fleet Status silhouettes, revealed sunk enemy vessels) along with sonar ocean grid and hit/miss/sunk visual polish.
  Done when: all 5 ship classes render distinct top-down vector hulls across grids, docks, and status cards in both orientations, ocean grids and shot markers feature tactical sonar/impact visuals while preserving colorblind symbols, and all frontend tests, linter, and build succeed.

### Wrap-up
- [x] T-21 [all] Write two-client WebSocket integration tests (full game, reconnect, rematch, simultaneous join race) and a Playwright E2E test of two browsers completing a game.
  Done when: all backend integration tests (`go test -race ./...`) and Playwright E2E tests pass reliably.
- [x] T-22 [all] Compare `PRD.md` to the code: confirm every "Done when" is met and list gaps.
  Done when: gap list is empty or turned into new tasks.
- [x] T-23 [all] Manual walkthrough of every user flow on desktop and mobile viewports.
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
- 2026-11-10: T-08, T-09, & T-10 implemented pure `internal/game` engine (10×10 grid, 5 standard ships, validation, random fleet generation, partial auto-placement preservation), backend placement orchestration (`placement.submit`, `placement.ready`, `placement.confirmed`, `placement.opponentReady`, 60-second placement timer with auto-completion on deadline), and frontend responsive Placement screen with desktop HTML5 drag-and-drop, mobile tap-to-place, rotation ('R' key/button), Randomize, and Confirm fleet. Coordinates are never leaked in opponent ready notifications.
- 2026-11-10: T-11 implemented pure `internal/game` combat mechanics: shot resolution into miss/hit/sunk, sunk vessel details (type and cells), random first-player selection, uniform random untargeted cell selector for turn timeouts, win detection on sinking all 5 ships, and zero-leakage player board view projection verified against JSON serialization leaks.
- 2026-11-10: T-12 implemented backend battle phase loop (`battle.start`, `battle.fire`, `battle.shotResult`, `battle.turn`, `game.over`), 30-second turn timer with injectable clock and random auto-shots on timeout, strict turn alternation, out-of-turn and duplicate-cell rejection, and room transition to `PhaseFinished` on win. Full room and WebSocket integration tests verify race freedom.
- 2026-11-10: T-13 implemented frontend Battle screen (`BattleScreen.tsx`) featuring primary Target Radar Grid (10×10), Defensive Fleet Grid (10×10), 30s urgency turn timer with countdown progress bar and `<10s` red alert pulsing, friendly/hostile remaining ships rosters with health pips and sunk badges, colorblind-accessible symbols (`•` miss, `✕` hit, sunken hull outline), live tactical combat event log with `AUTO` badges, responsive mobile tab switcher (supporting 360px viewports without horizontal scrolling), surrender confirmation modal, and Game Over overlay announcing victory/defeat with lobby return CTA. Verified with 13 frontend test suites (89 passing tests), clean ESLint, and zero TypeScript build errors.
- 2026-11-10: T-14 & T-15 implemented disconnect handling, heartbeats, reconnection, and forfeit across backend and frontend (F-06). Backend incorporates a 15s WebSocket ping/pong heartbeat with 5s timeout, 60s disconnect grace period across ready-up, placement, and battle phases (`opponent.disconnected`, `opponent.reconnected`), concurrent turn/placement timer execution during disconnect, grace timeout resolution (`disconnect_timeout` forfeit vs mutual `abandoned`), and explicit `game.leave` forfeit in placement/battle phases. Frontend features exponential backoff auto-reconnection, `OpponentDisconnectBanner` with live seconds countdown (<10s urgency styling), Leave Game confirmation modals in `PlacementScreen` and `BattleScreen`, and server-restart lost-game notice alert in `LobbyScreen`. Verified with backend unit/integration tests (`-race`), 14 frontend test suites (100 passing tests), and clean builds.
- 2026-11-10: Fixed reconnection during active placement/battle: `HandleReconnect` was returning internal `*Room` containing active `Timer` channels (`<-chan time.Time`) which caused JSON marshaling failures on `hello.ok`. Updated `HandleReconnect` to return filtered `*RoomStateView` and register WebSocket connection senders after `hello.ok` handshake, ensuring clean JSON serialization, zero coordinate leakage, and guaranteed `hello.ok` delivery as the first message on the wire. Added integration test `TestHandler_ReconnectInPlacementRestoresState`.
- 2026-11-11: T-16 implemented pure-Go SQLite persistence (`internal/store`) using `modernc.org/sqlite` without CGO. Automatically initializes the `games` table schema with WAL journal mode and 5s busy timeout. Integrated `LobbyManager` to write an immutable summary record exactly once per completed match (`all_sunk`, `forfeit`, `disconnect_timeout`, `abandoned`), persisting timestamps, display names, winner slot, total shot count, and JSON-marshaled `shot_log` asynchronously with context timeouts. Verified with unit and concurrency tests (`-race`).
- 2026-11-11: T-17 implemented backend rematch orchestration (`rematch.accept`, `rematch.decline`, `rematch.offered`, `rematch.cancelled`). Rematch is permitted strictly after `all_sunk` match conclusions while both commanders remain connected; forfeit, disconnect timeout, and abandonment reject rematch requests with `REMATCH_NOT_ALLOWED`. Unilateral acceptance broadcasts `rematch.offered`; mutual acceptance resets fleet, boards, and shot logs, creates a new game record, and restarts directly into the 60s placement phase with turn re-randomization on battle start. Explicit decline, room departure, or connection drop immediately cancels any pending rematch offer and notifies the counterpart. Verified with unit and WebSocket integration tests (`-race`).
- 2026-11-11: T-18 implemented the frontend Game Over screen (`GameOverScreen.tsx`) and complete rematch UX. Displays high-contrast victory/defeat debrief banners, match outcome reasons (hostile fleet destroyed, surrendered, disconnect grace timeout, or mutual abandonment), and combat statistics (total shots fired, hit accuracy %, ships sunk/lost). Rematch button is conditionally enabled exclusively after `all_sunk` finishes, with reactive waiting ("Rematch Offered — Waiting..."), opponent request ("Accept Rematch"), and cancelled notices. Surrender/forfeit or disconnect timeout cleanly hides the Rematch action and offers a direct "Back to Lobby" CTA. Routing integrated in `App.tsx` when `activeRoom.phase === 'finished'`. Verified with 15 frontend test suites (113 passing tests), clean ESLint, and zero TypeScript build errors.
- 2026-11-11: Fixed active room capacity accounting and lobby subscriber synchronization: concluded matches (`PhaseFinished`) are excluded from active capacity counts per PRD F-02 ("10 total rooms (waiting or in-play)"), ensuring players returning to the lobby see 0 active rooms instead of ghost in-flight counts. Added `lobby.update` broadcasts on room deletion, departure, and match conclusions so all players in the lobby receive instant capacity updates and stay perfectly synchronized. Added unit tests `TestFinishedMatch_ReturnToLobbyActiveRoomsCountAndUpdate` and `TestFinishedMatch_OpponentDisconnectCleansUpRoom`.
- 2026-11-12: T-19 implemented backend operational and security hardening: WebSocket origin verification via `AcceptOptions.OriginPatterns` (configurable via `ALLOWED_ORIGINS`), message payload read limits via `conn.SetReadLimit` (configurable via `MAX_PAYLOAD_BYTES`, default 32 KB), per-connection thread-safe token bucket rate limiter (configurable via `RATE_LIMIT_MESSAGES_PER_SEC` and `RATE_LIMIT_BURST`, default 10 msg/s with 20 burst), per-IP concurrent connection cap tracking IPv4 and IPv6 client addresses (configurable via `MAX_CONNS_PER_IP`, default 10 conns/IP), and graceful SIGTERM shutdown closing active sockets cleanly with code 1001 (`websocket.StatusGoingAway`) within a 5s drain window. Wire protocol fully specified in `docs/PROTOCOL.md`. Verified with race-enabled tests (`go test -race ./...`).
- 2026-11-12: T-20 implemented frontend responsive layout polish and accessibility: 360 px mobile viewports supported across placement and combat with zero horizontal scrolling (compact 26px cells, reduced container padding, and mobile tabbed view for radar/fleet/combat log); colorblind-accessible tactical markers (miss `○`, hit `✕`, sunk `⚓` with high-contrast diagonal hatch pattern); full desktop keyboard controls for fleet placement (1-5 dock selector, Arrow navigation, R rotation, Enter placement, C confirmation) and combat salvo targeting (Arrow navigation, Enter/Space fire) with roving tab indices and focus reticles. Verified with 15 test suites (117 tests passing), clean ESLint, and production Vite build.
- 2026-11-16: T-20b implemented top-down vector SVG ship blueprints (`ShipSvg` and `GridShipOverlay`) for all 5 vessel classes (Carrier angled flight deck & catapults, Battleship triple-barrel turrets & citadel, Cruiser VLS matrix & helipad, Submarine teardrop hull & missile hatches, Destroyer stealth hull & torpedo tubes). Integrated continuous multi-cell SVG vessels on the Placement Ocean Grid (with live emerald/crimson ghost placement previews), Fleet Dock cards, Battle Defensive Grid, Battle Target Radar (reconstructing and revealing opponent sunken hulls via `sunkShipToPlacedShip`), and Battle Fleet Status cards, alongside tactical sonar ring grid backdrops and impact/splash shot effects. Verified with 16 test suites (120 passing tests), clean ESLint, and production Vite build.
- 2026-11-16: T-21, T-22, & T-23 completed v1 wrap-up. Added two-client WebSocket integration tests (`TestWebSocket_TwoClientFullGameAndPersistence`, `TestWebSocket_TwoClientBattleReconnectAndResume`, `TestWebSocket_TwoClientFullGameAndRematch`, `TestWebSocket_DraftPlacementPreservedOnTimeout`, `TestWebSocket_SimultaneousJoinRace`) and Playwright two-browser E2E tests (`full-game.spec.ts` and `mobile-and-flows.spec.ts`). Closed two edge-case gaps found during PRD audit: synced unconfirmed draft ship placements (`draft: true` on `placement.submit`) so partial fleets are preserved on 60s placement timeout and mid-placement reconnect, and preserved `totalShots` and `recentEvents` from `shotLog` when `room.state` follows `game.over` or restores a battle session. Verified all flows across desktop (1280×800) and mobile (360×640) viewports.

## Archive
<!-- Move finished version groups here as one-line summaries, e.g. "v1 shipped: F-01 to F-03." -->
- v1 shipped: F-01 to F-08 (T-01 through T-23).
