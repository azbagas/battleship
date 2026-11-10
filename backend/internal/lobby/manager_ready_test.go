package lobby

import (
	"testing"
	"time"
)

func TestReadyUp_ToggleReadyAndPlacementTransition(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	pHost, err := reg.CreatePlayer("Host")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	pGuest, err := reg.CreatePlayer("Guest")
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

	if room.Phase != PhaseReadyUp {
		t.Fatalf("expected phase %s, got %s", PhaseReadyUp, room.Phase)
	}
	if room.HostReady || room.GuestReady {
		t.Fatalf("expected both players unready initially")
	}

	// Guest readies up
	if err := mgr.ReadyRoom(pGuest.ID); err != nil {
		t.Fatalf("failed to ready guest: %v", err)
	}
	if !room.GuestReady || room.HostReady {
		t.Errorf("expected guest ready and host unready, got hostReady=%v, guestReady=%v", room.HostReady, room.GuestReady)
	}
	if room.Phase != PhaseReadyUp {
		t.Errorf("room should remain in ready-up while only one player is ready")
	}

	// Guest unreadies
	if err := mgr.UnreadyRoom(pGuest.ID); err != nil {
		t.Fatalf("failed to unready guest: %v", err)
	}
	if room.GuestReady {
		t.Errorf("expected guest unready")
	}

	// Host readies up
	if err := mgr.ReadyRoom(pHost.ID); err != nil {
		t.Fatalf("failed to ready host: %v", err)
	}
	if !room.HostReady || room.GuestReady {
		t.Errorf("expected host ready and guest unready")
	}

	// Guest readies up -> both ready -> transition to placement
	if err := mgr.ReadyRoom(pGuest.ID); err != nil {
		t.Fatalf("failed to ready guest: %v", err)
	}

	if room.Phase != PhasePlacement {
		t.Fatalf("expected phase %s, got %s", PhasePlacement, room.Phase)
	}

	// Verify host received room.state with placement phase
	hostMsgs := sHost.getMessages()
	foundPlacement := false
	for _, m := range hostMsgs {
		if m.msgType == "room.state" {
			if view, ok := m.payload.(RoomStateView); ok && view.Phase == PhasePlacement {
				foundPlacement = true
				if !view.HostReady || !view.GuestReady {
					t.Errorf("expected both ready in placement view")
				}
			}
		}
	}
	if !foundPlacement {
		t.Errorf("host did not receive placement room.state: %+v", hostMsgs)
	}
}

func TestReadyUp_LeaveGuest(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	pHost, _ := reg.CreatePlayer("Host")
	pGuest, _ := reg.CreatePlayer("Guest")
	pLobby, _ := reg.CreatePlayer("LobbyWatcher")

	sHost := &mockSender{}
	sGuest := &mockSender{}
	sLobby := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHost)
	mgr.RegisterConnection(pGuest.ID, sGuest)
	mgr.RegisterConnection(pLobby.ID, sLobby)
	mgr.SubscribeToLobby(pLobby.ID)

	room, _ := mgr.CreateRoom(pHost.ID)
	_, _ = mgr.JoinRoom(pGuest.ID, room.ID)

	// Guest leaves the ready-up phase
	if err := mgr.LeaveRoom(pGuest.ID); err != nil {
		t.Fatalf("failed to leave room: %v", err)
	}

	// Room should revert to waiting
	if room.Phase != PhaseWaiting {
		t.Errorf("expected room phase %s, got %s", PhaseWaiting, room.Phase)
	}
	if room.GuestPlayerID != "" {
		t.Errorf("expected guest player to be cleared, got %s", room.GuestPlayerID)
	}
	if room.HostReady || room.GuestReady {
		t.Errorf("expected ready flags reset")
	}

	// Guest's active room in registry must be cleared
	guestRoom, _ := reg.GetPlayerRoom(pGuest.ID)
	if guestRoom != "" {
		t.Errorf("expected guest room cleared, got %s", guestRoom)
	}

	// Host should receive waiting room.state
	hostMsgs := sHost.getMessages()
	lastHostMsg := hostMsgs[len(hostMsgs)-1]
	if lastHostMsg.msgType != "room.state" {
		t.Errorf("expected host to receive room.state, got %s", lastHostMsg.msgType)
	}
	if view, ok := lastHostMsg.payload.(RoomStateView); !ok || view.Phase != PhaseWaiting {
		t.Errorf("expected waiting room view for host, got %+v", lastHostMsg.payload)
	}

	// LobbyWatcher should receive lobby.update upsert
	lobbyMsgs := sLobby.getMessages()
	foundUpsert := false
	for _, m := range lobbyMsgs {
		if m.msgType == "lobby.update" {
			if up, ok := m.payload.(LobbyUpdatePayload); ok && up.Action == "upsert" && up.Room.ID == room.ID {
				foundUpsert = true
			}
		}
	}
	if !foundUpsert {
		t.Errorf("lobby watcher did not receive upsert for returned waiting room: %+v", lobbyMsgs)
	}
}

func TestReadyUp_LeaveHost(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	pHost, _ := reg.CreatePlayer("Host")
	pGuest, _ := reg.CreatePlayer("Guest")

	sHost := &mockSender{}
	sGuest := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHost)
	mgr.RegisterConnection(pGuest.ID, sGuest)

	room, _ := mgr.CreateRoom(pHost.ID)
	_, _ = mgr.JoinRoom(pGuest.ID, room.ID)

	// Host leaves the ready-up phase
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("failed to leave room as host: %v", err)
	}

	if mgr.TotalRooms() != 0 {
		t.Errorf("expected 0 rooms remaining, got %d", mgr.TotalRooms())
	}

	hostRoom, _ := reg.GetPlayerRoom(pHost.ID)
	if hostRoom != "" {
		t.Errorf("expected host room cleared")
	}
	guestRoom, _ := reg.GetPlayerRoom(pGuest.ID)
	if guestRoom != "" {
		t.Errorf("expected guest room cleared")
	}

	// Both should receive lobby.snapshot
	guestMsgs := sGuest.getMessages()
	lastGuestMsg := guestMsgs[len(guestMsgs)-1]
	if lastGuestMsg.msgType != "lobby.snapshot" {
		t.Errorf("expected guest to receive lobby.snapshot, got %s", lastGuestMsg.msgType)
	}
}

func TestReadyUp_GuestDisconnectTimeout(t *testing.T) {
	clock := NewMockClock(time.Now())
	reg := NewSessionRegistry()
	mgr := NewLobbyManager(reg, WithClock(clock), WithDisconnectGrace(60*time.Second))

	pHost, _ := reg.CreatePlayer("Host")
	pGuest, _ := reg.CreatePlayer("Guest")

	sHost := &mockSender{}
	sGuest := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHost)
	mgr.RegisterConnection(pGuest.ID, sGuest)

	room, _ := mgr.CreateRoom(pHost.ID)
	_, _ = mgr.JoinRoom(pGuest.ID, room.ID)

	// Guest disconnects
	reg.DetachConnection(pGuest.SessionToken)
	mgr.HandleDisconnect(pGuest.ID)

	// Advance clock past 60s grace
	clock.Advance(61 * time.Second)

	// Room should return to waiting for host
	if room.Phase != PhaseWaiting {
		t.Errorf("expected room to return to %s, got %s", PhaseWaiting, room.Phase)
	}
	if room.GuestPlayerID != "" {
		t.Errorf("expected guest cleared")
	}
}

func TestReadyUp_HostDisconnectTimeout(t *testing.T) {
	clock := NewMockClock(time.Now())
	reg := NewSessionRegistry()
	mgr := NewLobbyManager(reg, WithClock(clock), WithDisconnectGrace(60*time.Second))

	pHost, _ := reg.CreatePlayer("Host")
	pGuest, _ := reg.CreatePlayer("Guest")

	sHost := &mockSender{}
	sGuest := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHost)
	mgr.RegisterConnection(pGuest.ID, sGuest)

	room, _ := mgr.CreateRoom(pHost.ID)
	_, _ = mgr.JoinRoom(pGuest.ID, room.ID)

	// Host disconnects
	reg.DetachConnection(pHost.SessionToken)
	mgr.HandleDisconnect(pHost.ID)

	// Advance clock past 60s grace
	clock.Advance(61 * time.Second)

	// Room should be closed
	if mgr.TotalRooms() != 0 {
		t.Errorf("expected room destroyed, got totalRooms=%d", mgr.TotalRooms())
	}
}

func TestReadyUp_ReconnectWithinGrace(t *testing.T) {
	clock := NewMockClock(time.Now())
	reg := NewSessionRegistry()
	mgr := NewLobbyManager(reg, WithClock(clock), WithDisconnectGrace(60*time.Second))

	pHost, _ := reg.CreatePlayer("Host")
	pGuest, _ := reg.CreatePlayer("Guest")

	sHost := &mockSender{}
	sGuest := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHost)
	mgr.RegisterConnection(pGuest.ID, sGuest)

	room, _ := mgr.CreateRoom(pHost.ID)
	_, _ = mgr.JoinRoom(pGuest.ID, room.ID)

	// Guest disconnects
	reg.DetachConnection(pGuest.SessionToken)
	mgr.HandleDisconnect(pGuest.ID)

	// 30 seconds pass
	clock.Advance(30 * time.Second)

	// Guest reconnects
	_, _ = reg.AttachConnection(pGuest.SessionToken)
	_, err := mgr.HandleReconnect(pGuest.ID)
	if err != nil {
		t.Fatalf("failed to reconnect: %v", err)
	}

	// Advance another 40 seconds (total 70 seconds)
	clock.Advance(40 * time.Second)

	// Room should still be in ready-up
	if room.Phase != PhaseReadyUp {
		t.Errorf("expected room to still be in %s, got %s", PhaseReadyUp, room.Phase)
	}
	if room.GuestPlayerID != pGuest.ID {
		t.Errorf("expected guest to still be present")
	}
}

func TestReadyUp_InvalidActions(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	p1, _ := reg.CreatePlayer("P1")

	// Not in room
	if err := mgr.ReadyRoom(p1.ID); err != ErrNotInRoom {
		t.Errorf("expected ErrNotInRoom, got %v", err)
	}
	if err := mgr.UnreadyRoom(p1.ID); err != ErrNotInRoom {
		t.Errorf("expected ErrNotInRoom, got %v", err)
	}

	// In waiting room
	_, _ = mgr.CreateRoom(p1.ID)
	if err := mgr.ReadyRoom(p1.ID); err != ErrNotInReadyUp {
		t.Errorf("expected ErrNotInReadyUp, got %v", err)
	}
	if err := mgr.UnreadyRoom(p1.ID); err != ErrNotInReadyUp {
		t.Errorf("expected ErrNotInReadyUp, got %v", err)
	}
}
