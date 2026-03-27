package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormVoteRepository stores votes in PostgreSQL using GORM.
type GormVoteRepository struct {
	db *gorm.DB
}

// NewGormVoteRepository creates a GORM-backed vote repository.
func NewGormVoteRepository(db *gorm.DB) *GormVoteRepository {
	return &GormVoteRepository{db: db}
}

// Create stores a vote.
func (r *GormVoteRepository) Create(ctx context.Context, vote domain.Vote) error {
	if err := r.db.WithContext(ctx).Create(&vote).Error; err != nil {
		return fmt.Errorf("insert vote: %w", err)
	}

	return nil
}

// GetByUserAndRound returns the user's vote for the round if present.
func (r *GormVoteRepository) GetByUserAndRound(ctx context.Context, userID, roundID string) (domain.Vote, error) {
	var vote domain.Vote
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND round_id = ?", userID, roundID).
		First(&vote).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Vote{}, domain.ErrVoteNotFound
		}
		return domain.Vote{}, fmt.Errorf("select vote by user and round: %w", err)
	}

	return vote, nil
}

// ListByRoundID returns all votes in a round.
func (r *GormVoteRepository) ListByRoundID(ctx context.Context, roundID string) ([]domain.Vote, error) {
	var votes []domain.Vote
	if err := r.db.WithContext(ctx).
		Where("round_id = ?", roundID).
		Order("created_at asc").
		Find(&votes).Error; err != nil {
		return nil, fmt.Errorf("list votes by round: %w", err)
	}

	return votes, nil
}

var _ VoteRepository = (*GormVoteRepository)(nil)
