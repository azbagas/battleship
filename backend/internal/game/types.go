package game

import "fmt"

// Grid dimensions for standard Battleship.
const (
	GridWidth  = 10
	GridHeight = 10
)

// Coord represents a 0-indexed position on the 10×10 grid.
// X corresponds to columns (0=A, 9=J).
// Y corresponds to rows (0=1, 9=10).
type Coord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// String returns standard algebraic notation (e.g. "A1", "J10").
func (c Coord) String() string {
	if c.X < 0 || c.X >= GridWidth || c.Y < 0 || c.Y >= GridHeight {
		return fmt.Sprintf("?(%d,%d)", c.X, c.Y)
	}
	col := rune('A' + c.X)
	row := c.Y + 1
	return fmt.Sprintf("%c%d", col, row)
}

// InBounds returns true if the coordinate is within the 10×10 grid.
func (c Coord) InBounds() bool {
	return c.X >= 0 && c.X < GridWidth && c.Y >= 0 && c.Y < GridHeight
}

// ShipType represents one of the 5 standard Battleship vessel types.
type ShipType string

const (
	ShipCarrier    ShipType = "carrier"
	ShipBattleship ShipType = "battleship"
	ShipCruiser    ShipType = "cruiser"
	ShipSubmarine  ShipType = "submarine"
	ShipDestroyer  ShipType = "destroyer"
)

// StandardShipLengths defines lengths for all 5 standard ship types.
var StandardShipLengths = map[ShipType]int{
	ShipCarrier:    5,
	ShipBattleship: 4,
	ShipCruiser:    3,
	ShipSubmarine:  3,
	ShipDestroyer:  2,
}

// StandardFleetTypes lists all 5 required ship types in standard order.
var StandardFleetTypes = []ShipType{
	ShipCarrier,
	ShipBattleship,
	ShipCruiser,
	ShipSubmarine,
	ShipDestroyer,
}

// Orientation represents ship direction on the grid.
type Orientation string

const (
	OrientationHorizontal Orientation = "horizontal"
	OrientationVertical   Orientation = "vertical"
)

// PlacedShip describes a positioned vessel on the grid.
type PlacedShip struct {
	Type        ShipType    `json:"type"`
	Origin      Coord       `json:"origin"`
	Orientation Orientation `json:"orientation"`
}

// Length returns the standard length of this ship type, or 0 if unknown.
func (s PlacedShip) Length() int {
	return StandardShipLengths[s.Type]
}

// Cells calculates the occupied grid coordinates of the ship.
func (s PlacedShip) Cells() ([]Coord, error) {
	length := s.Length()
	if length == 0 {
		return nil, fmt.Errorf("unknown ship type %q", s.Type)
	}

	cells := make([]Coord, length)
	for i := 0; i < length; i++ {
		var cell Coord
		switch s.Orientation {
		case OrientationHorizontal:
			cell = Coord{X: s.Origin.X + i, Y: s.Origin.Y}
		case OrientationVertical:
			cell = Coord{X: s.Origin.X, Y: s.Origin.Y + i}
		default:
			return nil, fmt.Errorf("invalid orientation %q for ship %q", s.Orientation, s.Type)
		}

		if !cell.InBounds() {
			return nil, fmt.Errorf("ship %q cell (%d,%d) out of grid bounds", s.Type, cell.X, cell.Y)
		}
		cells[i] = cell
	}

	return cells, nil
}

// ShotResult represents the resolution of a single shot.
type ShotResult string

const (
	ShotResultMiss ShotResult = "miss"
	ShotResultHit  ShotResult = "hit"
	ShotResultSunk ShotResult = "sunk"
)

// SunkShipInfo reveals the ship type and occupied coordinates once a vessel has been sunk.
type SunkShipInfo struct {
	Type  ShipType `json:"type"`
	Cells []Coord  `json:"cells"`
}

// ShotOutcome describes the immediate result of an executed shot.
type ShotOutcome struct {
	Result   ShotResult    `json:"result"`
	Cell     Coord         `json:"cell"`
	SunkShip *SunkShipInfo `json:"sunkShip,omitempty"`
}


