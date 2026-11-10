package game

import (
	"errors"
	"fmt"
	"math/rand"
	"time"
)

var (
	ErrInvalidShipCount  = errors.New("fleet must contain exactly 5 ships")
	ErrDuplicateShipType = errors.New("duplicate ship type in fleet")
	ErrMissingShipType   = errors.New("missing required ship type")
	ErrShipOutOfBounds   = errors.New("ship extends outside grid bounds")
	ErrShipOverlap       = errors.New("ships cannot overlap")
	ErrInvalidOrientation = errors.New("invalid ship orientation")
	ErrUnknownShipType   = errors.New("unknown ship type")
	ErrNoValidPlacement  = errors.New("no valid placement possible for remaining ships")
)

// Board represents a player's 10×10 ocean board and fleet.
type Board struct {
	Ships []PlacedShip `json:"ships"`
}

// NewBoard creates and validates a board containing the standard 5-ship fleet.
func NewBoard(ships []PlacedShip) (*Board, error) {
	if err := ValidateFleet(ships); err != nil {
		return nil, err
	}
	copied := make([]PlacedShip, len(ships))
	copy(copied, ships)
	return &Board{Ships: copied}, nil
}

// ValidateFleet checks that a fleet of 5 ships meets all Battleship rules:
// 1. Exactly 5 ships.
// 2. Exactly one Carrier (5), Battleship (4), Cruiser (3), Submarine (3), and Destroyer (2).
// 3. Each ship has a valid orientation ("horizontal" or "vertical").
// 4. Each ship lies entirely within the 10×10 grid.
// 5. No ships overlap (share any grid cell).
// Note: Adjacent/touching ships are allowed per PRD.
func ValidateFleet(ships []PlacedShip) error {
	if len(ships) != len(StandardFleetTypes) {
		return fmt.Errorf("%w: expected %d, got %d", ErrInvalidShipCount, len(StandardFleetTypes), len(ships))
	}

	seenTypes := make(map[ShipType]bool, len(StandardFleetTypes))
	occupied := [GridWidth][GridHeight]bool{}

	for _, ship := range ships {
		if _, exists := StandardShipLengths[ship.Type]; !exists {
			return fmt.Errorf("%w: %q", ErrUnknownShipType, ship.Type)
		}
		if seenTypes[ship.Type] {
			return fmt.Errorf("%w: %q", ErrDuplicateShipType, ship.Type)
		}
		seenTypes[ship.Type] = true

		cells, err := ship.Cells()
		if err != nil {
			return fmt.Errorf("ship %q: %w", ship.Type, err)
		}

		for _, c := range cells {
			if occupied[c.X][c.Y] {
				return fmt.Errorf("%w at (%d,%d) by ship %q", ErrShipOverlap, c.X, c.Y, ship.Type)
			}
			occupied[c.X][c.Y] = true
		}
	}

	for _, requiredType := range StandardFleetTypes {
		if !seenTypes[requiredType] {
			return fmt.Errorf("%w: %q", ErrMissingShipType, requiredType)
		}
	}

	return nil
}

// GenerateRandomFleet generates a complete, valid 5-ship fleet on a 10×10 grid.
// If rng is nil, a new local random generator seeded with current time is used.
func GenerateRandomFleet(rng *rand.Rand) []PlacedShip {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	fleet, err := AutoPlaceRemaining(nil, rng)
	if err != nil {
		// Should never happen with empty existing ships
		panic(fmt.Sprintf("failed to generate random fleet: %v", err))
	}
	return fleet
}

// AutoPlaceRemaining preserves any valid existing placed ships and randomly
// positions any unplaced ships from the standard fleet on the remaining free cells.
// If existing already contains a valid 5-ship fleet, it is validated and returned untouched.
func AutoPlaceRemaining(existing []PlacedShip, rng *rand.Rand) ([]PlacedShip, error) {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	if len(existing) > len(StandardFleetTypes) {
		return nil, fmt.Errorf("%w: got %d ships", ErrInvalidShipCount, len(existing))
	}

	occupied := [GridWidth][GridHeight]bool{}
	seenTypes := make(map[ShipType]bool, len(StandardFleetTypes))

	// Validate existing ships
	for _, ship := range existing {
		if _, exists := StandardShipLengths[ship.Type]; !exists {
			return nil, fmt.Errorf("%w: %q", ErrUnknownShipType, ship.Type)
		}
		if seenTypes[ship.Type] {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateShipType, ship.Type)
		}
		seenTypes[ship.Type] = true

		cells, err := ship.Cells()
		if err != nil {
			return nil, fmt.Errorf("existing ship %q: %w", ship.Type, err)
		}

		for _, c := range cells {
			if occupied[c.X][c.Y] {
				return nil, fmt.Errorf("%w at (%d,%d) by existing ship %q", ErrShipOverlap, c.X, c.Y, ship.Type)
			}
			occupied[c.X][c.Y] = true
		}
	}

	// Identify unplaced ship types
	unplaced := make([]ShipType, 0, len(StandardFleetTypes)-len(existing))
	for _, requiredType := range StandardFleetTypes {
		if !seenTypes[requiredType] {
			unplaced = append(unplaced, requiredType)
		}
	}

	// If all ships were already placed, validate the full fleet
	if len(unplaced) == 0 {
		if err := ValidateFleet(existing); err != nil {
			return nil, err
		}
		res := make([]PlacedShip, len(existing))
		copy(res, existing)
		return res, nil
	}

	// Backtracking solver to place remaining ships
	placedNew, ok := placeBacktrack(unplaced, 0, occupied, rng)
	if !ok {
		return nil, ErrNoValidPlacement
	}

	result := make([]PlacedShip, 0, len(existing)+len(placedNew))
	result = append(result, existing...)
	result = append(result, placedNew...)

	return result, nil
}

// placeBacktrack recursively places unplaced ships using backtracking and randomized candidate selection.
func placeBacktrack(unplaced []ShipType, index int, occupied [GridWidth][GridHeight]bool, rng *rand.Rand) ([]PlacedShip, bool) {
	if index >= len(unplaced) {
		return []PlacedShip{}, true
	}

	shipType := unplaced[index]
	length := StandardShipLengths[shipType]
	orientations := []Orientation{OrientationHorizontal, OrientationVertical}

	// Collect all valid candidate positions for this ship
	var candidates []PlacedShip
	for _, orientation := range orientations {
		maxX := GridWidth
		maxY := GridHeight
		if orientation == OrientationHorizontal {
			maxX = GridWidth - length + 1
		} else {
			maxY = GridHeight - length + 1
		}

		for x := 0; x < maxX; x++ {
			for y := 0; y < maxY; y++ {
				// Check if candidate cells are free
				fits := true
				for i := 0; i < length; i++ {
					cx := x
					cy := y
					if orientation == OrientationHorizontal {
						cx += i
					} else {
						cy += i
					}
					if occupied[cx][cy] {
						fits = false
						break
					}
				}

				if fits {
					candidates = append(candidates, PlacedShip{
						Type:        shipType,
						Origin:      Coord{X: x, Y: y},
						Orientation: orientation,
					})
				}
			}
		}
	}

	if len(candidates) == 0 {
		return nil, false
	}

	// Shuffle candidates for uniform randomization
	rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	for _, cand := range candidates {
		// Mark cells as occupied
		nextOccupied := occupied
		for i := 0; i < length; i++ {
			cx := cand.Origin.X
			cy := cand.Origin.Y
			if cand.Orientation == OrientationHorizontal {
				cx += i
			} else {
				cy += i
			}
			nextOccupied[cx][cy] = true
		}

		// Recurse for the next unplaced ship
		subResult, ok := placeBacktrack(unplaced, index+1, nextOccupied, rng)
		if ok {
			return append([]PlacedShip{cand}, subResult...), true
		}
	}

	return nil, false
}

