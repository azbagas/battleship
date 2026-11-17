package lobby

import (
	mrand "math/rand"
	"testing"
	"time"
)

func setupBattleTest(t *testing.T) (*SessionRegistry, *LobbyManager, *MockClock, *Player, *Player, *mockSender, *mockSender, *Room) {
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

	if err := mgr.ReadyRoom(pHost.ID); err != nil {
		t.Fatalf("failed to ready host: %v", err)
	}
	if err := mgr.ReadyRoom(pGuest.ID); err != nil {
		t.Fatalf("failed to ready guest: %v", err)
	}

	fleet := validFleetForTest()
	if err := mgr.SubmitFleet(pHost.ID, fleet); err != nil {
		t.Fatalf("failed to submit host fleet: %v", err)
	}
	if err := mgr.SubmitFleet(pGuest.ID, fleet); err != nil {
		t.Fatalf("failed to submit guest fleet: %v", err)
	}

	if room.Phase != PhaseBattle {
		t.Fatalf("expected room to be PhaseBattle, got %v", room.Phase)
	}

	return reg, mgr, clock, pHost, pGuest, sHost, sGuest, room
}

func TestDisconnect_Placement_ReconnectWithin60s(t *testing.T) {
	reg, mgr, clock, pHost, pGuest, sHost, sGuest, room := setupPlacementTest(t)

	// Guest disconnects
	reg.DetachConnection(pGuest.SessionToken)
	mgr.UnregisterConnection(pGuest.ID)
	mgr.HandleDisconnect(pGuest.ID)

	// Host should receive opponent.disconnected with graceDeadline
	hostMsgs := sHost.getMessages()
	var foundDisconnect bool
	for _, m := range hostMsgs {
		if m.msgType == "opponent.disconnected" {
			foundDisconnect = true
			payload, ok := m.payload.(OpponentDisconnectedPayload)
			if !ok {
				t.Fatalf("expected OpponentDisconnectedPayload for opponent.disconnected, got %T", m.payload)
			}
			if payload.GraceDeadline.IsZero() {
				t.Fatalf("expected valid graceDeadline, got %v", payload.GraceDeadline)
			}
		}
	}
	if !foundDisconnect {
		t.Fatalf("expected host to receive opponent.disconnected event")
	}

	// Advance time by 30s (less than 60s grace)
	clock.Advance(30 * time.Second)

	// Guest reconnects
	_, _ = reg.AttachConnection(pGuest.SessionToken)
	sGuestNew := &mockSender{}
	mgr.RegisterConnection(pGuest.ID, sGuestNew)
	reconnectedRoom, err := mgr.HandleReconnect(pGuest.ID)
	if err != nil {
		t.Fatalf("unexpected err on reconnect: %v", err)
	}
	if reconnectedRoom == nil || reconnectedRoom.RoomID != room.ID {
		t.Fatalf("expected reconnected room to match, got %v", reconnectedRoom)
	}

	// Host should receive opponent.reconnected
	hostMsgsAfter := sHost.getMessages()
	var foundReconnect bool
	for _, m := range hostMsgsAfter {
		if m.msgType == "opponent.reconnected" {
			foundReconnect = true
		}
	}
	if !foundReconnect {
		t.Fatalf("expected host to receive opponent.reconnected event")
	}

	// Both can complete fleet submission
	fleet := validFleetForTest()
	if err := mgr.SubmitFleet(pHost.ID, fleet); err != nil {
		t.Fatalf("unexpected err submitting host fleet: %v", err)
	}
	if err := mgr.SubmitFleet(pGuest.ID, fleet); err != nil {
		t.Fatalf("unexpected err submitting guest fleet: %v", err)
	}

	if room.Phase != PhaseBattle {
		t.Fatalf("expected room to transition to PhaseBattle, got %v", room.Phase)
	}
	_ = sGuest
}

func TestDisconnect_Placement_TimeoutForfeit(t *testing.T) {
	reg, mgr, clock, pHost, pGuest, sHost, _, room := setupPlacementTest(t)

	// Guest disconnects
	reg.DetachConnection(pGuest.SessionToken)
	mgr.UnregisterConnection(pGuest.ID)
	mgr.HandleDisconnect(pGuest.ID)

	// Advance time past 60s grace timer
	clock.Advance(61 * time.Second)

	if room.Phase != PhaseFinished {
		t.Fatalf("expected room to be PhaseFinished, got %v", room.Phase)
	}
	if room.WinnerPlayerID != pHost.ID {
		t.Fatalf("expected winner to be host (%v), got %v", pHost.ID, room.WinnerPlayerID)
	}
	if room.EndReason != "disconnect_timeout" {
		t.Fatalf("expected endReason disconnect_timeout, got %v", room.EndReason)
	}

	// Host should receive game.over
	hostMsgs := sHost.getMessages()
	var foundGameOver bool
	for _, m := range hostMsgs {
		if m.msgType == "game.over" {
			foundGameOver = true
			payload := m.payload.(GameOverPayload)
			if payload.WinnerPlayerID != pHost.ID {
				t.Errorf("expected winner %v, got %v", pHost.ID, payload.WinnerPlayerID)
			}
			if payload.Reason != "disconnect_timeout" {
				t.Errorf("expected reason disconnect_timeout, got %v", payload.Reason)
			}
		}
	}
	if !foundGameOver {
		t.Fatalf("expected host to receive game.over message")
	}
}

func TestDisconnect_Battle_ReconnectWithin60s(t *testing.T) {
	reg, mgr, clock, pHost, pGuest, sHost, _, room := setupBattleTest(t)
	_ = room

	// Host disconnects
	reg.DetachConnection(pHost.SessionToken)
	mgr.UnregisterConnection(pHost.ID)
	mgr.HandleDisconnect(pHost.ID)

	// Advance clock by 25s
	clock.Advance(25 * time.Second)

	// Host reconnects
	_, _ = reg.AttachConnection(pHost.SessionToken)
	sHostNew := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHostNew)
	reconnectedRoom, err := mgr.HandleReconnect(pHost.ID)
	if err != nil {
		t.Fatalf("unexpected err on reconnect: %v", err)
	}
	if reconnectedRoom == nil || reconnectedRoom.Phase != PhaseBattle {
		t.Fatalf("expected reconnected battle room, got %v", reconnectedRoom)
	}

	// State view sent to host should include valid battle view
	hostMsgs := sHostNew.getMessages()
	var foundState bool
	for _, m := range hostMsgs {
		if m.msgType == "room.state" {
			foundState = true
			view, ok := m.payload.(RoomStateView)
			if !ok {
				t.Fatalf("expected RoomStateView, got %T", m.payload)
			}
			if view.Phase != PhaseBattle || view.BattleView == nil {
				t.Fatalf("expected active battle view in restored room.state")
			}
		}
	}
	if !foundState {
		t.Fatalf("expected host to receive room.state upon reconnect")
	}
	_ = sHost
	_ = pGuest
}

func TestDisconnect_Battle_TimeoutForfeit(t *testing.T) {
	reg, mgr, clock, pHost, pGuest, _, sGuest, room := setupBattleTest(t)

	// Host disconnects
	reg.DetachConnection(pHost.SessionToken)
	mgr.UnregisterConnection(pHost.ID)
	mgr.HandleDisconnect(pHost.ID)

	// 60s grace expires
	clock.Advance(61 * time.Second)

	if room.Phase != PhaseFinished {
		t.Fatalf("expected room to be PhaseFinished, got %v", room.Phase)
	}
	if room.WinnerPlayerID != pGuest.ID {
		t.Fatalf("expected guest to win, got %v", room.WinnerPlayerID)
	}
	if room.EndReason != "disconnect_timeout" {
		t.Fatalf("expected disconnect_timeout, got %v", room.EndReason)
	}

	// Guest should receive game.over
	guestMsgs := sGuest.getMessages()
	var foundGameOver bool
	for _, m := range guestMsgs {
		if m.msgType == "game.over" {
			foundGameOver = true
			payload := m.payload.(GameOverPayload)
			if payload.WinnerPlayerID != pGuest.ID {
				t.Errorf("expected winner %v, got %v", pGuest.ID, payload.WinnerPlayerID)
			}
			if payload.Reason != "disconnect_timeout" {
				t.Errorf("expected reason disconnect_timeout, got %v", payload.Reason)
			}
		}
	}
	if !foundGameOver {
		t.Fatalf("expected guest to receive game.over message")
	}
}

func TestDisconnect_BothPlayersDisconnect_Abandoned(t *testing.T) {
	reg, mgr, clock, pHost, pGuest, _, _, room := setupBattleTest(t)

	// Host disconnects at t=0
	reg.DetachConnection(pHost.SessionToken)
	mgr.UnregisterConnection(pHost.ID)
	mgr.HandleDisconnect(pHost.ID)

	// Guest disconnects at t=10s
	clock.Advance(10 * time.Second)
	reg.DetachConnection(pGuest.SessionToken)
	mgr.UnregisterConnection(pGuest.ID)
	mgr.HandleDisconnect(pGuest.ID)

	// Advance past host's 60s timer (total t=61s)
	clock.Advance(51 * time.Second)

	if room.Phase != PhaseFinished {
		t.Fatalf("expected room to be PhaseFinished, got %v", room.Phase)
	}
	if room.WinnerPlayerID != "" {
		t.Fatalf("expected empty winner for abandoned match, got %v", room.WinnerPlayerID)
	}
	if room.EndReason != "abandoned" {
		t.Fatalf("expected endReason abandoned, got %v", room.EndReason)
	}
}

func TestExplicitForfeit_Placement(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, room := setupPlacementTest(t)

	// Host explicitly leaves / forfeits during placement
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("unexpected err on leave: %v", err)
	}

	if room.Phase != PhaseFinished {
		t.Fatalf("expected room to be PhaseFinished, got %v", room.Phase)
	}
	if room.WinnerPlayerID != pGuest.ID {
		t.Fatalf("expected guest to win by forfeit, got %v", room.WinnerPlayerID)
	}
	if room.EndReason != "forfeit" {
		t.Fatalf("expected endReason forfeit, got %v", room.EndReason)
	}

	// Host should receive lobby.snapshot
	hostMsgs := sHost.getMessages()
	var foundSnapshot bool
	for _, m := range hostMsgs {
		if m.msgType == "lobby.snapshot" {
			foundSnapshot = true
		}
	}
	if !foundSnapshot {
		t.Fatalf("expected leaving host to receive lobby.snapshot")
	}

	// Guest should receive game.over with reason forfeit
	guestMsgs := sGuest.getMessages()
	var foundGameOver bool
	for _, m := range guestMsgs {
		if m.msgType == "game.over" {
			foundGameOver = true
			payload := m.payload.(GameOverPayload)
			if payload.WinnerPlayerID != pGuest.ID {
				t.Errorf("expected winner %v, got %v", pGuest.ID, payload.WinnerPlayerID)
			}
			if payload.Reason != "forfeit" {
				t.Errorf("expected reason forfeit, got %v", payload.Reason)
			}
		}
	}
	if !foundGameOver {
		t.Fatalf("expected guest to receive game.over forfeit")
	}
}

func TestExplicitForfeit_Battle(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, room := setupBattleTest(t)

	// Guest explicitly leaves / forfeits during battle
	if err := mgr.LeaveRoom(pGuest.ID); err != nil {
		t.Fatalf("unexpected err on leave: %v", err)
	}

	if room.Phase != PhaseFinished {
		t.Fatalf("expected room to be PhaseFinished, got %v", room.Phase)
	}
	if room.WinnerPlayerID != pHost.ID {
		t.Fatalf("expected host to win by forfeit, got %v", room.WinnerPlayerID)
	}
	if room.EndReason != "forfeit" {
		t.Fatalf("expected endReason forfeit, got %v", room.EndReason)
	}

	// Guest should receive lobby.snapshot
	guestMsgs := sGuest.getMessages()
	var foundSnapshot bool
	for _, m := range guestMsgs {
		if m.msgType == "lobby.snapshot" {
			foundSnapshot = true
		}
	}
	if !foundSnapshot {
		t.Fatalf("expected leaving guest to receive lobby.snapshot")
	}

	// Host should receive game.over with reason forfeit
	hostMsgs := sHost.getMessages()
	var foundGameOver bool
	for _, m := range hostMsgs {
		if m.msgType == "game.over" {
			foundGameOver = true
			payload := m.payload.(GameOverPayload)
			if payload.WinnerPlayerID != pHost.ID {
				t.Errorf("expected winner %v, got %v", pHost.ID, payload.WinnerPlayerID)
			}
			if payload.Reason != "forfeit" {
				t.Errorf("expected reason forfeit, got %v", payload.Reason)
			}
		}
	}
	if !foundGameOver {
		t.Fatalf("expected host to receive game.over forfeit")
	}
}

func TestLeaveRoom_FinishedPhase(t *testing.T) {
	_, mgr, _, pHost, pGuest, _, _, room := setupBattleTest(t)

	// Forfeit to finish room
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if room.Phase != PhaseFinished {
		t.Fatalf("expected PhaseFinished, got %v", room.Phase)
	}

	// Now guest also leaves the finished room
	if err := mgr.LeaveRoom(pGuest.ID); err != nil {
		t.Fatalf("unexpected err when guest leaves finished room: %v", err)
	}

	// Since both players left, room should be deleted
	if mgr.TotalRooms() != 0 {
		t.Fatalf("expected 0 rooms remaining, got %d", mgr.TotalRooms())
	}
}
