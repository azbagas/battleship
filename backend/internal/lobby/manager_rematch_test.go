package lobby

import (
	"errors"
	"testing"

	"github.com/azbagas/battleship/backend/internal/game"
)

// setupFinishedMatch creates a room and plays it to all_sunk victory by host.
func setupFinishedMatch(t *testing.T) (*SessionRegistry, *LobbyManager, *MockClock, *Player, *Player, *mockSender, *mockSender, *Room) {
	t.Helper()
	reg, mgr, clock, pHost, pGuest, sHost, sGuest, room := setupPlacementTest(t)

	if err := mgr.SubmitFleet(pHost.ID, validFleetForTest()); err != nil {
		t.Fatalf("SubmitFleet host failed: %v", err)
	}
	if err := mgr.SubmitFleet(pGuest.ID, validGuestFleetForTest()); err != nil {
		t.Fatalf("SubmitFleet guest failed: %v", err)
	}

	// Host sinks all ships on guest board:
	// Carrier: (5,0)..(5,4)
	// Battleship: (6,0)..(6,3)
	// Cruiser: (7,0)..(7,2)
	// Submarine: (8,0)..(8,2)
	// Destroyer: (9,0)..(9,1)
	guestTargets := []game.Coord{
		{X: 5, Y: 0}, {X: 5, Y: 1}, {X: 5, Y: 2}, {X: 5, Y: 3}, {X: 5, Y: 4},
		{X: 6, Y: 0}, {X: 6, Y: 1}, {X: 6, Y: 2}, {X: 6, Y: 3},
		{X: 7, Y: 0}, {X: 7, Y: 1}, {X: 7, Y: 2},
		{X: 8, Y: 0}, {X: 8, Y: 1}, {X: 8, Y: 2},
		{X: 9, Y: 0}, {X: 9, Y: 1},
	}

	// Ensure Host is the active shooter
	mgr.mu.Lock()
	room.TurnPlayerID = pHost.ID
	mgr.mu.Unlock()

	for i, target := range guestTargets {
		if err := mgr.FireShot(pHost.ID, target); err != nil {
			t.Fatalf("host shot %d at %v failed: %v", i, target, err)
		}
		if i < len(guestTargets)-1 {
			// Switch turn back to Host by firing guest dummy shot
			guestShot := game.Coord{X: i % 10, Y: 7 + (i / 10)}
			if err := mgr.FireShot(pGuest.ID, guestShot); err != nil {
				t.Fatalf("guest dummy shot %d failed: %v", i, err)
			}
		}
	}

	if room.Phase != PhaseFinished {
		t.Fatalf("expected room to be PhaseFinished, got %v", room.Phase)
	}
	if room.EndReason != "all_sunk" {
		t.Fatalf("expected endReason all_sunk, got %v", room.EndReason)
	}

	sHost.reset()
	sGuest.reset()

	return reg, mgr, clock, pHost, pGuest, sHost, sGuest, room
}

func TestRematch_NotAllowedOnForfeitOrTimeout(t *testing.T) {
	_, mgr, _, pHost, pGuest, _, _, room := setupPlacementTest(t)

	// Forfeit via leaveMatchLocked
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("LeaveRoom forfeit failed: %v", err)
	}
	if room.Phase != PhaseFinished || room.EndReason != "forfeit" {
		t.Fatalf("expected forfeit finish, got phase=%v reason=%v", room.Phase, room.EndReason)
	}

	// Rematch must be rejected
	err := mgr.AcceptRematch(pGuest.ID)
	if !errors.Is(err, ErrRematchNotAllowed) {
		t.Errorf("expected ErrRematchNotAllowed on forfeit, got %v", err)
	}
}

func TestRematch_RejectedIfNotInFinishedPhase(t *testing.T) {
	_, mgr, _, pHost, _, _, _, _ := setupPlacementTest(t)

	// In placement phase -> accept rematch must fail
	err := mgr.AcceptRematch(pHost.ID)
	if !errors.Is(err, ErrNotInFinished) {
		t.Errorf("expected ErrNotInFinished during placement, got %v", err)
	}
}

func TestRematch_MutualAcceptanceStartsPlacement(t *testing.T) {
	_, mgr, clock, pHost, pGuest, sHost, sGuest, room := setupFinishedMatch(t)

	// 1. Host accepts rematch
	if err := mgr.AcceptRematch(pHost.ID); err != nil {
		t.Fatalf("host AcceptRematch failed: %v", err)
	}

	// Both should receive rematch.offered with by: pHost.ID
	hostOffers := filterMessages(sHost, "rematch.offered")
	guestOffers := filterMessages(sGuest, "rematch.offered")
	if len(hostOffers) != 1 || len(guestOffers) != 1 {
		t.Fatalf("expected 1 rematch.offered for both, got host=%d guest=%d", len(hostOffers), len(guestOffers))
	}
	if hostOffers[0].payload.(map[string]any)["by"] != pHost.ID {
		t.Errorf("expected offer by host, got %v", hostOffers[0].payload)
	}

	sHost.reset()
	sGuest.reset()

	// 2. Guest accepts rematch -> triggers mutual transition to PhasePlacement
	if err := mgr.AcceptRematch(pGuest.ID); err != nil {
		t.Fatalf("guest AcceptRematch failed: %v", err)
	}

	if room.Phase != PhasePlacement {
		t.Fatalf("expected room to transition to PhasePlacement, got %v", room.Phase)
	}
	if room.PlacementDeadline.IsZero() {
		t.Fatalf("expected non-zero placement deadline")
	}
	if room.PlacementTimer == nil {
		t.Fatalf("expected active placement timer")
	}
	if room.HostBoard != nil || room.GuestBoard != nil {
		t.Errorf("expected cleared boards for rematch")
	}
	if len(room.ShotLog) != 0 {
		t.Errorf("expected cleared shot log for rematch, got %d", len(room.ShotLog))
	}

	// Both players receive room.state with phase: "placement"
	hostState := filterMessages(sHost, "room.state")
	guestState := filterMessages(sGuest, "room.state")
	if len(hostState) != 1 || len(guestState) != 1 {
		t.Fatalf("expected room.state on rematch start, got host=%d guest=%d", len(hostState), len(guestState))
	}
	hView := hostState[0].payload.(RoomStateView)
	if hView.Phase != PhasePlacement {
		t.Errorf("expected host view phase placement, got %v", hView.Phase)
	}

	sHost.reset()
	sGuest.reset()

	// 3. Confirm fleets in the rematched game and verify battle starts cleanly
	if err := mgr.SubmitFleet(pHost.ID, validFleetForTest()); err != nil {
		t.Fatalf("host SubmitFleet in rematch failed: %v", err)
	}
	if err := mgr.SubmitFleet(pGuest.ID, validGuestFleetForTest()); err != nil {
		t.Fatalf("guest SubmitFleet in rematch failed: %v", err)
	}

	if room.Phase != PhaseBattle {
		t.Fatalf("expected room to transition to PhaseBattle after placements, got %v", room.Phase)
	}
	if room.TurnPlayerID != pHost.ID && room.TurnPlayerID != pGuest.ID {
		t.Errorf("expected valid turn player, got %s", room.TurnPlayerID)
	}

	_ = clock
}

func TestRematch_DeclineCancelsOffer(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, _ := setupFinishedMatch(t)

	// Host accepts rematch
	if err := mgr.AcceptRematch(pHost.ID); err != nil {
		t.Fatalf("host AcceptRematch failed: %v", err)
	}

	sHost.reset()
	sGuest.reset()

	// Guest declines rematch
	if err := mgr.DeclineRematch(pGuest.ID); err != nil {
		t.Fatalf("guest DeclineRematch failed: %v", err)
	}

	// Host and guest should receive rematch.cancelled with reason: "declined"
	hostCancelled := filterMessages(sHost, "rematch.cancelled")
	guestCancelled := filterMessages(sGuest, "rematch.cancelled")
	if len(hostCancelled) != 1 || len(guestCancelled) != 1 {
		t.Fatalf("expected rematch.cancelled on decline, got host=%d guest=%d", len(hostCancelled), len(guestCancelled))
	}
	if hostCancelled[0].payload.(map[string]any)["reason"] != "declined" {
		t.Errorf("expected reason 'declined', got %v", hostCancelled[0].payload)
	}
}

func TestRematch_LeaveRoomCancelsOffer(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, _ := setupFinishedMatch(t)
	_ = pGuest

	// Host accepts rematch
	if err := mgr.AcceptRematch(pHost.ID); err != nil {
		t.Fatalf("host AcceptRematch failed: %v", err)
	}

	sHost.reset()
	sGuest.reset()

	// Host leaves room -> returns to lobby and notifies Guest
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("host LeaveRoom failed: %v", err)
	}

	guestCancelled := filterMessages(sGuest, "rematch.cancelled")
	if len(guestCancelled) != 1 {
		t.Fatalf("expected 1 rematch.cancelled to guest on host leave, got %d", len(guestCancelled))
	}
	if guestCancelled[0].payload.(map[string]any)["reason"] != "left" {
		t.Errorf("expected reason 'left', got %v", guestCancelled[0].payload)
	}
}

func TestRematch_DisconnectCancelsOffer(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, _ := setupFinishedMatch(t)

	// Host accepts rematch
	if err := mgr.AcceptRematch(pHost.ID); err != nil {
		t.Fatalf("host AcceptRematch failed: %v", err)
	}

	sHost.reset()
	sGuest.reset()

	// Guest disconnects
	mgr.HandleDisconnect(pGuest.ID)

	hostCancelled := filterMessages(sHost, "rematch.cancelled")
	if len(hostCancelled) != 1 {
		t.Fatalf("expected 1 rematch.cancelled to host on guest disconnect, got %d", len(hostCancelled))
	}
	if hostCancelled[0].payload.(map[string]any)["reason"] != "disconnected" {
		t.Errorf("expected reason 'disconnected', got %v", hostCancelled[0].payload)
	}
}

func TestFinishedMatch_ReturnToLobbyActiveRoomsCountAndUpdate(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, room := setupFinishedMatch(t)

	sHost.reset()
	sGuest.reset()

	// 1. Host clicks Return to Lobby first
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("host LeaveRoom failed: %v", err)
	}

	// Host should receive lobby.snapshot showing 0 active rooms
	hostSnapshots := filterMessages(sHost, "lobby.snapshot")
	if len(hostSnapshots) != 1 {
		t.Fatalf("expected 1 lobby.snapshot to host, got %d", len(hostSnapshots))
	}
	snap := hostSnapshots[0].payload.(LobbySnapshotPayload)
	if snap.TotalRooms != 0 {
		t.Errorf("expected host to see 0 active rooms in lobby.snapshot, got %d", snap.TotalRooms)
	}

	// The room still exists in memory because Guest has not left yet
	if mgr.TotalRoomsInMemory() != 1 {
		t.Errorf("expected 1 room in memory while guest remains on game over screen, got %d", mgr.TotalRoomsInMemory())
	}
	// But active rooms count is 0
	if mgr.TotalRooms() != 0 {
		t.Errorf("expected 0 active rooms, got %d", mgr.TotalRooms())
	}

	sHost.reset()
	sGuest.reset()

	// 2. Guest clicks Return to Lobby second
	if err := mgr.LeaveRoom(pGuest.ID); err != nil {
		t.Fatalf("guest LeaveRoom failed: %v", err)
	}

	// Guest should receive lobby.snapshot showing 0 active rooms
	guestSnapshots := filterMessages(sGuest, "lobby.snapshot")
	if len(guestSnapshots) != 1 {
		t.Fatalf("expected 1 lobby.snapshot to guest, got %d", len(guestSnapshots))
	}
	gSnap := guestSnapshots[0].payload.(LobbySnapshotPayload)
	if gSnap.TotalRooms != 0 {
		t.Errorf("expected guest to see 0 active rooms in lobby.snapshot, got %d", gSnap.TotalRooms)
	}

	// CRITICAL: Host (already in the lobby) MUST receive lobby.update showing TotalRooms: 0!
	hostUpdates := filterMessages(sHost, "lobby.update")
	if len(hostUpdates) != 1 {
		t.Fatalf("expected host to receive 1 lobby.update when room is cleaned up, got %d", len(hostUpdates))
	}
	upd := hostUpdates[0].payload.(LobbyUpdatePayload)
	if upd.TotalRooms != 0 {
		t.Errorf("expected host lobby.update TotalRooms to be 0, got %d", upd.TotalRooms)
	}
	if upd.RoomID != room.ID {
		t.Errorf("expected lobby.update RoomID to be %s, got %s", room.ID, upd.RoomID)
	}

	// The room should now be completely deleted from memory
	if mgr.TotalRoomsInMemory() != 0 {
		t.Errorf("expected 0 rooms in memory after both left, got %d", mgr.TotalRoomsInMemory())
	}
}

func TestFinishedMatch_OpponentDisconnectCleansUpRoom(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, _, _ := setupFinishedMatch(t)

	sHost.reset()

	// 1. Host clicks Return to Lobby
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("host LeaveRoom failed: %v", err)
	}
	sHost.reset()

	// 2. Guest closes browser / disconnects
	mgr.HandleDisconnect(pGuest.ID)

	// Host should receive lobby.update confirming room deletion and 0 total rooms
	hostUpdates := filterMessages(sHost, "lobby.update")
	if len(hostUpdates) != 1 {
		t.Fatalf("expected host to receive 1 lobby.update on guest disconnect cleanup, got %d", len(hostUpdates))
	}
	upd := hostUpdates[0].payload.(LobbyUpdatePayload)
	if upd.TotalRooms != 0 {
		t.Errorf("expected TotalRooms 0, got %d", upd.TotalRooms)
	}

	// Room must be completely deleted from memory
	if mgr.TotalRoomsInMemory() != 0 {
		t.Errorf("expected 0 rooms in memory after guest disconnect, got %d", mgr.TotalRoomsInMemory())
	}
}
