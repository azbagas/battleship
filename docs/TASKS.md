# TASKS

<!-- The plan and the progress tracker. Every PRD feature must have at least one task.
     Tag tasks with feature IDs. Each task has a "Done when" line. -->

## Current focus
In progress: none (v1 complete)

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

## Archive
<!-- Move finished version groups here as one-line summaries, e.g. "v1 shipped: F-01 to F-03." -->
- v1 shipped (2026-11-16): Setup (T-01, T-01b), F-01 Session Identity (T-02–T-03), F-02 Live Lobby & Logging (T-04–T-05b), F-03 Ready-Up (T-06–T-07), F-04 Fleet Placement (T-08–T-10), F-05 Turn-Based Battle (T-11–T-13), F-06 Disconnect/Reconnect/Forfeit (T-14–T-15), F-07 Game Over/Rematch/SQLite Store (T-16–T-18), F-08 Security/Responsive/A11y/SVG Visuals (T-19–T-20b), and Wrap-up E2E/PRD Audit/Walkthrough (T-21–T-23).
