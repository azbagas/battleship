package lobby

import (
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
)

// RoomPhase represents the lifecycle stage of a game room.
type RoomPhase string

const (
	PhaseWaiting   RoomPhase = "waiting"
	PhaseReadyUp   RoomPhase = "ready-up"
	PhasePlacement RoomPhase = "placement"
	PhaseBattle    RoomPhase = "battle"
	PhaseFinished  RoomPhase = "finished"
)

// Room represents an in-memory game room.
type Room struct {
	ID                   string
	HostPlayerID         string
	GuestPlayerID        string
	Phase                RoomPhase
	HostReady            bool
	GuestReady           bool
	CreatedAt            time.Time
	DisconnectTimer      Timer
	HostDisconnectTimer  Timer
	GuestDisconnectTimer Timer
	PlacementDeadline    time.Time
	PlacementTimer       Timer
	HostFleet            []game.PlacedShip
	GuestFleet           []game.PlacedShip
	HostFleetConfirmed   bool
	GuestFleetConfirmed  bool
	HostBoard            *game.Board
	GuestBoard           *game.Board
	TurnPlayerID         string
	TurnDeadline         time.Time
	TurnTimer            Timer
	ShotLog              []ShotRecord
	WinnerPlayerID       string
	EndReason            string
}

// ShotRecord logs an individual shot fired during battle.
type ShotRecord struct {
	ShooterID string             `json:"shooterId"`
	Cell      game.Coord         `json:"cell"`
	Result    game.ShotResult    `json:"result"`
	SunkShip  *game.SunkShipInfo `json:"sunkShip,omitempty"`
	Auto      bool               `json:"auto"`
	Timestamp time.Time          `json:"timestamp"`
}

// WaitingRoomView describes an open waiting room visible to lobby browsers.
type WaitingRoomView struct {
	ID        string    `json:"id"`
	HostName  string    `json:"hostName"`
	CreatedAt time.Time `json:"createdAt"`
}

// RoomPlayerView contains the public identifier and display name of a player in a room.
type RoomPlayerView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// RoomReadyStateView carries real-time ready-up status for both players.
type RoomReadyStateView struct {
	RoomID     string `json:"roomId"`
	HostReady  bool   `json:"hostReady"`
	GuestReady bool   `json:"guestReady"`
}

// RoomStateView is sent to players upon entering or updating a room.
type RoomStateView struct {
	RoomID            string                 `json:"roomId"`
	Phase             RoomPhase              `json:"phase"`
	Role              string                 `json:"role"` // "host" or "guest"
	OpponentName      string                 `json:"opponentName,omitempty"`
	Host              RoomPlayerView         `json:"host"`
	Guest             *RoomPlayerView        `json:"guest,omitempty"`
	HostReady         bool                   `json:"hostReady"`
	GuestReady        bool                   `json:"guestReady"`
	CreatedAt         time.Time              `json:"createdAt"`
	PlacementDeadline *time.Time             `json:"placementDeadline,omitempty"`
	FleetConfirmed    bool                   `json:"fleetConfirmed"`
	OpponentConfirmed bool                   `json:"opponentConfirmed"`
	OwnFleet          []game.PlacedShip      `json:"ownFleet,omitempty"`
	TurnPlayerID      string                 `json:"turnPlayerId,omitempty"`
	TurnDeadline      *time.Time             `json:"turnDeadline,omitempty"`
	BattleView        *game.PlayerBattleView `json:"battleView,omitempty"`
	ShotLog           []ShotRecord           `json:"shotLog,omitempty"`
	WinnerPlayerID    string                 `json:"winnerPlayerId,omitempty"`
	EndReason         string                 `json:"endReason,omitempty"`
}

// LobbySnapshotPayload represents the full list of waiting rooms sent on entry.
type LobbySnapshotPayload struct {
	Rooms      []WaitingRoomView `json:"rooms"`
	TotalRooms int               `json:"totalRooms"`
	MaxRooms   int               `json:"maxRooms"`
}

// LobbyUpdatePayload broadcasts incremental updates to lobby subscribers.
type LobbyUpdatePayload struct {
	Action     string           `json:"action"` // "upsert" or "remove"
	Room       *WaitingRoomView `json:"room,omitempty"`
	RoomID     string           `json:"roomId,omitempty"`
	TotalRooms int              `json:"totalRooms"`
	MaxRooms   int              `json:"maxRooms"`
}

