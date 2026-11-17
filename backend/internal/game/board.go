package game

import (
	"errors"
	"fmt"
	"math/rand"
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
	ErrCellOutOfBounds   = errors.New("shot coordinate is out of grid bounds")
	ErrCellAlreadyTargeted = errors.New("cell has already been targeted")
	ErrNoUntargetedCells = errors.New("no untargeted cells remain on grid")
)

// Board represents a player's 10×10 ocean board and fleet.
type Board struct {
	Ships []PlacedShip         `json:"ships"`
	Shots map[Coord]ShotResult `json:"shots"`
}

// NewBoard creates and validates a board containing the standard 5-ship fleet.
func NewBoard(ships []PlacedShip) (*Board, error) {
	if err := ValidateFleet(ships); err != nil {
		return nil, err
	}
	copied := make([]PlacedShip, len(ships))
	copy(copied, ships)
	return &Board{
		Ships: copied,
		Shots: make(map[Coord]ShotResult),
	}, nil
}

// FireShot executes a shot against the board and resolves it as miss, hit, or sunk.
func (b *Board) FireShot(cell Coord) (ShotOutcome, error) {
	if !cell.InBounds() {
		return ShotOutcome{}, fmt.Errorf("%w: (%d,%d)", ErrCellOutOfBounds, cell.X, cell.Y)
	}

	if b.Shots == nil {
		b.Shots = make(map[Coord]ShotResult)
	}

	if _, exists := b.Shots[cell]; exists {
		return ShotOutcome{}, fmt.Errorf("%w: %s (%d,%d)", ErrCellAlreadyTargeted, cell.String(), cell.X, cell.Y)
	}

	// Check if this shot hits any ship
	for _, ship := range b.Ships {
		cells, err := ship.Cells()
		if err != nil {
			continue
		}

		for _, sc := range cells {
			if sc == cell {
				// Hit!
				b.Shots[cell] = ShotResultHit

				// Check if all cells of this ship are now hit
				allHit := true
				for _, c := range cells {
					res, hit := b.Shots[c]
					if !hit || (res != ShotResultHit && res != ShotResultSunk) {
						allHit = false
						break
					}
				}

				if allHit {
					// Mark all cells of this ship as sunk
					for _, c := range cells {
						b.Shots[c] = ShotResultSunk
					}
					sunkInfo := &SunkShipInfo{
						Type:  ship.Type,
						Cells: cells,
					}
					return ShotOutcome{
						Result:   ShotResultSunk,
						Cell:     cell,
						SunkShip: sunkInfo,
					}, nil
				}

				return ShotOutcome{
					Result: ShotResultHit,
					Cell:   cell,
				}, nil
			}
		}
	}

	// Miss
	b.Shots[cell] = ShotResultMiss
	return ShotOutcome{
		Result: ShotResultMiss,
		Cell:   cell,
	}, nil
}

// SunkShips returns information on all ships that have been completely sunk on this board.
func (b *Board) SunkShips() []SunkShipInfo {
	if b.Shots == nil {
		return nil
	}

	var sunk []SunkShipInfo
	for _, ship := range b.Ships {
		cells, err := ship.Cells()
		if err != nil {
			continue
		}

		allHit := true
		for _, c := range cells {
			res, hit := b.Shots[c]
			if !hit || (res != ShotResultHit && res != ShotResultSunk) {
				allHit = false
				break
			}
		}

		if allHit {
			sunk = append(sunk, SunkShipInfo{
				Type:  ship.Type,
				Cells: cells,
			})
		}
	}

	return sunk
}

// RemainingShips returns the count of ships in the fleet that are still afloat.
func (b *Board) RemainingShips() int {
	return len(b.Ships) - len(b.SunkShips())
}

// AllShipsSunk returns true if all 5 ships in the fleet have been sunk.
func (b *Board) AllShipsSunk() bool {
	if len(b.Ships) == 0 {
		return false
	}
	return len(b.SunkShips()) == len(b.Ships)
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
// If rng is nil, a cryptographically backed RNG is used.
func GenerateRandomFleet(rng *rand.Rand) []PlacedShip {
	if rng == nil {
		rng = NewCryptoSeededRNG()
	}
	fleet, err := AutoPlaceRemaining(nil, rng)
	if err != nil {
		// Should never happen with empty existing ships
		panic(fmt.Sprintf("failed to generate random fleet: %v", err))
	}
	return fleet
}

// ValidatePartialFleet checks that a partial or complete slice of placed ships (0..5 ships)
// contains only valid ship types without duplicates, valid orientations, in-bounds cells, and no overlaps.
func ValidatePartialFleet(ships []PlacedShip) error {
	if len(ships) > len(StandardFleetTypes) {
		return fmt.Errorf("%w: got %d ships", ErrInvalidShipCount, len(ships))
	}

	occupied := [GridWidth][GridHeight]bool{}
	seenTypes := make(map[ShipType]bool, len(StandardFleetTypes))

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
			return fmt.Errorf("existing ship %q: %w", ship.Type, err)
		}

		for _, c := range cells {
			if occupied[c.X][c.Y] {
				return fmt.Errorf("%w at (%d,%d) by existing ship %q", ErrShipOverlap, c.X, c.Y, ship.Type)
			}
			occupied[c.X][c.Y] = true
		}
	}

	return nil
}

// AutoPlaceRemaining preserves any valid existing placed ships and randomly
// positions any unplaced ships from the standard fleet on the remaining free cells.
// If existing already contains a valid 5-ship fleet, it is validated and returned untouched.
func AutoPlaceRemaining(existing []PlacedShip, rng *rand.Rand) ([]PlacedShip, error) {
	if rng == nil {
		rng = NewCryptoSeededRNG()
	}

	if err := ValidatePartialFleet(existing); err != nil {
		return nil, err
	}

	occupied := [GridWidth][GridHeight]bool{}
	seenTypes := make(map[ShipType]bool, len(StandardFleetTypes))

	for _, ship := range existing {
		seenTypes[ship.Type] = true
		cells, _ := ship.Cells()
		for _, c := range cells {
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

