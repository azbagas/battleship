package game_test

import (
	"encoding/json"
	"errors"
	"math/rand"
	"testing"

	"github.com/azbagas/battleship/backend/internal/game"
)

// helper create standard test fleet
func createTestFleet() []game.PlacedShip {
	return []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 0, Y: 0}, Orientation: game.OrientationHorizontal},    // (0,0)..(4,0)
		{Type: game.ShipBattleship, Origin: game.Coord{X: 0, Y: 2}, Orientation: game.OrientationHorizontal}, // (0,2)..(3,2)
		{Type: game.ShipCruiser, Origin: game.Coord{X: 0, Y: 4}, Orientation: game.OrientationHorizontal},    // (0,4)..(2,4)
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 0, Y: 6}, Orientation: game.OrientationHorizontal},  // (0,6)..(2,6)
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 0, Y: 8}, Orientation: game.OrientationHorizontal},  // (0,8)..(1,8)
	}
}

func TestFireShot_MissHitSunkTransitions(t *testing.T) {
	board, err := game.NewBoard(createTestFleet())
	if err != nil {
		t.Fatalf("unexpected NewBoard error: %v", err)
	}

	// 1. Miss on empty cell (9, 9)
	outcome, err := board.FireShot(game.Coord{X: 9, Y: 9})
	if err != nil {
		t.Fatalf("unexpected error on valid miss: %v", err)
	}
	if outcome.Result != game.ShotResultMiss {
		t.Errorf("expected miss, got %v", outcome.Result)
	}
	if outcome.SunkShip != nil {
		t.Errorf("expected SunkShip to be nil on miss, got %+v", outcome.SunkShip)
	}

	// 2. Hit on Destroyer first cell (0, 8)
	outcome, err = board.FireShot(game.Coord{X: 0, Y: 8})
	if err != nil {
		t.Fatalf("unexpected error on hit: %v", err)
	}
	if outcome.Result != game.ShotResultHit {
		t.Errorf("expected hit, got %v", outcome.Result)
	}
	if outcome.SunkShip != nil {
		t.Errorf("expected SunkShip to be nil on partial hit, got %+v", outcome.SunkShip)
	}
	if board.RemainingShips() != 5 {
		t.Errorf("destroyer not yet sunk, expected 5 remaining ships, got %d", board.RemainingShips())
	}

	// 3. Sinking hit on Destroyer second cell (1, 8)
	outcome, err = board.FireShot(game.Coord{X: 1, Y: 8})
	if err != nil {
		t.Fatalf("unexpected error on sinking shot: %v", err)
	}
	if outcome.Result != game.ShotResultSunk {
		t.Errorf("expected sunk, got %v", outcome.Result)
	}
	if outcome.SunkShip == nil {
		t.Fatalf("expected SunkShip info, got nil")
	}
	if outcome.SunkShip.Type != game.ShipDestroyer {
		t.Errorf("expected sunk ship type %q, got %q", game.ShipDestroyer, outcome.SunkShip.Type)
	}
	expectedCells := []game.Coord{{X: 0, Y: 8}, {X: 1, Y: 8}}
	if len(outcome.SunkShip.Cells) != len(expectedCells) {
		t.Fatalf("expected %d cells, got %d", len(expectedCells), len(outcome.SunkShip.Cells))
	}
	for i, c := range expectedCells {
		if outcome.SunkShip.Cells[i] != c {
			t.Errorf("cell %d mismatch: expected %v, got %v", i, c, outcome.SunkShip.Cells[i])
		}
	}

	if board.RemainingShips() != 4 {
		t.Errorf("destroyer sunk, expected 4 remaining ships, got %d", board.RemainingShips())
	}
	if board.AllShipsSunk() {
		t.Errorf("expected not all ships sunk yet")
	}
}

func TestFireShot_Rejections(t *testing.T) {
	board, err := game.NewBoard(createTestFleet())
	if err != nil {
		t.Fatalf("unexpected NewBoard error: %v", err)
	}

	// Out of bounds
	outOfBounds := []game.Coord{
		{X: -1, Y: 0},
		{X: 10, Y: 0},
		{X: 0, Y: -1},
		{X: 0, Y: 10},
	}
	for _, c := range outOfBounds {
		_, err := board.FireShot(c)
		if !errors.Is(err, game.ErrCellOutOfBounds) {
			t.Errorf("expected ErrCellOutOfBounds for coord %v, got %v", c, err)
		}
	}

	// First shot succeeds
	c := game.Coord{X: 3, Y: 3}
	if _, err := board.FireShot(c); err != nil {
		t.Fatalf("unexpected shot error: %v", err)
	}

	// Duplicate shot rejected
	_, err = board.FireShot(c)
	if !errors.Is(err, game.ErrCellAlreadyTargeted) {
		t.Errorf("expected ErrCellAlreadyTargeted for duplicate cell, got %v", err)
	}
}

func TestAllShipsSunk(t *testing.T) {
	board, err := game.NewBoard(createTestFleet())
	if err != nil {
		t.Fatalf("unexpected NewBoard error: %v", err)
	}

	// Sink Carrier (0,0)..(4,0)
	for x := 0; x < 5; x++ {
		_, _ = board.FireShot(game.Coord{X: x, Y: 0})
	}
	// Sink Battleship (0,2)..(3,2)
	for x := 0; x < 4; x++ {
		_, _ = board.FireShot(game.Coord{X: x, Y: 2})
	}
	// Sink Cruiser (0,4)..(2,4)
	for x := 0; x < 3; x++ {
		_, _ = board.FireShot(game.Coord{X: x, Y: 4})
	}
	// Sink Submarine (0,6)..(2,6)
	for x := 0; x < 3; x++ {
		_, _ = board.FireShot(game.Coord{X: x, Y: 6})
	}

	if board.RemainingShips() != 1 {
		t.Errorf("expected 1 remaining ship, got %d", board.RemainingShips())
	}
	if board.AllShipsSunk() {
		t.Errorf("expected AllShipsSunk to be false when destroyer is still afloat")
	}

	// Sink Destroyer (0,8) and (1,8)
	_, _ = board.FireShot(game.Coord{X: 0, Y: 8})
	finalOutcome, err := board.FireShot(game.Coord{X: 1, Y: 8})
	if err != nil {
		t.Fatalf("unexpected shot error: %v", err)
	}
	if finalOutcome.Result != game.ShotResultSunk {
		t.Errorf("expected final shot to result in sunk, got %v", finalOutcome.Result)
	}

	if board.RemainingShips() != 0 {
		t.Errorf("expected 0 remaining ships, got %d", board.RemainingShips())
	}
	if !board.AllShipsSunk() {
		t.Errorf("expected AllShipsSunk to be true once all 5 ships are sunk")
	}
}

func TestRandomUntargetedCell(t *testing.T) {
	board, err := game.NewBoard(createTestFleet())
	if err != nil {
		t.Fatalf("unexpected NewBoard error: %v", err)
	}

	rng := rand.New(rand.NewSource(42))

	// Initial board has 100 untargeted cells
	cell, err := game.RandomUntargetedCell(board, rng)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cell.InBounds() {
		t.Errorf("cell %v is out of bounds", cell)
	}

	// Target 99 cells, leaving exactly (7, 7) untargeted
	for x := 0; x < game.GridWidth; x++ {
		for y := 0; y < game.GridHeight; y++ {
			if x == 7 && y == 7 {
				continue
			}
			_, _ = board.FireShot(game.Coord{X: x, Y: y})
		}
	}

	// Selection must find (7, 7)
	cell, err = game.RandomUntargetedCell(board, rng)
	if err != nil {
		t.Fatalf("unexpected error finding sole untargeted cell: %v", err)
	}
	if cell.X != 7 || cell.Y != 7 {
		t.Errorf("expected (7,7), got (%d,%d)", cell.X, cell.Y)
	}

	// Target the last cell
	_, _ = board.FireShot(game.Coord{X: 7, Y: 7})

	// When full, must return ErrNoUntargetedCells
	_, err = game.RandomUntargetedCell(board, rng)
	if !errors.Is(err, game.ErrNoUntargetedCells) {
		t.Errorf("expected ErrNoUntargetedCells, got %v", err)
	}
}

func TestChooseFirstPlayer(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	counts := map[string]int{"p1": 0, "p2": 0}

	for i := 0; i < 1000; i++ {
		p := game.ChooseFirstPlayer("p1", "p2", rng)
		counts[p]++
	}

	// Should be roughly 500 each (+/- 75)
	if counts["p1"] < 425 || counts["p1"] > 575 {
		t.Errorf("distribution outside expected 50/50 bounds: p1=%d, p2=%d", counts["p1"], counts["p2"])
	}
}

func TestBuildPlayerBattleView_ZeroInformationLeakage(t *testing.T) {
	fleetHost := createTestFleet()
	fleetGuest := []game.PlacedShip{
		{Type: game.ShipCarrier, Origin: game.Coord{X: 5, Y: 0}, Orientation: game.OrientationVertical},    // (5,0)..(5,4)
		{Type: game.ShipBattleship, Origin: game.Coord{X: 6, Y: 0}, Orientation: game.OrientationVertical}, // (6,0)..(6,3)
		{Type: game.ShipCruiser, Origin: game.Coord{X: 7, Y: 0}, Orientation: game.OrientationVertical},    // (7,0)..(7,2)
		{Type: game.ShipSubmarine, Origin: game.Coord{X: 8, Y: 0}, Orientation: game.OrientationVertical},  // (8,0)..(8,2)
		{Type: game.ShipDestroyer, Origin: game.Coord{X: 9, Y: 0}, Orientation: game.OrientationVertical},  // (9,0)..(9,1)
	}

	hostBoard, _ := game.NewBoard(fleetHost)
	guestBoard, _ := game.NewBoard(fleetGuest)

	// Guest shoots Host at (0,0) -> Hit on Host's Carrier
	_, _ = hostBoard.FireShot(game.Coord{X: 0, Y: 0})
	// Host shoots Guest at (2,2) -> Miss
	_, _ = guestBoard.FireShot(game.Coord{X: 2, Y: 2})
	// Host shoots Guest at (9,0) -> Hit on Guest's Destroyer
	_, _ = guestBoard.FireShot(game.Coord{X: 9, Y: 0})

	// Generate view for Host
	hostView := game.BuildPlayerBattleView(hostBoard, guestBoard)

	// Host's OwnBoard should show Host's ships and incoming shot at (0,0)
	if len(hostView.OwnBoard.Ships) != 5 {
		t.Errorf("host own board should have 5 ships, got %d", len(hostView.OwnBoard.Ships))
	}
	if len(hostView.OwnBoard.Shots) != 1 {
		t.Errorf("host own board should have 1 incoming shot, got %d", len(hostView.OwnBoard.Shots))
	}

	// CRITICAL ZERO-LEAKAGE CHECK: Host's TargetBoardView must NOT leak guest's hidden ships!
	if len(hostView.TargetBoard.SunkShips) != 0 {
		t.Errorf("guest destroyer is not sunk yet; sunkShips must be empty, got %d", len(hostView.TargetBoard.SunkShips))
	}
	if len(hostView.TargetBoard.Shots) != 2 {
		t.Errorf("host fired 2 shots at guest, expected 2 in TargetBoard.Shots, got %d", len(hostView.TargetBoard.Shots))
	}

	// JSON serialize hostView and ensure guest's unsunk ship coordinates (e.g. "5,0", "6,0", "7,0", "8,0")
	// and ship types carrier/battleship/cruiser/submarine do NOT appear in the targetBoard JSON!
	jsonBytes, err := json.Marshal(hostView.TargetBoard)
	if err != nil {
		t.Fatalf("failed to marshal target board: %v", err)
	}
	jsonStr := string(jsonBytes)
	for _, forbidden := range []string{"\"carrier\"", "\"battleship\"", "\"cruiser\"", "\"submarine\"", "\"origin\""} {
		if forbidden == "\"carrier\"" || forbidden == "\"battleship\"" {
			// Ensure it doesn't appear
			if containsSubstring(jsonStr, forbidden) {
				t.Fatalf("information leakage detected! TargetBoardView JSON contains %s: %s", forbidden, jsonStr)
			}
		}
	}

	// Now Host sinks Guest's Destroyer at (9,1)
	_, _ = guestBoard.FireShot(game.Coord{X: 9, Y: 1})
	hostViewAfterSunk := game.BuildPlayerBattleView(hostBoard, guestBoard)

	if len(hostViewAfterSunk.TargetBoard.SunkShips) != 1 {
		t.Fatalf("expected 1 sunk ship in TargetBoard after sink, got %d", len(hostViewAfterSunk.TargetBoard.SunkShips))
	}
	sunk := hostViewAfterSunk.TargetBoard.SunkShips[0]
	if sunk.Type != game.ShipDestroyer {
		t.Errorf("expected sunk type %q, got %q", game.ShipDestroyer, sunk.Type)
	}
	if len(sunk.Cells) != 2 {
		t.Errorf("expected 2 cells for destroyer, got %d", len(sunk.Cells))
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || (len(s) > len(sub) && jsonHasSubstring(s, sub)))
}

func jsonHasSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

