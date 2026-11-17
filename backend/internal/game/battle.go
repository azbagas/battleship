package game

import (
	"math/rand"
)

// ChooseFirstPlayer randomly selects one of two player IDs to take the opening turn.
// If rng is nil, a cryptographically backed RNG is used.
func ChooseFirstPlayer(playerA, playerB string, rng *rand.Rand) string {
	if rng == nil {
		rng = NewCryptoSeededRNG()
	}
	if rng.Intn(2) == 0 {
		return playerA
	}
	return playerB
}

// RandomUntargetedCell returns a randomly chosen cell on the 10×10 grid that has
// not yet received a shot on the specified board.
// Returns ErrNoUntargetedCells if all 100 cells on the grid have already been targeted.
func RandomUntargetedCell(board *Board, rng *rand.Rand) (Coord, error) {
	if rng == nil {
		rng = NewCryptoSeededRNG()
	}

	untargeted := make([]Coord, 0, GridWidth*GridHeight)
	for x := 0; x < GridWidth; x++ {
		for y := 0; y < GridHeight; y++ {
			c := Coord{X: x, Y: y}
			if board.Shots == nil {
				untargeted = append(untargeted, c)
				continue
			}
			if _, hit := board.Shots[c]; !hit {
				untargeted = append(untargeted, c)
			}
		}
	}

	if len(untargeted) == 0 {
		return Coord{}, ErrNoUntargetedCells
	}

	idx := rng.Intn(len(untargeted))
	return untargeted[idx], nil
}

// ShotView describes a single shot and its result on a grid.
type ShotView struct {
	Cell   Coord      `json:"cell"`
	Result ShotResult `json:"result"`
}

// OwnBoardView contains the player's own fleet and all incoming shots received.
type OwnBoardView struct {
	Ships          []PlacedShip   `json:"ships"`
	Shots          []ShotView     `json:"shots"`
	SunkShips      []SunkShipInfo `json:"sunkShips"`
	RemainingShips int            `json:"remainingShips"`
	AllSunk        bool           `json:"allSunk"`
}

// TargetBoardView provides a fog-of-war filtered perspective of the opponent's grid.
// Unrevealed opponent ships are strictly omitted to guarantee zero information leakage.
type TargetBoardView struct {
	Shots          []ShotView     `json:"shots"`
	SunkShips      []SunkShipInfo `json:"sunkShips"`
	RemainingShips int            `json:"remainingShips"`
	AllSunk        bool           `json:"allSunk"`
}

// PlayerBattleView packages the filtered board views for a specific player.
type PlayerBattleView struct {
	OwnBoard    OwnBoardView    `json:"ownBoard"`
	TargetBoard TargetBoardView `json:"targetBoard"`
}

// BuildPlayerBattleView constructs a per-player battle view from the authoritative boards.
// The opponent's unrevealed ships are never exposed to the client.
func BuildPlayerBattleView(ownBoard, opponentBoard *Board) PlayerBattleView {
	ownShots := make([]ShotView, 0)
	if ownBoard != nil && ownBoard.Shots != nil {
		for cell, res := range ownBoard.Shots {
			ownShots = append(ownShots, ShotView{Cell: cell, Result: res})
		}
	}

	targetShots := make([]ShotView, 0)
	if opponentBoard != nil && opponentBoard.Shots != nil {
		for cell, res := range opponentBoard.Shots {
			targetShots = append(targetShots, ShotView{Cell: cell, Result: res})
		}
	}

	var ownShips []PlacedShip
	var ownSunk []SunkShipInfo
	ownRemaining := 0
	ownAllSunk := false
	if ownBoard != nil {
		ownShips = ownBoard.Ships
		ownSunk = ownBoard.SunkShips()
		ownRemaining = ownBoard.RemainingShips()
		ownAllSunk = ownBoard.AllShipsSunk()
	}

	var oppSunk []SunkShipInfo
	oppRemaining := 0
	oppAllSunk := false
	if opponentBoard != nil {
		oppSunk = opponentBoard.SunkShips()
		oppRemaining = opponentBoard.RemainingShips()
		oppAllSunk = opponentBoard.AllShipsSunk()
	}

	return PlayerBattleView{
		OwnBoard: OwnBoardView{
			Ships:          ownShips,
			Shots:          ownShots,
			SunkShips:      ownSunk,
			RemainingShips: ownRemaining,
			AllSunk:        ownAllSunk,
		},
		TargetBoard: TargetBoardView{
			Shots:          targetShots,
			SunkShips:      oppSunk,
			RemainingShips: oppRemaining,
			AllSunk:        oppAllSunk,
		},
	}
}

