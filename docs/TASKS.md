# TASKS

<!-- The plan and the progress tracker. Every PRD feature must have at least one task.
     Tag tasks with feature IDs. Each task has a "Done when" line. -->

## Current focus
In progress: none (v1 + frontend/backend refactors complete)

Blocked: nothing

Known issues: none

Next up: v1.1 planning (Single-player AI opponent & Match history/replay UI per PRD)

<!-- Update this block at the end of every session. If it passes ~15 lines, move it to STATUS.md. -->

## v1.1
> _Add new feature groups here AFTER [PRD.md](PRD.md) has been updated and approved._

## Notes / Decisions
<!-- Non-obvious choices made during implementation, with a date.
     Format: YYYY-MM-DD: what was decided and why. -->
- 2026-11-09: Locked v1 defaults from PRD: 60s placement timer, 30s turn timer (auto-fires random untargeted cell on timeout; no AFK forfeit on repeated timeouts), 60s disconnect grace period, 10-room global cap (waiting + active), no ready-up timeout, and second tab with same session token rejected (`SESSION_ALREADY_ACTIVE`).
- 2026-11-09: Unrecognized `sessionToken` in `hello` gracefully issues a fresh session (`restoredState: null`) so server restarts return clients cleanly to the lobby; `WebSocketClient.connect` is idempotent for identical options to avoid false session-conflict errors under React StrictMode.
- 2026-11-10: `HandleReconnect` returns filtered `*RoomStateView` (avoiding timer channel marshal failures and coordinate leaks) and registers the connection sender after `hello.ok` is written.
- 2026-11-11: `internal/store` uses pure-Go `modernc.org/sqlite` (WAL mode, 5s busy timeout, zero CGO); `PhaseFinished` rooms are excluded from the 10-room active capacity count (`activeRoomsCountLocked`) and cleaned up with `lobby.update` broadcasts once both players leave or disconnect.
- 2026-11-12: Rate limits, payload limits, origin checks, and per-IP connection caps run in-memory per process (env-configurable); `SIGTERM` drains active WebSockets with close code `1001` within 5s.
- 2026-11-16: `placement.submit` accepts `draft: true` to sync partial unconfirmed fleets so 60s placement timeouts and mid-placement reconnects preserve already-placed ships; `LobbyProvider` preserves `totalShots` and hydrates `recentEvents` from `shotLog` across `room.state` updates.
- 2026-11-17: Refactored frontend into phase-scoped `src/features/` modules (`home`, `lobby`, `ready-up`, `placement`, `battle`, `game-over`) with shared UI in `src/components/`, shared hooks (`useCountdown`, `useGridKeyboardNav`), and `utils/roomParticipants.ts`; de-duplicated `LobbyProvider` via `hydrateFromRoomState`, `roomStateMappers.ts`, and `lobbyErrorMessages.ts` while preserving the `useLobby()` API and leaving `ShipSvg.tsx` intact.
- 2026-11-17: Security hardening pass: gated `X-Real-IP`/`X-Forwarded-For` extraction behind `TRUST_PROXY`, added per-IP handshake rate limiting, added TTL/cap eviction for disconnected roomless sessions in `SessionRegistry`, replaced `http.Server.WriteTimeout` with `ReadHeaderTimeout` and added bounded async outbox + 5s write timeouts in `wsSender`, switched fallback game RNG to `crypto/rand`, prevented mid-room display name updates on reconnect, and hardened Docker/Nginx configs (non-root user, `.dockerignore` for `.env`/`*.db*`, `X-Forwarded-For` overwrite, and CSP/security headers).
- 2026-11-17: Refactored Go backend (`B-01`–`B-05`) with zero behavior change: deduplicated `internal/game/board.go` (`isShipSunk`, `validateFleetShips`), split `internal/lobby/manager.go` (2,317 lines) into feature-scoped files (`lobby_ops.go`, `ready_ops.go`, `placement_ops.go`, `battle_ops.go`, `disconnect_ops.go`, `rematch_ops.go`, `persistence.go`) with typed `events.go` payloads (aliased in `ws/protocol.go`) and `Room` timer/role helpers, extracted `internal/ws/sender.go` and cleaned up `ws/handler.go`, and extracted `cmd/server/config.go`.
- 2026-11-17: Fixed finished-room rematch cancellation (`ready_ops.go`, `disconnect_ops.go`, `rematch_ops.go`, `LobbyProvider.tsx`, `GameOverScreen.tsx`): `leaveFinishedLocked` and `HandleDisconnect` (`PhaseFinished`) now emit `rematch.cancelled` on normal `all_sunk` finishes even when no rematch offer is active yet, `RoomStateView` includes `rematchOfferedBy`/`rematchCancelReason` for reconnect hydration, `OPPONENT_NOT_PRESENT` cancels rematch state in `LobbyProvider`, and `GameOverScreen` renders `lobbyError` alerts.

## Archive
<!-- Move finished version groups here as one-line summaries, e.g. "v1 shipped: F-01 to F-03." -->
- v1 shipped (2026-11-16): Setup (T-01, T-01b), F-01 Session Identity (T-02–T-03), F-02 Live Lobby & Logging (T-04–T-05b), F-03 Ready-Up (T-06–T-07), F-04 Fleet Placement (T-08–T-10), F-05 Turn-Based Battle (T-11–T-13), F-06 Disconnect/Reconnect/Forfeit (T-14–T-15), F-07 Game Over/Rematch/SQLite Store (T-16–T-18), F-08 Security/Responsive/A11y/SVG Visuals (T-19–T-20b), and Wrap-up E2E/PRD Audit/Walkthrough (T-21–T-23).
- Frontend refactor (2026-11-17): Split screens into `src/features/<screen>/`, extracted shared hooks/components, and deduplicated `LobbyProvider` (R-01–R-06).
- Backend refactor (2026-11-17): Deduplicated `internal/game/board.go`, split `internal/lobby` into `*_ops.go` with typed `events.go`, extracted `internal/ws/sender.go` and `cmd/server/config.go` (B-01–B-05).
