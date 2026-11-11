package lobby

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
	"github.com/azbagas/battleship/backend/internal/store"
)

type testMockStore struct {
	mu    sync.Mutex
	saved []*store.GameRecord
}

func (m *testMockStore) SaveGame(ctx context.Context, record *store.GameRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saved = append(m.saved, record)
	return nil
}

func (m *testMockStore) GetGame(ctx context.Context, id string) (*store.GameRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.saved {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *testMockStore) Close() error {
	return nil
}

func (s *mockSender) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}

func filterMessages(s *mockSender, msgType string) []sentMessage {
	all := s.getMessages()
	var matched []sentMessage
	for _, m := range all {
		if m.msgType == msgType {
			matched = append(matched, m)
		}
	}
	return matched
}

func validGuestFleetForTest() []game.PlacedShip {
	return []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 5, Y: 0}, Orientation: game.OrientationVertical},    // (5,0)..(5,4)
		{Type: game.ShipBattleship, Origin: game.Coord{X: 6, Y: 0}, Orientation: game.OrientationVertical}, // (6,0)..(6,3)
		{Type: game.ShipCruiser, Origin: game.Coord{X: 7, Y: 0}, Orientation: game.OrientationVertical},    // (7,0)..(7,2)
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 8, Y: 0}, Orientation: game.OrientationVertical},  // (8,0)..(8,2)
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 9, Y: 0}, Orientation: game.OrientationVertical},  // (9,0)..(9,1)
	}
}

func TestBattle_TransitionAndStart(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, room := setupPlacementTest(t)

	sHost.reset()
	sGuest.reset()

	fleetHost := validFleetForTest()
	fleetGuest := validGuestFleetForTest()

	// Host submits fleet
	err := mgr.SubmitFleet(pHost.ID, fleetHost)
	if err != nil {
		t.Fatalf("host SubmitFleet failed: %v", err)
	}

	// Battle has not started yet
	if len(filterMessages(sHost, "battle.start")) != 0 {
		t.Fatalf("battle.start broadcast prematurely before guest confirmed")
	}

	// Guest submits fleet -> transition to battle!
	err = mgr.SubmitFleet(pGuest.ID, fleetGuest)
	if err != nil {
		t.Fatalf("guest SubmitFleet failed: %v", err)
	}

	if room.Phase != PhaseBattle {
		t.Fatalf("expected room phase %s, got %s", PhaseBattle, room.Phase)
	}

	// Both should receive battle.start and battle.turn
	hostStarts := filterMessages(sHost, "battle.start")
	guestStarts := filterMessages(sGuest, "battle.start")
	if len(hostStarts) != 1 || len(guestStarts) != 1 {
		t.Fatalf("expected 1 battle.start for each, got host=%d, guest=%d", len(hostStarts), len(guestStarts))
	}

	hostTurns := filterMessages(sHost, "battle.turn")
	guestTurns := filterMessages(sGuest, "battle.turn")
	if len(hostTurns) != 1 || len(guestTurns) != 1 {
		t.Fatalf("expected 1 battle.turn for each, got host=%d, guest=%d", len(hostTurns), len(guestTurns))
	}

	turnPayload, ok := hostTurns[0].payload.(map[string]any)
	if !ok {
		t.Fatalf("invalid turn payload structure")
	}
	turnPlayerID := turnPayload["turnPlayerId"].(string)
	if turnPlayerID != pHost.ID && turnPlayerID != pGuest.ID {
		t.Errorf("turnPlayerId %q is neither host nor guest", turnPlayerID)
	}

	// Room state view must have phase=battle and battleView populated
	roomStates := filterMessages(sHost, "room.state")
	if len(roomStates) == 0 {
		t.Fatalf("expected room.state update on battle start")
	}
	latestState := roomStates[len(roomStates)-1].payload.(RoomStateView)
	if latestState.Phase != PhaseBattle {
		t.Errorf("expected room phase battle, got %q", latestState.Phase)
	}
	if latestState.BattleView == nil {
		t.Fatalf("expected BattleView in room.state during battle")
	}
	if len(latestState.BattleView.OwnBoard.Ships) != 5 {
		t.Errorf("expected 5 own ships in battle view, got %d", len(latestState.BattleView.OwnBoard.Ships))
	}
}

func TestBattle_StrictAlternationAndShots(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, sGuest, _ := setupPlacementTest(t)

	_ = mgr.SubmitFleet(pHost.ID, validFleetForTest())
	_ = mgr.SubmitFleet(pGuest.ID, validGuestFleetForTest())

	turnPayload := filterMessages(sHost, "battle.turn")[0].payload.(map[string]any)
	firstShooter := turnPayload["turnPlayerId"].(string)
	secondShooter := pHost.ID
	if firstShooter == pHost.ID {
		secondShooter = pGuest.ID
	}

	sHost.reset()
	sGuest.reset()

	// 1. Inactive player tries to shoot out of turn -> rejected
	err := mgr.FireShot(secondShooter, game.Coord{X: 0, Y: 0})
	if !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("expected ErrNotYourTurn for inactive player, got %v", err)
	}

	// 2. Active player fires miss at (0, 9)
	err = mgr.FireShot(firstShooter, game.Coord{X: 0, Y: 9})
	if err != nil {
		t.Fatalf("active shooter FireShot failed: %v", err)
	}

	// Both receive battle.shotResult
	hostResults := filterMessages(sHost, "battle.shotResult")
	if len(hostResults) != 1 {
		t.Fatalf("expected 1 shotResult on host, got %d", len(hostResults))
	}
	resPayload := hostResults[0].payload.(map[string]any)
	if resPayload["result"] != game.ShotResultMiss {
		t.Errorf("expected miss at (0,9), got %v", resPayload["result"])
	}

	// Turn must have alternated to secondShooter
	hostTurns := filterMessages(sHost, "battle.turn")
	if len(hostTurns) != 1 {
		t.Fatalf("expected 1 turn update after shot, got %d", len(hostTurns))
	}
	nextTurn := hostTurns[0].payload.(map[string]any)["turnPlayerId"].(string)
	if nextTurn != secondShooter {
		t.Errorf("expected turn to alternate to %s, got %s", secondShooter, nextTurn)
	}

	// 3. First shooter cannot shoot again immediately (no extra turn on hit/miss)
	err = mgr.FireShot(firstShooter, game.Coord{X: 1, Y: 1})
	if !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("expected ErrNotYourTurn for first shooter after alternation, got %v", err)
	}

	// 4. Second shooter fires at (0, 9) on first shooter's board
	sHost.reset()
	sGuest.reset()
	err = mgr.FireShot(secondShooter, game.Coord{X: 0, Y: 9})
	if err != nil {
		t.Fatalf("second shooter shot failed: %v", err)
	}

	// Duplicate cell rejection: second shooter cannot fire at (0,9) again on next turn
	// Let first shooter shoot miss at (1, 9)
	_ = mgr.FireShot(firstShooter, game.Coord{X: 1, Y: 9})
	// Now second shooter tries to fire at (0, 9) again
	err = mgr.FireShot(secondShooter, game.Coord{X: 0, Y: 9})
	if !errors.Is(err, game.ErrCellAlreadyTargeted) {
		t.Errorf("expected ErrCellAlreadyTargeted when firing at same cell twice, got %v", err)
	}
}

func TestBattle_TurnTimerAutoShot(t *testing.T) {
	_, mgr, clock, pHost, pGuest, sHost, sGuest, _ := setupPlacementTest(t)

	_ = mgr.SubmitFleet(pHost.ID, validFleetForTest())
	_ = mgr.SubmitFleet(pGuest.ID, validGuestFleetForTest())

	turnPayload := filterMessages(sHost, "battle.turn")[0].payload.(map[string]any)
	firstShooter := turnPayload["turnPlayerId"].(string)
	secondShooter := pHost.ID
	if firstShooter == pHost.ID {
		secondShooter = pGuest.ID
	}

	sHost.reset()
	sGuest.reset()

	// Advance clock by 30 seconds -> turn timer expires
	clock.Advance(30 * time.Second)

	// An automatic shot should have been executed on behalf of firstShooter
	hostResults := filterMessages(sHost, "battle.shotResult")
	if len(hostResults) != 1 {
		t.Fatalf("expected 1 auto shotResult, got %d", len(hostResults))
	}
	shotPayload := hostResults[0].payload.(map[string]any)
	if shotPayload["by"] != firstShooter {
		t.Errorf("expected shooter to be %s, got %s", firstShooter, shotPayload["by"])
	}
	if shotPayload["auto"] != true {
		t.Errorf("expected auto: true on turn timeout shot, got %v", shotPayload["auto"])
	}

	// Turn must have automatically advanced to secondShooter
	turnMsgs := filterMessages(sHost, "battle.turn")
	if len(turnMsgs) != 1 {
		t.Fatalf("expected 1 turn message after auto-shot, got %d", len(turnMsgs))
	}
	nextShooter := turnMsgs[0].payload.(map[string]any)["turnPlayerId"].(string)
	if nextShooter != secondShooter {
		t.Errorf("expected turn to advance to %s, got %s", secondShooter, nextShooter)
	}

	// Second shooter can now fire normally
	err := mgr.FireShot(secondShooter, game.Coord{X: 3, Y: 9})
	if err != nil {
		t.Fatalf("second shooter normal shot failed after auto-shot: %v", err)
	}
}

func TestBattle_WinResolution(t *testing.T) {
	_, mgr, _, pHost, pGuest, sHost, _, _ := setupPlacementTest(t)

	_ = mgr.SubmitFleet(pHost.ID, validFleetForTest())
	_ = mgr.SubmitFleet(pGuest.ID, validGuestFleetForTest())

	turnPayload := filterMessages(sHost, "battle.turn")[0].payload.(map[string]any)
	currentShooter := turnPayload["turnPlayerId"].(string)

	// List of all coordinates on Guest's fleet (total 17 cells)
	guestCells := []game.Coord{
		{X: 5, Y: 0}, {X: 5, Y: 1}, {X: 5, Y: 2}, {X: 5, Y: 3}, {X: 5, Y: 4}, // Carrier
		{X: 6, Y: 0}, {X: 6, Y: 1}, {X: 6, Y: 2}, {X: 6, Y: 3}, // Battleship
		{X: 7, Y: 0}, {X: 7, Y: 1}, {X: 7, Y: 2}, // Cruiser
		{X: 8, Y: 0}, {X: 8, Y: 1}, {X: 8, Y: 2}, // Submarine
		{X: 9, Y: 0}, {X: 9, Y: 1}, // Destroyer
	}

	// If guest starts, guest shoots a miss at (0,9) so host gets the turn
	if currentShooter == pGuest.ID {
		_ = mgr.FireShot(pGuest.ID, game.Coord{X: 0, Y: 9})
	}

	// Host systematically sinks all guest ships (guest shoots harmless misses on their turns)
	for i, c := range guestCells {
		err := mgr.FireShot(pHost.ID, c)
		if err != nil {
			t.Fatalf("host shot %d at %v failed: %v", i, c, err)
		}

		if i < len(guestCells)-1 {
			// Guest fires a miss at an empty cell on host's board (rows 7 or 8)
			guestShot := game.Coord{X: i % 10, Y: 7 + (i / 10)}
			if err := mgr.FireShot(pGuest.ID, guestShot); err != nil {
				t.Fatalf("guest dummy shot %d at %v failed: %v", i, guestShot, err)
			}
		}
	}

	// Match should now be won by Host
	gameOverMsgs := filterMessages(sHost, "game.over")
	if len(gameOverMsgs) != 1 {
		t.Fatalf("expected 1 game.over message on match win, got %d", len(gameOverMsgs))
	}
	gameResult := gameOverMsgs[0].payload.(map[string]any)
	if gameResult["winnerPlayerId"] != pHost.ID {
		t.Errorf("expected winner to be host %s, got %s", pHost.ID, gameResult["winnerPlayerId"])
	}
	if gameResult["reason"] != "all_sunk" {
		t.Errorf("expected reason all_sunk, got %v", gameResult["reason"])
	}

	// Subsequent shots must be rejected with ErrGameOver
	err := mgr.FireShot(pGuest.ID, game.Coord{X: 9, Y: 9})
	if !errors.Is(err, ErrGameOver) {
		t.Errorf("expected ErrGameOver after game conclusion, got %v", err)
	}
}

func TestBattle_PlacementTimeoutStartsBattle(t *testing.T) {
	_, mgr, clock, _, _, sHost, sGuest, _ := setupPlacementTest(t)

	sHost.reset()
	sGuest.reset()

	// Neither player submits placement. Advance clock by 60s
	clock.Advance(60 * time.Second)

	// Both players should receive placement.confirmed (auto: true), battle.start, and battle.turn
	hostStarts := filterMessages(sHost, "battle.start")
	guestStarts := filterMessages(sGuest, "battle.start")
	if len(hostStarts) != 1 || len(guestStarts) != 1 {
		t.Fatalf("expected battle.start on placement timeout, got host=%d, guest=%d", len(hostStarts), len(guestStarts))
	}

	hostTurns := filterMessages(sHost, "battle.turn")
	if len(hostTurns) != 1 {
		t.Fatalf("expected battle.turn on placement timeout, got %d", len(hostTurns))
	}

	// Verify both players can take battle turns immediately
	firstShooter := hostTurns[0].payload.(map[string]any)["turnPlayerId"].(string)
	err := mgr.FireShot(firstShooter, game.Coord{X: 0, Y: 0})
	if err != nil {
		t.Fatalf("first shooter fire failed in auto-started battle: %v", err)
	}
}

func TestBattle_GameStorePersistence(t *testing.T) {
	reg := NewSessionRegistry()
	pHost, _ := reg.CreatePlayer("Fleet Admiral")
	pGuest, _ := reg.CreatePlayer("General Officer")

	mockSt := &testMockStore{}
	clock := NewMockClock(time.Now())
	mgr := NewLobbyManager(reg, WithClock(clock), WithStore(mockSt))

	sHost := &mockSender{}
	sGuest := &mockSender{}
	mgr.RegisterConnection(pHost.ID, sHost)
	mgr.RegisterConnection(pGuest.ID, sGuest)

	room, err := mgr.CreateRoom(pHost.ID)
	if err != nil {
		t.Fatalf("CreateRoom failed: %v", err)
	}
	if _, err := mgr.JoinRoom(pGuest.ID, room.ID); err != nil {
		t.Fatalf("JoinRoom failed: %v", err)
	}
	if err := mgr.ReadyRoom(pHost.ID); err != nil {
		t.Fatalf("ReadyRoom host failed: %v", err)
	}
	if err := mgr.ReadyRoom(pGuest.ID); err != nil {
		t.Fatalf("ReadyRoom guest failed: %v", err)
	}

	// Submit placements
	if err := mgr.SubmitFleet(pHost.ID, validFleetForTest()); err != nil {
		t.Fatalf("SubmitFleet host failed: %v", err)
	}
	if err := mgr.SubmitFleet(pGuest.ID, validGuestFleetForTest()); err != nil {
		t.Fatalf("SubmitFleet guest failed: %v", err)
	}

	// Forfeit via leaveMatchLocked
	if err := mgr.LeaveRoom(pHost.ID); err != nil {
		t.Fatalf("LeaveRoom forfeit failed: %v", err)
	}

	// Allow goroutine to execute persistence
	time.Sleep(50 * time.Millisecond)

	mockSt.mu.Lock()
	savedCount := len(mockSt.saved)
	var rec *store.GameRecord
	if savedCount > 0 {
		rec = mockSt.saved[0]
	}
	mockSt.mu.Unlock()

	if savedCount != 1 {
		t.Fatalf("expected 1 saved record on forfeit, got %d", savedCount)
	}
	if rec.EndReason != "forfeit" {
		t.Errorf("expected reason 'forfeit', got %q", rec.EndReason)
	}
	if rec.WinnerSlot == nil || *rec.WinnerSlot != 2 {
		t.Errorf("expected WinnerSlot 2 for guest on host forfeit, got %v", rec.WinnerSlot)
	}
	if rec.Player1Name != "Fleet Admiral" {
		t.Errorf("expected Player1Name 'Fleet Admiral', got %q", rec.Player1Name)
	}
	if rec.Player2Name != "General Officer" {
		t.Errorf("expected Player2Name 'General Officer', got %q", rec.Player2Name)
	}
}
