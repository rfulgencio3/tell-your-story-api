package domain

import (
	"strings"
	"time"
)

const (
	// GameTypeIDTellYourStory is the stable catalog identifier for the default mode.
	GameTypeIDTellYourStory = "00000000000000000000000000000001"
	// GameTypeIDThreeLiesOneTruth is the stable catalog identifier for the three-lies mode.
	GameTypeIDThreeLiesOneTruth = "00000000000000000000000000000002"
	// GameTypeTellYourStory is the public slug for the default mode.
	GameTypeTellYourStory = "tell-your-story"
	// GameTypeThreeLiesOneTruth is the public slug for the alternative mode.
	GameTypeThreeLiesOneTruth = "three-lies-one-truth"
)

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
	RoundStatusCountdown          RoundStatus = "countdown"
	RoundStatusWriting            RoundStatus = "writing"
	RoundStatusVoting             RoundStatus = "voting"
	RoundStatusRevealed           RoundStatus = "revealed"
	RoundStatusPresentationVoting RoundStatus = "presentation_voting"
	RoundStatusReveal             RoundStatus = "reveal"
	RoundStatusCommentary         RoundStatus = "commentary"
	RoundStatusFinished           RoundStatus = "finished"
)

// GameType represents a supported game mode.
type GameType struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Slug        string     `json:"slug" gorm:"uniqueIndex;size:64;not null"`
	Name        string     `json:"name" gorm:"size:100;not null"`
	Description string     `json:"description,omitempty" gorm:"size:255"`
	IsActive    bool       `json:"is_active" gorm:"not null;default:true"`
	CreatedAt   time.Time  `json:"created_at" gorm:"not null;index"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// Room represents a game room.
type Room struct {
	ID           string     `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Code         string     `json:"code" gorm:"uniqueIndex;size:6;not null"`
	HostID       string     `json:"host_id" gorm:"type:varchar(64);not null;index"`
	GameTypeID   string     `json:"-" gorm:"type:varchar(64);index"`
	GameType     string     `json:"game_type,omitempty" gorm:"-"`
	MaxRounds    int        `json:"max_rounds" gorm:"not null"`
	TimePerRound int        `json:"time_per_round" gorm:"not null"`
	Status       RoomStatus `json:"status" gorm:"type:varchar(32);not null;index"`
	CreatedAt    time.Time  `json:"created_at" gorm:"not null;index"`
	ExpiresAt    time.Time  `json:"expires_at" gorm:"not null;index"`
}

// User represents a player in a room.
type User struct {
	ID           string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoomID       string    `json:"room_id" gorm:"type:varchar(64);not null;index"`
	Nickname     string    `json:"nickname" gorm:"size:50;not null"`
	AvatarURL    string    `json:"avatar_url" gorm:"size:255"`
	SessionToken string    `json:"-" gorm:"size:128;not null;uniqueIndex"`
	IsHost       bool      `json:"is_host" gorm:"not null;default:false"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null;index"`
}

// Round represents a game round.
type Round struct {
	ID               string      `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoomID           string      `json:"room_id" gorm:"type:varchar(64);not null;index"`
	RoundNumber      int         `json:"round_number" gorm:"not null"`
	Status           RoundStatus `json:"status" gorm:"type:varchar(32);not null;index"`
	ActiveTruthSetID string      `json:"active_truth_set_id,omitempty" gorm:"type:varchar(64);index"`
	StartedAt        time.Time   `json:"started_at" gorm:"not null;index"`
	PhaseEndsAt      *time.Time  `json:"phase_ends_at,omitempty"`
	PausedAt         *time.Time  `json:"paused_at,omitempty"`
	CompletedAt      *time.Time  `json:"completed_at,omitempty"`
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

// NormalizeGameTypeSlug returns the default slug when the provided value is blank.
func NormalizeGameTypeSlug(value string) string {
	if strings.TrimSpace(value) == "" {
		return GameTypeTellYourStory
	}

	return strings.TrimSpace(value)
}

// GameTypeSlugFromID resolves the public slug for a stored game type id.
func GameTypeSlugFromID(id string) string {
	switch strings.TrimSpace(id) {
	case "", GameTypeIDTellYourStory:
		return GameTypeTellYourStory
	case GameTypeIDThreeLiesOneTruth:
		return GameTypeThreeLiesOneTruth
	default:
		return GameTypeTellYourStory
	}
}

// IsThreeLiesOneTruthGameTypeID reports whether the provided id belongs to the new mode.
func IsThreeLiesOneTruthGameTypeID(id string) bool {
	return strings.TrimSpace(id) == GameTypeIDThreeLiesOneTruth
}
