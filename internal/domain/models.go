package domain

import "time"

// RoomStatus represents the lifecycle state of a room.
type RoomStatus string

const (
	RoomStatusWaiting  RoomStatus = "waiting"
	RoomStatusActive   RoomStatus = "active"
	RoomStatusPaused   RoomStatus = "paused"
	RoomStatusFinished RoomStatus = "finished"
	RoomStatusExpired  RoomStatus = "expired"
)

// RoundStatus represents the lifecycle state of a round.
type RoundStatus string

const (
	RoundStatusWriting  RoundStatus = "writing"
	RoundStatusVoting   RoundStatus = "voting"
	RoundStatusRevealed RoundStatus = "revealed"
)

// Room represents a game room.
type Room struct {
	ID           string     `json:"id"`
	Code         string     `json:"code"`
	HostID       string     `json:"host_id"`
	MaxRounds    int        `json:"max_rounds"`
	TimePerRound int        `json:"time_per_round"`
	Status       RoomStatus `json:"status"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    time.Time  `json:"expires_at"`
}

// User represents a player in a room.
type User struct {
	ID        string    `json:"id"`
	RoomID    string    `json:"room_id"`
	Nickname  string    `json:"nickname"`
	AvatarURL string    `json:"avatar_url"`
	IsHost    bool      `json:"is_host"`
	CreatedAt time.Time `json:"created_at"`
}

// Round represents a game round.
type Round struct {
	ID          string      `json:"id"`
	RoomID      string      `json:"room_id"`
	RoundNumber int         `json:"round_number"`
	Status      RoundStatus `json:"status"`
	StartedAt   time.Time   `json:"started_at"`
	PausedAt    *time.Time  `json:"paused_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}

// Story represents an anonymous story submitted by a user.
type Story struct {
	ID         string    `json:"id"`
	RoundID    string    `json:"round_id"`
	UserID     string    `json:"user_id"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	IsRevealed bool      `json:"is_revealed"`
	CreatedAt  time.Time `json:"created_at"`
}

// Vote represents a vote cast for a story.
type Vote struct {
	ID        string    `json:"id"`
	StoryID   string    `json:"story_id"`
	UserID    string    `json:"user_id"`
	RoundID   string    `json:"round_id"`
	CreatedAt time.Time `json:"created_at"`
}
