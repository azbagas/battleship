package lobby

import (
	"errors"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
)

func validFleetForTest() []game.PlacedShip {
	return []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 0, Y: 0}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipBattleship, Origin: game.Coord{X: 0, Y: 1}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipCruiser, Origin: game.Coord{X: 0, Y: 2}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 0, Y: 3}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 0, Y: 4}, Orientation: game.OrientationHorizontal},
	}
}

func setupPlacementTest(t *testing.T) (*SessionRegistry, *LobbyManager, *MockClock, *Player, *Player, *mockSender, *mockSender, *Room) {
	t.Helper()
	clock := NewMockClock(time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC))
	rng := mrand.New(mrand.NewSource(42))
	reg, mgr := setupLobbyTest(t, WithClock(clock), WithRNG(rng))

	pHost, err := reg.CreatePlayer("Admiral_Host")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	pGuest, err := reg.CreatePlayer("Captain_Guest")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	sHost := &mockSender{}
	sGuest := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHost)
	mgr.RegisterConnection(pGuest.ID, sGuest)

	room, err := mgr.CreateRoom(pHost.ID)
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	_, err = mgr.JoinRoom(pGuest.ID, room.ID)
	if err != nil {
		t.Fatalf("failed to join room: %v", err)
	}

	// Ready both players to advance to placement phase
	if err := mgr.ReadyRoom(pHost.ID); err != nil {
		t.Fatalf("failed to ready host: %v", err)
	}
	if err := mgr.ReadyRoom(pGuest.ID); err != nil {
		t.Fatalf("failed to ready guest: %v", err)
	}

	if room.Phase != PhasePlacement {
		t.Fatalf("expected room to transition to %s, got %s", PhasePlacement, room.Phase)
	}

	return reg, mgr, clock, pHost, pGuest, sHost, sGuest, room
}

func TestPlacement_TransitionAndDeadlines(t *testing.T) {
	_, _, clock, _, _, sHost, sGuest, room := setupPlacementTest(t)

	expectedDeadline := clock.Now().Add(60 * time.Second)
	if !room.PlacementDeadline.Equal(expectedDeadline) {
		t.Fatalf("expected placement deadline %v, got %v", expectedDeadline, room.PlacementDeadline)
	}

	// Verify both players received room.state with phase: placement and placementDeadline
	hostMsgs := sHost.getMessages()
	var hostPlacementState *RoomStateView
	for _, m := range hostMsgs {
		if m.msgType == "room.state" {
			if st, ok := m.payload.(RoomStateView); ok && st.Phase == PhasePlacement {
				hostPlacementState = &st
			}
		}
	}
	if hostPlacementState == nil || hostPlacementState.PlacementDeadline == nil {
		t.Fatalf("host did not receive placement room.state with deadline")
	}

	guestMsgs := sGuest.getMessages()
	var guestPlacementState *RoomStateView
	for _, m := range guestMsgs {
		if m.msgType == "room.state" {
			if st, ok := m.payload.(RoomStateView); ok && st.Phase == PhasePlacement {
				guestPlacementState = &st
			}
		}
	}
	if guestPlacementState == nil || guestPlacementState.PlacementDeadline == nil {
		t.Fatalf("guest did not receive placement room.state with deadline")
	}
}

func TestPlacement_SubmitValidFleetAndOpponentReadyNotification(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, room := setupPlacementTest(t)

	fleetHost := validFleetForTest()

	// Host submits valid fleet
	if err := mgr.SubmitFleet(pHost.ID, fleetHost); err != nil {
		t.Fatalf("failed to submit host fleet: %v", err)
	}

	if !room.HostFleetConfirmed {
		t.Fatal("expected host fleet to be confirmed")
	}
	if room.GuestFleetConfirmed {
		t.Fatal("guest fleet should not be confirmed yet")
	}
	if room.Phase != PhasePlacement {
		t.Fatalf("room should remain in placement until both confirm, got %s", room.Phase)
	}

	// Verify host received placement.confirmed with their fleet
	hostConfirmedFound := false
	for _, m := range sHost.getMessages() {
		if m.msgType == "placement.confirmed" {
			hostConfirmedFound = true
		}
	}
	if !hostConfirmedFound {
		t.Fatal("host did not receive placement.confirmed")
	}

	// Verify guest received placement.opponentReady (and NO coordinates leaked)
	guestOpponentReadyFound := false
	for _, m := range sGuest.getMessages() {
		if m.msgType == "placement.opponentReady" {
			guestOpponentReadyFound = true
			payloadMap, ok := m.payload.(map[string]any)
			if !ok {
				t.Fatalf("unexpected opponentReady payload type: %T", m.payload)
			}
			if _, exists := payloadMap["ships"]; exists {
				t.Fatal("SECURITY LEAK: placement.opponentReady leaked ship coordinates!")
			}
		}
	}
	if !guestOpponentReadyFound {
		t.Fatal("guest did not receive placement.opponentReady")
	}

	// Host submitting again is rejected
	if err := mgr.SubmitFleet(pHost.ID, fleetHost); !errors.Is(err, ErrFleetAlreadyLocked) {
		t.Fatalf("expected ErrFleetAlreadyLocked, got %v", err)
	}

	// Guest submits valid fleet -> transitions to PhaseBattle
	fleetGuest := validFleetForTest()
	// Change positions slightly to make it distinct
	fleetGuest[0].Origin = game.Coord{X: 5, Y: 0}
	fleetGuest[1].Origin = game.Coord{X: 5, Y: 1}
	fleetGuest[2].Origin = game.Coord{X: 5, Y: 2}
	fleetGuest[3].Origin = game.Coord{X: 5, Y: 3}
	fleetGuest[4].Origin = game.Coord{X: 5, Y: 4}

	if err := mgr.SubmitFleet(pGuest.ID, fleetGuest); err != nil {
		t.Fatalf("failed to submit guest fleet: %v", err)
	}

	if !room.GuestFleetConfirmed {
		t.Fatal("expected guest fleet to be confirmed")
	}
	if room.Phase != PhaseBattle {
		t.Fatalf("expected room to transition to %s, got %s", PhaseBattle, room.Phase)
	}
}

func TestPlacement_RejectsInvalidFleet(t *testing.T) {
	_, mgr, _, pHost, _, _, _, room := setupPlacementTest(t)

	// Incomplete fleet (4 ships)
	shortFleet := []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 0, Y: 0}, Orientation: game.OrientationHorizontal},
	}
	if err := mgr.SubmitFleet(pHost.ID, shortFleet); !errors.Is(err, ErrInvalidPlacement) {
		t.Fatalf("expected ErrInvalidPlacement for short fleet, got %v", err)
	}

	// Overlapping fleet
	overlappingFleet := validFleetForTest()
	overlappingFleet[1].Origin = game.Coord{X: 0, Y: 0} // Overlaps Carrier at (0,0)
	if err := mgr.SubmitFleet(pHost.ID, overlappingFleet); !errors.Is(err, ErrInvalidPlacement) {
		t.Fatalf("expected ErrInvalidPlacement for overlapping fleet, got %v", err)
	}

	if room.HostFleetConfirmed {
		t.Fatal("invalid placement must not confirm host fleet")
	}
}

func TestPlacement_60SecondTimeoutAutoPlacement(t *testing.T) {
	_, _, clock, _, _, sHost, sGuest, room := setupPlacementTest(t)

	// Host places 2 ships manually before timeout
	room.HostFleet = []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 2, Y: 2}, Orientation: game.OrientationHorizontal},
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 7, Y: 8}, Orientation: game.OrientationVertical},
	}
	// Host has not confirmed yet!
	// Guest has placed 0 ships.

	// Advance clock by 60 seconds to trigger timeout
	clock.Advance(60 * time.Second)

	if room.Phase != PhaseBattle {
		t.Fatalf("expected room to transition to %s after 60s timeout, got %s", PhaseBattle, room.Phase)
	}
	if !room.HostFleetConfirmed || !room.GuestFleetConfirmed {
		t.Fatal("both fleets must be confirmed after 60s timeout")
	}

	// Validate Host fleet: must be 5 ships, valid, and preserve the 2 manually placed ships
	if err := game.ValidateFleet(room.HostFleet); err != nil {
		t.Fatalf("auto-placed host fleet is invalid: %v", err)
	}
	carrierFound := false
	destroyerFound := false
	for _, s := range room.HostFleet {
		if s.Type == game.ShipCarrier && s.Origin == (game.Coord{X: 2, Y: 2}) && s.Orientation == game.OrientationHorizontal {
			carrierFound = true
		}
		if s.Type == game.ShipDestroyer && s.Origin == (game.Coord{X: 7, Y: 8}) && s.Orientation == game.OrientationVertical {
			destroyerFound = true
		}
	}
	if !carrierFound || !destroyerFound {
		t.Fatal("host pre-placed ships were not preserved during timeout auto-placement")
	}

	// Validate Guest fleet: must be 5 valid ships
	if err := game.ValidateFleet(room.GuestFleet); err != nil {
		t.Fatalf("auto-placed guest fleet is invalid: %v", err)
	}

	// Verify both players received placement.confirmed with auto: true
	hostAutoConfirmed := false
	for _, m := range sHost.getMessages() {
		if m.msgType == "placement.confirmed" {
			if pl, ok := m.payload.(map[string]any); ok && pl["auto"] == true {
				hostAutoConfirmed = true
			}
		}
	}
	if !hostAutoConfirmed {
		t.Fatal("host did not receive placement.confirmed with auto=true")
	}

	guestAutoConfirmed := false
	for _, m := range sGuest.getMessages() {
		if m.msgType == "placement.confirmed" {
			if pl, ok := m.payload.(map[string]any); ok && pl["auto"] == true {
				guestAutoConfirmed = true
			}
		}
	}
	if !guestAutoConfirmed {
		t.Fatal("guest did not receive placement.confirmed with auto=true")
	}
}

func TestPlacement_ReconnectRestoresPlacementStateWithoutLeakingOpponent(t *testing.T) {
	reg, mgr, clock, pHost, pGuest, _, _, _ := setupPlacementTest(t)

	// Host submits fleet
	fleetHost := validFleetForTest()
	_ = mgr.SubmitFleet(pHost.ID, fleetHost)

	// Guest disconnects
	mgr.HandleDisconnect(pGuest.ID)

	// 20 seconds pass
	clock.Advance(20 * time.Second)

	// Guest reconnects
	_, _ = reg.AttachConnection(pGuest.SessionToken)
	sGuestNew := &mockSender{}
	mgr.RegisterConnection(pGuest.ID, sGuestNew)

	reconnectedRoom, err := mgr.HandleReconnect(pGuest.ID)
	if err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	if reconnectedRoom.Phase != PhasePlacement {
		t.Fatalf("expected room in %s, got %s", PhasePlacement, reconnectedRoom.Phase)
	}

	// Verify guest receives room.state with correct placement fields
	var restoredState *RoomStateView
	for _, m := range sGuestNew.getMessages() {
		if m.msgType == "room.state" {
			if st, ok := m.payload.(RoomStateView); ok {
				restoredState = &st
			}
		}
	}
	if restoredState == nil {
		t.Fatal("reconnected guest did not receive room.state")
	}
	if restoredState.PlacementDeadline == nil {
		t.Fatal("reconnected guest missing placement deadline")
	}
	if !restoredState.OpponentConfirmed {
		t.Fatal("reconnected guest should see OpponentConfirmed=true")
	}
	if restoredState.FleetConfirmed {
		t.Fatal("reconnected guest has not confirmed yet")
	}
	if len(restoredState.OwnFleet) != 0 {
		t.Fatal("reconnected guest should have 0 confirmed ships in OwnFleet")
	}
}
