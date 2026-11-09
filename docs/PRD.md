# Online Battleship: Product Requirements

<!-- This file is the product truth. NO code, frameworks, or database schemas here.
     Describe WHAT the product does and for WHOM, never HOW it is built. -->

## Vision
Online Battleship is a browser-based, real-time, two-player implementation of the classic Battleship board game. It lets two strangers or friends start and finish a fair, cheat-proof game in a couple of clicks with zero sign-up friction, graceful handling of brief connection drops, and full support for both desktop and mobile browsers.

## Target User
Casual players on desktop or mobile browsers who want a quick game against another person online—either friends sharing the site URL or strangers finding each other in the public lobby—without creating an account or logging in.

## Success Criteria
- **Zero-friction entry:** A new visitor can enter a display name and reach the fleet placement screen in under 60 seconds when another player is waiting.
- **High completion rate:** At least 70% of started games end in a normal win (all ships sunk) rather than forfeit or abandonment.
- **Responsive feedback:** Shot results are delivered to both players with typical end-to-end latency under 300 ms on a normal connection.
- **Resilient sessions:** Fewer than 1% of games end due to server error, and the majority of reconnects within the 60-second grace period successfully resume the match.
- **Classic rules enforced:** 10×10 grids (columns A–J, rows 1–10); 5-ship fleet per player (Carrier 5, Battleship 4, Cruiser 3, Submarine 3, Destroyer 2); horizontal or vertical placement fully on the grid with no overlap (adjacent/touching ships allowed); alternating single-shot turns (no extra turn on hit); random starting player; first to sink all 5 opponent ships wins.

## MVP Features

<!-- Each feature gets an ID, a short description, and a "Done when" line.
     Tasks in TASKS.md reference these IDs. -->

### F-01 Display Name & Session Identity
**What:** Frictionless player identification using a chosen display name backed by an unguessable session identity that persists across page refreshes and brief disconnects.

**User flow:**
1. The player lands on the home screen, sees a short tagline, enters a display name (1–20 characters after trimming leading/trailing whitespace), and clicks **Play**.
2. The display name is remembered locally for future visits, and the player is assigned an opaque session identity.
3. If the player refreshes the page or briefly loses connection, they are automatically restored to their active state (lobby, waiting room, ready-up, placement, or battle).

**Done when:**
- Display names of 1–20 trimmed characters are accepted (control characters stripped, rendered strictly as plain text) and remembered locally across visits.
- Duplicate display names are allowed; the UI never relies on name uniqueness and always disambiguates by room or player role.
- Reconnecting with an existing session restores the player's current state, while opening a second browser tab with the same session is rejected so the first stays active.

### F-02 Live Public Lobby
**What:** Real-time public lobby where players browse waiting games, host a new room, or join an open room.

**User flow:**
1. Entering the lobby displays all currently waiting games in real time (oldest first), showing each host's display name and elapsed wait time, or an empty state inviting the player to host.
2. A player clicks **Create game** to open a waiting room and wait for an opponent, or clicks **Join** on an existing waiting room.
3. While waiting as host, the player can click **Cancel** to close the room and return to the lobby.

**Done when:**
- Lobby list updates live without manual refresh whenever waiting rooms are created, joined, cancelled, or expired.
- A player can only be in one active room/game at a time.
- Joining a room is atomic: if two players click **Join** simultaneously, exactly one enters and the other sees a clear "game no longer available" message.
- Up to 10 total rooms (waiting or in-play) can exist at once; when the cap is reached, **Create game** is disabled with an explanatory message.
- A waiting room is removed immediately if the host leaves or cancels, or after the 60-second grace period if the host disconnects.

### F-03 Pre-Game Ready-Up
**What:** Pre-game confirmation step after a guest joins a room so neither player begins placement until both are attentive and ready.

**User flow:**
1. When a guest joins a waiting room, the room is removed from the public lobby and both players enter the ready-up screen.
2. Each player sees their opponent's display name, both players' ready status, and a **Ready** button.
3. Players click **Ready** (and may un-ready while waiting for the opponent); once both players are ready, the match advances to fleet placement.

**Done when:**
- Joining a room transitions it to ready-up without starting placement automatically.
- A player can toggle between Ready and Un-ready until both players are simultaneously ready, which immediately starts the placement phase (no timeout on ready-up).
- If a player leaves or disconnects past the 60-second grace period during ready-up, a departing guest returns the room to the waiting lobby for the host, whereas a departing host closes the room and returns the guest to the lobby.

### F-04 Fleet Placement Phase
**What:** Timed 60-second setup phase where both players position their 5 ships on their 10×10 grids manually or randomly and lock in their fleets.

**User flow:**
1. Each player sees their 10×10 grid, a ship dock containing their 5 unplaced ships, rotate and **Randomize** controls, a 60-second countdown, and an indicator showing whether the opponent has confirmed their fleet.
2. Players place ships via drag-and-drop on desktop or tap-to-select then tap-to-place on touch devices, rotate ships before or after placing, move already-placed ships, or click **Randomize** to place all 5 ships automatically.
3. The player clicks **Confirm fleet** to lock their arrangement; if the 60-second timer expires first, any remaining unplaced ships are auto-placed while keeping already-placed ships untouched, and the fleet is confirmed automatically.

**Done when:**
- Desktop drag-and-drop, touch tap-to-select/tap-to-place, ship rotation (button or key on desktop; button on mobile), moving placed ships, and **Randomize** all work smoothly.
- Out-of-bounds or overlapping placements are prevented in the UI and rejected by the server; adjacent/touching ships are permitted.
- **Confirm fleet** locks the player's board, updates the opponent's "opponent ready" indicator without leaking ship coordinates, and starts the battle phase once both fleets are confirmed (or auto-confirmed on the 60-second deadline).

### F-05 Turn-Based Battle Phase
**What:** Alternating single-shot combat with a 30-second turn timer, immediate hit/miss/sunk feedback, and automatic random shots on turn timeout.

**User flow:**
1. Once both fleets are confirmed, the starting player is chosen at random and a turn banner with a 30-second countdown appears.
2. On their turn, the active player selects an untargeted cell on the opponent's grid to fire; if the 30-second timer expires, a random untargeted cell is fired automatically on their behalf.
3. Both players see the shot result (**miss**, **hit**, or **sunk** with the sunk ship's type and cells revealed), updated remaining-ship counts, and an entry in the recent events log, and turns alternate until all 5 ships of one fleet are sunk.

**Done when:**
- First turn is chosen randomly, turns strictly alternate (one shot per turn, no bonus shot on hit), and previously targeted cells are visually distinct and unselectable.
- Every shot resolves as miss, hit, or sunk (revealing the sunk ship type and occupied cells) and is logged in a short recent-events feed.
- Expiring the 30-second turn timer fires a random untargeted cell labeled as an automatic shot and advances the turn (no AFK forfeit on repeated timeouts).
- Both grids (own fleet with incoming shots; target grid with outgoing shots), active turn indicator, 30-second countdown, and remaining ships per side are clearly displayed.

### F-06 Disconnect Handling, Reconnection & Forfeit
**What:** Graceful handling of dropped connections during placement or battle via a 60-second reconnection window, plus explicit forfeit and server-restart recovery.

**User flow:**
1. If a player's connection drops, their client automatically attempts to reconnect and shows a reconnecting indicator, while the opponent sees a disconnect banner with a 60-second countdown.
2. If the disconnected player returns within 60 seconds, play resumes seamlessly (active placement or turn timers continue running during the disconnect).
3. If the 60-second grace period expires—or if a player clicks **Leave game** and confirms—the match ends immediately.

**Done when:**
- Clients automatically reconnect with backoff and restore full match state when returning within 60 seconds.
- Disconnect expiry forfeits the disconnected player and awards the win to the connected opponent, or records the match as *abandoned* with no winner if both players are disconnected when the grace period expires.
- Clicking **Leave game** prompts for confirmation and immediately forfeits the match in favor of the opponent.
- If the server restarts while a match is in progress, reconnecting clients are returned cleanly to the home/lobby screen with a message that their in-progress game was lost.

### F-07 Game Over, Rematch & Match History Recording
**What:** End-of-match summary screen with a mutual rematch flow and persistent server-side recording of finished games.

**User flow:**
1. When a match ends, both players see the Game Over screen displaying the outcome (win, loss, or abandoned) and reason (all ships sunk, forfeit, or opponent disconnected).
2. After a normal game end, players can click **Rematch** or **Back to lobby**; if both accept **Rematch**, a new game starts directly in the placement phase with a re-randomized starting player.
3. If either player clicks **Back to lobby** or disconnects, any pending rematch offer is cancelled and the remaining player is notified.

**Done when:**
- Result screen accurately displays the outcome and end reason (`all_sunk`, `forfeit`, `disconnect_timeout`, or `abandoned`).
- **Rematch** is offered only after a normal game end where both players are present, starts a new placement phase once both accept, and cancels cleanly if either leaves or disconnects.
- Every finished game (win, forfeit, disconnect timeout, or abandoned) is persistently recorded once on completion with timestamps, display names, winner, end reason, total shots, and ordered shot log.

### F-08 Responsive UX, Accessibility & Fair Play
**What:** Responsive desktop and mobile layout, accessible visual and keyboard controls, and strict cheat prevention so hidden ship coordinates are never exposed prematurely.

**User flow:**
1. Desktop players view own and opponent grids side by side and can select cells, rotate ships, and confirm actions via mouse or keyboard.
2. Mobile players (down to ~360 px width) view the firing target grid as the primary view during battle with a toggle or compact view of their own fleet and touch-friendly cell sizes.
3. Color-blind players distinguish hit, miss, and sunk cells via distinct symbols and patterns in addition to color.

**Done when:**
- Layouts and grids remain usable without horizontal overflow from 360 px mobile screens up to desktop viewports.
- Hit, miss, and sunk markers use distinct shapes/symbols/patterns in addition to color, and core desktop interactions (cell selection, ship rotation, confirming) are keyboard accessible.
- Opponent ship positions are never transmitted to a client until revealed by a sunk ship event or the end of the game.

## Out of Scope (v1)
- Accounts, authentication, passwords, OAuth, or persistent player profiles.
- Single-player mode or AI opponents.
- Spectators.
- In-game or lobby chat of any kind (free text or preset emotes/messages).
- Rankings, ELO, leaderboards, skill-based matchmaking, or in-app match history/replay UI.
- Alternate rule sets (salvo mode, shoot-again-on-hit, custom board sizes, custom fleets, ship-spacing rules).
- Sound effects or background audio.
- Name moderation/profanity filters and automatic AFK forfeit on repeated turn timeouts.
- Multi-instance horizontal scaling and native mobile apps.

## Open Questions
- None for v1. All initial product questions (timers, 10-room cap, single active tab per session, no ready-up timeout, no AFK forfeit on auto-shots) are resolved.

## Later Versions
### v1.1 (Planned)
- **Single-player AI opponent:** First priority after v1 so solo visitors can play immediately when the lobby is empty.
- **Match history & replay UI:** Expose persisted match records and shot logs in the UI.
