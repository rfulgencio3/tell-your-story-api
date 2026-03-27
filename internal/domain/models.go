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
	ID           string     `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Code         string     `json:"code" gorm:"uniqueIndex;size:6;not null"`
	HostID       string     `json:"host_id" gorm:"type:varchar(64);not null;index"`
	MaxRounds    int        `json:"max_rounds" gorm:"not null"`
	TimePerRound int        `json:"time_per_round" gorm:"not null"`
	Status       RoomStatus `json:"status" gorm:"type:varchar(32);not null;index"`
	CreatedAt    time.Time  `json:"created_at" gorm:"not null;index"`
	ExpiresAt    time.Time  `json:"expires_at" gorm:"not null;index"`
}

// User represents a player in a room.
type User struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoomID    string    `json:"room_id" gorm:"type:varchar(64);not null;index"`
	Nickname  string    `json:"nickname" gorm:"size:50;not null"`
	AvatarURL string    `json:"avatar_url" gorm:"size:255"`
	IsHost    bool      `json:"is_host" gorm:"not null;default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;index"`
}

// Round represents a game round.
type Round struct {
	ID          string      `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoomID      string      `json:"room_id" gorm:"type:varchar(64);not null;index"`
	RoundNumber int         `json:"round_number" gorm:"not null"`
	Status      RoundStatus `json:"status" gorm:"type:varchar(32);not null;index"`
	StartedAt   time.Time   `json:"started_at" gorm:"not null;index"`
	PausedAt    *time.Time  `json:"paused_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
}

// Story represents an anonymous story submitted by a user.
type Story struct {
	ID         string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoundID    string    `json:"round_id" gorm:"type:varchar(64);not null;index"`
	UserID     string    `json:"user_id" gorm:"type:varchar(64);not null;index"`
	Title      string    `json:"title" gorm:"size:100;not null"`
	Body       string    `json:"body" gorm:"size:500;not null"`
	IsRevealed bool      `json:"is_revealed" gorm:"not null;default:false"`
	CreatedAt  time.Time `json:"created_at" gorm:"not null;index"`
}

// Vote represents a vote cast for a story.
type Vote struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	StoryID   string    `json:"story_id" gorm:"type:varchar(64);not null;index"`
	UserID    string    `json:"user_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_vote_user_round"`
	RoundID   string    `json:"round_id" gorm:"type:varchar(64);not null;uniqueIndex:idx_vote_user_round;index"`
	CreatedAt time.Time `json:"created_at" gorm:"not null;index"`
}
