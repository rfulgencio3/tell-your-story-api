package domain

import "time"

// TruthSet stores one player's four statements for a round.
type TruthSet struct {
	ID                 string              `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoomID             string              `json:"room_id" gorm:"type:varchar(64);not null;index"`
	RoundID            string              `json:"round_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_truth_set_round_author"`
	AuthorUserID       string              `json:"author_user_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_truth_set_round_author"`
	PresentationOrder  int                 `json:"presentation_order" gorm:"not null;default:0;uniqueIndex:idx_truth_set_round_order"`
	TrueStatementIndex int                 `json:"true_statement_index" gorm:"not null"`
	CommentaryText     string              `json:"commentary_text,omitempty" gorm:"size:500"`
	ScoredAt           *time.Time          `json:"-" gorm:"index"`
	CreatedAt          time.Time           `json:"created_at" gorm:"not null;index"`
	UpdatedAt          time.Time           `json:"updated_at" gorm:"not null;index"`
	Statements         []TruthSetStatement `json:"statements,omitempty" gorm:"foreignKey:TruthSetID;constraint:OnDelete:CASCADE"`
}

// TruthSetStatement stores one normalized statement belonging to a truth set.
type TruthSetStatement struct {
	ID             string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	TruthSetID     string    `json:"truth_set_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_truth_set_statement_order"`
	StatementIndex int       `json:"statement_index" gorm:"not null;uniqueIndex:idx_truth_set_statement_order"`
	Content        string    `json:"content" gorm:"size:500;not null"`
	CreatedAt      time.Time `json:"created_at" gorm:"not null;index"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"not null;index"`
}

// TruthSetVote stores one player's current vote for a presented truth set.
type TruthSetVote struct {
	ID                     string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoomID                 string    `json:"room_id" gorm:"type:varchar(64);not null;index"`
	RoundID                string    `json:"round_id" gorm:"type:varchar(64);not null;index"`
	TruthSetID             string    `json:"truth_set_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_truth_set_vote_user"`
	UserID                 string    `json:"user_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_truth_set_vote_user"`
	SelectedStatementIndex int       `json:"selected_statement_index" gorm:"not null"`
	CreatedAt              time.Time `json:"created_at" gorm:"not null;index"`
	UpdatedAt              time.Time `json:"updated_at" gorm:"not null;index"`
}

// RoomScore stores the accumulated score for one participant within a room.
type RoomScore struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(64)"`
	RoomID    string    `json:"room_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_room_score_user"`
	UserID    string    `json:"user_id" gorm:"type:varchar(64);not null;index;uniqueIndex:idx_room_score_user"`
	Score     int       `json:"score" gorm:"not null;default:0"`
	UpdatedAt time.Time `json:"updated_at" gorm:"not null;index"`
}
