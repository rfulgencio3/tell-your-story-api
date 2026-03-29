package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormTruthSetVoteRepository stores truth set votes in PostgreSQL using GORM.
type GormTruthSetVoteRepository struct {
	db *gorm.DB
}

// NewGormTruthSetVoteRepository creates a GORM-backed truth set vote repository.
func NewGormTruthSetVoteRepository(db *gorm.DB) *GormTruthSetVoteRepository {
	return &GormTruthSetVoteRepository{db: db}
}

// Upsert stores or updates one vote per truth_set_id/user_id pair.
func (r *GormTruthSetVoteRepository) Upsert(ctx context.Context, vote domain.TruthSetVote) error {
	existing, err := r.GetByTruthSetAndUser(ctx, vote.TruthSetID, vote.UserID)
	switch {
	case errors.Is(err, domain.ErrTruthSetVoteNotFound):
		if createErr := r.db.WithContext(ctx).Create(&vote).Error; createErr != nil {
			return fmt.Errorf("insert truth set vote: %w", createErr)
		}
		return nil
	case err != nil:
		return err
	default:
		vote.ID = existing.ID
		vote.CreatedAt = existing.CreatedAt
		result := r.db.WithContext(ctx).Model(&domain.TruthSetVote{}).Where("id = ?", existing.ID).Select("*").Updates(&vote)
		if result.Error != nil {
			return fmt.Errorf("update truth set vote: %w", result.Error)
		}
		return nil
	}
}

// GetByTruthSetAndUser returns the user's vote for a truth set if present.
func (r *GormTruthSetVoteRepository) GetByTruthSetAndUser(ctx context.Context, truthSetID, userID string) (domain.TruthSetVote, error) {
	var vote domain.TruthSetVote
	if err := r.db.WithContext(ctx).
		Where("truth_set_id = ? AND user_id = ?", truthSetID, userID).
		First(&vote).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.TruthSetVote{}, domain.ErrTruthSetVoteNotFound
		}
		return domain.TruthSetVote{}, fmt.Errorf("select truth set vote by truth set and user: %w", err)
	}

	return vote, nil
}

// ListByTruthSetID returns all votes cast for a truth set.
func (r *GormTruthSetVoteRepository) ListByTruthSetID(ctx context.Context, truthSetID string) ([]domain.TruthSetVote, error) {
	var votes []domain.TruthSetVote
	if err := r.db.WithContext(ctx).
		Where("truth_set_id = ?", truthSetID).
		Order("created_at asc").
		Find(&votes).Error; err != nil {
		return nil, fmt.Errorf("list truth set votes by truth set: %w", err)
	}

	return votes, nil
}

var _ TruthSetVoteRepository = (*GormTruthSetVoteRepository)(nil)
