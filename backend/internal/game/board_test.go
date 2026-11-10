package game

import (
	"math/rand"
	"testing"
)

func validTestFleet() []PlacedShip {
	return []PlacedShip{
		{Type: ShipCarrier, Origin: Coord{X: 0, Y: 0}, Orientation: OrientationHorizontal},    // (0..4, 0)
		{Type: ShipBattleship, Origin: Coord{X: 0, Y: 1}, Orientation: OrientationHorizontal}, // (0..3, 1) - adjacent touching
		{Type: ShipCruiser, Origin: Coord{X: 0, Y: 2}, Orientation: OrientationHorizontal},    // (0..2, 2)
		{Type: ShipSubmarine, Origin: Coord{X: 0, Y: 3}, Orientation: OrientationHorizontal},  // (0..2, 3)
		{Type: ShipDestroyer, Origin: Coord{X: 0, Y: 4}, Orientation: OrientationHorizontal},  // (0..1, 4)
	}
}

func TestValidateFleet_Valid(t *testing.T) {
	fleet := validTestFleet()
	if err := ValidateFleet(fleet); err != nil {
		t.Fatalf("expected valid fleet, got error: %v", err)
	}

	// Mixed vertical & horizontal, corner placements, touching ships
	cornerFleet := []PlacedShip{
		{Type: ShipCarrier, Origin: Coord{X: 9, Y: 5}, Orientation: OrientationVertical},      // (9, 5..9)
		{Type: ShipBattleship, Origin: Coord{X: 5, Y: 9}, Orientation: OrientationHorizontal}, // (5..8, 9) touches carrier at (9,9)
		{Type: ShipCruiser, Origin: Coord{X: 0, Y: 0}, Orientation: OrientationVertical},       // (0, 0..2)
		{Type: ShipSubmarine, Origin: Coord{X: 0, Y: 3}, Orientation: OrientationHorizontal},  // (0..2, 3) touches cruiser at (0,2)
		{Type: ShipDestroyer, Origin: Coord{X: 4, Y: 4}, Orientation: OrientationVertical},    // (4, 4..5)
	}
	if err := ValidateFleet(cornerFleet); err != nil {
		t.Fatalf("expected valid corner fleet, got error: %v", err)
	}
}

func TestValidateFleet_InvalidCount(t *testing.T) {
	// 4 ships (missing Destroyer)
	shortFleet := []PlacedShip{
		{Type: ShipCarrier, Origin: Coord{X: 0, Y: 0}, Orientation: OrientationHorizontal},
		{Type: ShipBattleship, Origin: Coord{X: 0, Y: 1}, Orientation: OrientationHorizontal},
		{Type: ShipCruiser, Origin: Coord{X: 0, Y: 2}, Orientation: OrientationHorizontal},
		{Type: ShipSubmarine, Origin: Coord{X: 0, Y: 3}, Orientation: OrientationHorizontal},
	}
	if err := ValidateFleet(shortFleet); err == nil {
		t.Fatal("expected error for 4-ship fleet, got nil")
	}

	// 6 ships
	longFleet := append(validTestFleet(), PlacedShip{
		Type: ShipDestroyer, Origin: Coord{X: 5, Y: 5}, Orientation: OrientationHorizontal,
	})
	if err := ValidateFleet(longFleet); err == nil {
		t.Fatal("expected error for 6-ship fleet, got nil")
	}
}

func TestValidateFleet_DuplicatesAndMissing(t *testing.T) {
	// Duplicate Destroyer, missing Carrier
	fleet := []PlacedShip{
		{Type: ShipDestroyer, Origin: Coord{X: 0, Y: 0}, Orientation: OrientationHorizontal},
		{Type: ShipBattleship, Origin: Coord{X: 0, Y: 1}, Orientation: OrientationHorizontal},
		{Type: ShipCruiser, Origin: Coord{X: 0, Y: 2}, Orientation: OrientationHorizontal},
		{Type: ShipSubmarine, Origin: Coord{X: 0, Y: 3}, Orientation: OrientationHorizontal},
		{Type: ShipDestroyer, Origin: Coord{X: 0, Y: 4}, Orientation: OrientationHorizontal},
	}
	if err := ValidateFleet(fleet); err == nil {
		t.Fatal("expected error for duplicate Destroyer, got nil")
	}
}

func TestValidateFleet_OutOfBounds(t *testing.T) {
	tests := []struct {
		name string
		ship PlacedShip
	}{
		{
			name: "horizontal exceeds width",
			ship: PlacedShip{Type: ShipCarrier, Origin: Coord{X: 6, Y: 0}, Orientation: OrientationHorizontal}, // 6+5 = 11 > 10
		},
		{
			name: "vertical exceeds height",
			ship: PlacedShip{Type: ShipBattleship, Origin: Coord{X: 0, Y: 7}, Orientation: OrientationVertical}, // 7+4 = 11 > 10
		},
		{
			name: "negative X origin",
			ship: PlacedShip{Type: ShipDestroyer, Origin: Coord{X: -1, Y: 0}, Orientation: OrientationHorizontal},
		},
		{
			name: "negative Y origin",
			ship: PlacedShip{Type: ShipDestroyer, Origin: Coord{X: 0, Y: -1}, Orientation: OrientationVertical},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fleet := []PlacedShip{
				tt.ship,
				{Type: ShipBattleship, Origin: Coord{X: 0, Y: 1}, Orientation: OrientationHorizontal},
				{Type: ShipCruiser, Origin: Coord{X: 0, Y: 2}, Orientation: OrientationHorizontal},
				{Type: ShipSubmarine, Origin: Coord{X: 0, Y: 3}, Orientation: OrientationHorizontal},
				{Type: ShipDestroyer, Origin: Coord{X: 0, Y: 4}, Orientation: OrientationHorizontal},
			}
			// replace the matching type in template to avoid duplicate errors masking bounds error
			for i, s := range fleet[1:] {
				if s.Type == tt.ship.Type {
					fleet[i+1].Type = ShipCarrier
					break
				}
			}
			if err := ValidateFleet(fleet); err == nil {
				t.Fatalf("expected error for %s, got nil", tt.name)
			}
		})
	}
}

func TestValidateFleet_Overlap(t *testing.T) {
	// Carrier horizontal on Y=0 (X=0..4), Battleship vertical at (2, 0..3) -> intersects at (2,0)
	fleet := []PlacedShip{
		{Type: ShipCarrier, Origin: Coord{X: 0, Y: 0}, Orientation: OrientationHorizontal},
		{Type: ShipBattleship, Origin: Coord{X: 2, Y: 0}, Orientation: OrientationVertical},
		{Type: ShipCruiser, Origin: Coord{X: 0, Y: 2}, Orientation: OrientationHorizontal},
		{Type: ShipSubmarine, Origin: Coord{X: 0, Y: 3}, Orientation: OrientationHorizontal},
		{Type: ShipDestroyer, Origin: Coord{X: 0, Y: 4}, Orientation: OrientationHorizontal},
	}
	if err := ValidateFleet(fleet); err == nil {
		t.Fatal("expected error for overlapping ships, got nil")
	}
}

func TestValidateFleet_InvalidOrientation(t *testing.T) {
	fleet := validTestFleet()
	fleet[0].Orientation = "diagonal"
	if err := ValidateFleet(fleet); err == nil {
		t.Fatal("expected error for invalid orientation, got nil")
	}
}

func TestGenerateRandomFleet(t *testing.T) {
	rng := rand.New(rand.NewSource(42))

	// Generate 1000 random fleets and assert that every single one passes ValidateFleet
	for i := 0; i < 1000; i++ {
		fleet := GenerateRandomFleet(rng)
		if err := ValidateFleet(fleet); err != nil {
			t.Fatalf("random fleet #%d failed validation: %v\nfleet: %+v", i, err, fleet)
		}
	}
}

func TestAutoPlaceRemaining_PreservesExistingShips(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))

	// Test case 1: 0 placed ships -> generates complete fleet
	emptyFleet, err := AutoPlaceRemaining(nil, rng)
	if err != nil {
		t.Fatalf("AutoPlaceRemaining with 0 ships failed: %v", err)
	}
	if err := ValidateFleet(emptyFleet); err != nil {
		t.Fatalf("AutoPlaceRemaining(nil) resulted in invalid fleet: %v", err)
	}

	// Test case 2: 2 already placed ships (Carrier & Destroyer)
	existing := []PlacedShip{
		{Type: ShipCarrier, Origin: Coord{X: 1, Y: 1}, Orientation: OrientationHorizontal},
		{Type: ShipDestroyer, Origin: Coord{X: 8, Y: 7}, Orientation: OrientationVertical},
	}

	result, err := AutoPlaceRemaining(existing, rng)
	if err != nil {
		t.Fatalf("AutoPlaceRemaining with 2 ships failed: %v", err)
	}
	if err := ValidateFleet(result); err != nil {
		t.Fatalf("result fleet failed validation: %v", err)
	}

	// Verify existing ships preserved with exact same origin and orientation
	carrierFound := false
	destroyerFound := false
	for _, s := range result {
		if s.Type == ShipCarrier {
			carrierFound = true
			if s.Origin != (Coord{X: 1, Y: 1}) || s.Orientation != OrientationHorizontal {
				t.Fatalf("carrier was modified: %+v", s)
			}
		}
		if s.Type == ShipDestroyer {
			destroyerFound = true
			if s.Origin != (Coord{X: 8, Y: 7}) || s.Orientation != OrientationVertical {
				t.Fatalf("destroyer was modified: %+v", s)
			}
		}
	}
	if !carrierFound || !destroyerFound {
		t.Fatal("existing ships missing in result")
	}

	// Test case 3: 5 valid already placed ships -> returns existing untouched
	fullFleet := validTestFleet()
	resultFull, err := AutoPlaceRemaining(fullFleet, rng)
	if err != nil {
		t.Fatalf("AutoPlaceRemaining with 5 ships failed: %v", err)
	}
	if len(resultFull) != 5 {
		t.Fatalf("expected 5 ships, got %d", len(resultFull))
	}

	// Test case 4: invalid existing ships (overlapping) -> returns error
	invalidExisting := []PlacedShip{
		{Type: ShipCarrier, Origin: Coord{X: 0, Y: 0}, Orientation: OrientationHorizontal},
		{Type: ShipDestroyer, Origin: Coord{X: 0, Y: 0}, Orientation: OrientationVertical},
	}
	if _, err := AutoPlaceRemaining(invalidExisting, rng); err == nil {
		t.Fatal("expected error when auto-placing with overlapping existing ships, got nil")
	}
}

func TestCoord_String(t *testing.T) {
	tests := []struct {
		c    Coord
		want string
	}{
		{Coord{X: 0, Y: 0}, "A1"},
		{Coord{X: 9, Y: 9}, "J10"},
		{Coord{X: 4, Y: 6}, "E7"},
		{Coord{X: -1, Y: 0}, "?(-1,0)"},
		{Coord{X: 10, Y: 0}, "?(10,0)"},
	}
	for _, tt := range tests {
		if got := tt.c.String(); got != tt.want {
			t.Errorf("Coord(%d,%d).String() = %q; want %q", tt.c.X, tt.c.Y, got, tt.want)
		}
	}
}

func TestNewBoard(t *testing.T) {
	fleet := validTestFleet()
	board, err := NewBoard(fleet)
	if err != nil {
		t.Fatalf("NewBoard failed: %v", err)
	}
	if len(board.Ships) != 5 {
		t.Fatalf("expected 5 ships, got %d", len(board.Ships))
	}

	// Mutating input slice shouldn't affect board (defensive copy)
	fleet[0].Origin.X = 99
	if board.Ships[0].Origin.X == 99 {
		t.Fatal("board.Ships was mutated when input slice changed")
	}
}

