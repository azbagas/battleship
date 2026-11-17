package lobby

import (
	"time"

	"github.com/azbagas/battleship/backend/internal/game"
)

const (
	placementTimeout = 60 * time.Second
	turnTimeout      = 30 * time.Second

	endReasonAllSunk           = "all_sunk"
	endReasonForfeit           = "forfeit"
	endReasonDisconnectTimeout = "disconnect_timeout"
	endReasonAbandoned         = "abandoned"
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
	ID                      string
	HostPlayerID            string
	GuestPlayerID           string
	Phase                   RoomPhase
	HostReady               bool
	GuestReady              bool
	CreatedAt               time.Time
	DisconnectTimer         Timer
	HostDisconnectTimer     Timer
	GuestDisconnectTimer    Timer
	HostDisconnectDeadline  time.Time
	GuestDisconnectDeadline time.Time
	PlacementDeadline       time.Time
	PlacementTimer          Timer
	HostFleet               []game.PlacedShip
	GuestFleet              []game.PlacedShip
	HostFleetConfirmed      bool
	GuestFleetConfirmed     bool
	HostBoard               *game.Board
	GuestBoard              *game.Board
	TurnPlayerID            string
	TurnDeadline            time.Time
	TurnTimer               Timer
	ShotLog                 []ShotRecord
	WinnerPlayerID          string
	EndReason               string
	GameID                  string
	StartedAt               time.Time
	EndedAt                 time.Time
	Recorded                bool
	HostRematch             bool
	GuestRematch            bool
	RematchCancelReason     string
}

// stopDisconnectTimers stops all disconnect grace timers and clears their deadlines.
func (r *Room) stopDisconnectTimers() {
	if r.DisconnectTimer != nil {
		r.DisconnectTimer.Stop()
		r.DisconnectTimer = nil
	}
	if r.HostDisconnectTimer != nil {
		r.HostDisconnectTimer.Stop()
		r.HostDisconnectTimer = nil
	}
	if r.GuestDisconnectTimer != nil {
		r.GuestDisconnectTimer.Stop()
		r.GuestDisconnectTimer = nil
	}
	r.HostDisconnectDeadline = time.Time{}
	r.GuestDisconnectDeadline = time.Time{}
}

// stopMatchTimers stops active placement and turn timers.
func (r *Room) stopMatchTimers() {
	if r.PlacementTimer != nil {
		r.PlacementTimer.Stop()
		r.PlacementTimer = nil
	}
	if r.TurnTimer != nil {
		r.TurnTimer.Stop()
		r.TurnTimer = nil
	}
}

// stopAllTimers stops all disconnect and match timers associated with the room.
func (r *Room) stopAllTimers() {
	r.stopDisconnectTimers()
	r.stopMatchTimers()
}

// roleAndOpponent returns the player's role ("host" or "guest"), their opponent's ID,
// and whether playerID belongs to this room.
func (r *Room) roleAndOpponent(playerID string) (string, string, bool) {
	switch playerID {
	case r.HostPlayerID:
		return "host", r.GuestPlayerID, true
	case r.GuestPlayerID:
		return "guest", r.HostPlayerID, true
	default:
		return "", "", false
	}
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
	RoomID                 string                 `json:"roomId"`
	Phase                  RoomPhase              `json:"phase"`
	Role                   string                 `json:"role"` // "host" or "guest"
	OpponentName           string                 `json:"opponentName,omitempty"`
	Host                   RoomPlayerView         `json:"host"`
	Guest                  *RoomPlayerView        `json:"guest,omitempty"`
	HostReady              bool                   `json:"hostReady"`
	GuestReady             bool                   `json:"guestReady"`
	CreatedAt              time.Time              `json:"createdAt"`
	PlacementDeadline      *time.Time             `json:"placementDeadline,omitempty"`
	FleetConfirmed         bool                   `json:"fleetConfirmed"`
	OpponentConfirmed      bool                   `json:"opponentConfirmed"`
	OwnFleet               []game.PlacedShip      `json:"ownFleet,omitempty"`
	TurnPlayerID           string                 `json:"turnPlayerId,omitempty"`
	TurnDeadline           *time.Time             `json:"turnDeadline,omitempty"`
	BattleView             *game.PlayerBattleView `json:"battleView,omitempty"`
	ShotLog                []ShotRecord           `json:"shotLog,omitempty"`
	WinnerPlayerID         string                 `json:"winnerPlayerId,omitempty"`
	EndReason              string                 `json:"endReason,omitempty"`
	OpponentDisconnected   bool                   `json:"opponentDisconnected,omitempty"`
	OpponentGraceDeadline  *time.Time             `json:"opponentGraceDeadline,omitempty"`
	RematchOfferedBy       string                 `json:"rematchOfferedBy,omitempty"`
	RematchCancelReason    string                 `json:"rematchCancelReason,omitempty"`
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

