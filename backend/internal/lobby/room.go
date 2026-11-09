package lobby

import (
	"time"
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
	ID              string
	HostPlayerID    string
	GuestPlayerID   string
	Phase           RoomPhase
	HostReady       bool
	GuestReady      bool
	CreatedAt       time.Time
	DisconnectTimer Timer
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

// RoomStateView is sent to players upon entering or updating a room.
type RoomStateView struct {
	RoomID       string          `json:"roomId"`
	Phase        RoomPhase       `json:"phase"`
	Role         string          `json:"role"` // "host" or "guest"
	OpponentName string          `json:"opponentName,omitempty"`
	Host         RoomPlayerView  `json:"host"`
	Guest        *RoomPlayerView `json:"guest,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
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

