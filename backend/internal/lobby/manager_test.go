package lobby

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockSender records sent messages in a thread-safe manner.
type mockSender struct {
	mu       sync.Mutex
	messages []sentMessage
}

type sentMessage struct {
	msgType string
	payload any
}

func (s *mockSender) Send(msgType string, payload any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, sentMessage{msgType: msgType, payload: payload})
	return nil
}

func (s *mockSender) getMessages() []sentMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]sentMessage, len(s.messages))
	copy(copied, s.messages)
	return copied
}

func setupLobbyTest(t *testing.T, opts ...Option) (*SessionRegistry, *LobbyManager) {
	t.Helper()
	reg := NewSessionRegistry()
	mgr := NewLobbyManager(reg, opts...)
	return reg, mgr
}

func TestCreateRoom_SuccessAndBroadcast(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	p1, err := reg.CreatePlayer("Alice")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	p2, err := reg.CreatePlayer("Bob")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	s1 := &mockSender{}
	s2 := &mockSender{}
	mgr.RegisterConnection(p1.ID, s1)
	mgr.RegisterConnection(p2.ID, s2)
	mgr.SubscribeToLobby(p1.ID)
	mgr.SubscribeToLobby(p2.ID)

	room, err := mgr.CreateRoom(p1.ID)
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	if room.Phase != PhaseWaiting {
		t.Errorf("expected phase %s, got %s", PhaseWaiting, room.Phase)
	}

	// Host should receive room.state
	p1Msgs := s1.getMessages()
	if len(p1Msgs) != 1 || p1Msgs[0].msgType != "room.state" {
		t.Errorf("host did not receive room.state: %+v", p1Msgs)
	}

	// Subscriber Bob should receive lobby.update (action: upsert)
	p2Msgs := s2.getMessages()
	if len(p2Msgs) != 1 || p2Msgs[0].msgType != "lobby.update" {
		t.Fatalf("subscriber did not receive lobby.update: %+v", p2Msgs)
	}

	update, ok := p2Msgs[0].payload.(LobbyUpdatePayload)
	if !ok || update.Action != "upsert" || update.Room.ID != room.ID || update.TotalRooms != 1 {
		t.Errorf("unexpected update payload: %+v", p2Msgs[0].payload)
	}

	// Snapshot should show 1 waiting room
	snap := mgr.GetLobbySnapshot()
	if len(snap.Rooms) != 1 || snap.Rooms[0].ID != room.ID || snap.TotalRooms != 1 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestCreateRoom_MaxRoomsCap(t *testing.T) {
	reg, mgr := setupLobbyTest(t, WithMaxRooms(3))

	for i := 1; i <= 3; i++ {
		p, _ := reg.CreatePlayer(fmt.Sprintf("Player%d", i))
		_, err := mgr.CreateRoom(p.ID)
		if err != nil {
			t.Fatalf("room %d failed to create: %v", i, err)
		}
	}

	// 4th room exceeds cap of 3
	p4, _ := reg.CreatePlayer("Player4")
	_, err := mgr.CreateRoom(p4.ID)
	if !errors.Is(err, ErrRoomCapReached) {
		t.Fatalf("expected ErrRoomCapReached, got %v", err)
	}
}

func TestCreateRoom_AlreadyInRoom(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	p, _ := reg.CreatePlayer("Host")
	_, err := mgr.CreateRoom(p.ID)
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Second attempt while still hosting
	_, err = mgr.CreateRoom(p.ID)
	if !errors.Is(err, ErrAlreadyInRoom) {
		t.Fatalf("expected ErrAlreadyInRoom, got %v", err)
	}
}

func TestCancelRoom_Success(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	host, _ := reg.CreatePlayer("Host")
	guest, _ := reg.CreatePlayer("Guest")

	hostSender := &mockSender{}
	guestSender := &mockSender{}
	mgr.RegisterConnection(host.ID, hostSender)
	mgr.RegisterConnection(guest.ID, guestSender)
	mgr.SubscribeToLobby(guest.ID)

	room, err := mgr.CreateRoom(host.ID)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	err = mgr.CancelRoom(host.ID)
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	if mgr.TotalRooms() != 0 {
		t.Errorf("expected 0 total rooms, got %d", mgr.TotalRooms())
	}

	// Guest should receive lobby.update (action: remove)
	gMsgs := guestSender.getMessages()
	foundRemove := false
	for _, m := range gMsgs {
		if m.msgType == "lobby.update" {
			if u, ok := m.payload.(LobbyUpdatePayload); ok && u.Action == "remove" && u.RoomID == room.ID {
				foundRemove = true
			}
		}
	}
	if !foundRemove {
		t.Errorf("guest did not receive remove update: %+v", gMsgs)
	}
}

func TestJoinRoom_CannotJoinOwnRoom(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	host, _ := reg.CreatePlayer("Host")
	room, _ := mgr.CreateRoom(host.ID)

	_, err := mgr.JoinRoom(host.ID, room.ID)
	if !errors.Is(err, ErrCannotJoinOwnRoom) {
		t.Fatalf("expected ErrCannotJoinOwnRoom, got %v", err)
	}
}

func TestJoinRoom_AtomicSimultaneousJoins(t *testing.T) {
	reg, mgr := setupLobbyTest(t)

	host, _ := reg.CreatePlayer("Host")
	hostSender := &mockSender{}
	mgr.RegisterConnection(host.ID, hostSender)

	room, err := mgr.CreateRoom(host.ID)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	const numJoiners = 20
	var wg sync.WaitGroup
	var successCount int64
	var notAvailableCount int64

	startBarrier := make(chan struct{})

	for i := 0; i < numJoiners; i++ {
		wg.Add(1)
		player, _ := reg.CreatePlayer(fmt.Sprintf("Joiner%d", i))
		sender := &mockSender{}
		mgr.RegisterConnection(player.ID, sender)

		go func(p *Player) {
			defer wg.Done()
			<-startBarrier
			_, joinErr := mgr.JoinRoom(p.ID, room.ID)
			if joinErr == nil {
				atomic.AddInt64(&successCount, 1)
			} else if errors.Is(joinErr, ErrRoomNotAvailable) {
				atomic.AddInt64(&notAvailableCount, 1)
			} else {
				t.Errorf("unexpected error: %v", joinErr)
			}
		}(player)
	}

	// Unleash all joiners simultaneously
	close(startBarrier)
	wg.Wait()

	if successCount != 1 {
		t.Fatalf("expected exactly 1 successful join, got %d", successCount)
	}
	if notAvailableCount != int64(numJoiners-1) {
		t.Fatalf("expected %d ROOM_NOT_AVAILABLE errors, got %d", numJoiners-1, notAvailableCount)
	}

	// Room should now be in PhaseReadyUp
	if room.Phase != PhaseReadyUp {
		t.Errorf("expected phase %s, got %s", PhaseReadyUp, room.Phase)
	}

	// Waiting rooms in snapshot should now be empty
	snap := mgr.GetLobbySnapshot()
	if len(snap.Rooms) != 0 {
		t.Errorf("expected 0 waiting rooms after join, got %d", len(snap.Rooms))
	}
}

func TestWaitingRoomHostDisconnect_GracePeriod(t *testing.T) {
	start := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	mockClock := NewMockClock(start)
	reg, mgr := setupLobbyTest(t, WithClock(mockClock), WithDisconnectGrace(60*time.Second))

	host, _ := reg.CreatePlayer("Host")
	guest, _ := reg.CreatePlayer("LobbyBrowser")

	hostSender := &mockSender{}
	guestSender := &mockSender{}
	mgr.RegisterConnection(host.ID, hostSender)
	mgr.RegisterConnection(guest.ID, guestSender)
	mgr.SubscribeToLobby(guest.ID)

	room, err := mgr.CreateRoom(host.ID)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Host disconnects
	reg.DetachConnection(host.SessionToken)
	mgr.UnregisterConnection(host.ID)
	mgr.HandleDisconnect(host.ID)

	// Advance clock by 30s (within 60s grace period)
	mockClock.Advance(30 * time.Second)
	if mgr.TotalRooms() != 1 {
		t.Errorf("room should still exist within grace period, total: %d", mgr.TotalRooms())
	}

	// Advance clock past 60s deadline
	mockClock.Advance(31 * time.Second)
	if mgr.TotalRooms() != 0 {
		t.Errorf("room should be removed after 60s grace period expired, total: %d", mgr.TotalRooms())
	}

	// Guest should receive lobby.update remove
	foundRemove := false
	for _, m := range guestSender.getMessages() {
		if m.msgType == "lobby.update" {
			if u, ok := m.payload.(LobbyUpdatePayload); ok && u.Action == "remove" && u.RoomID == room.ID {
				foundRemove = true
			}
		}
	}
	if !foundRemove {
		t.Errorf("guest did not receive remove update upon grace period expiration")
	}
}

func TestWaitingRoomHostDisconnect_ReconnectCancelsGracePeriod(t *testing.T) {
	start := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	mockClock := NewMockClock(start)
	reg, mgr := setupLobbyTest(t, WithClock(mockClock), WithDisconnectGrace(60*time.Second))

	host, _ := reg.CreatePlayer("Host")
	hostSender := &mockSender{}
	mgr.RegisterConnection(host.ID, hostSender)

	room, err := mgr.CreateRoom(host.ID)
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Disconnect
	reg.DetachConnection(host.SessionToken)
	mgr.UnregisterConnection(host.ID)
	mgr.HandleDisconnect(host.ID)

	// Advance 30s
	mockClock.Advance(30 * time.Second)

	// Reconnect
	_, err = reg.AttachConnection(host.SessionToken)
	if err != nil {
		t.Fatalf("reconnect attach failed: %v", err)
	}
	newHostSender := &mockSender{}
	mgr.RegisterConnection(host.ID, newHostSender)
	reconnectedRoom, err := mgr.HandleReconnect(host.ID)
	if err != nil || reconnectedRoom.ID != room.ID {
		t.Fatalf("reconnect handle failed: %v, room: %+v", err, reconnectedRoom)
	}

	// Advance another 60s -> should NOT expire because host reconnected
	mockClock.Advance(60 * time.Second)
	if mgr.TotalRooms() != 1 {
		t.Errorf("expected room to still be alive after reconnect, got total %d", mgr.TotalRooms())
	}
}

