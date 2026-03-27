package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/tell-your-story/backend/internal/domain"
	"gorm.io/gorm"
)

// GormRoundRepository stores rounds in PostgreSQL using GORM.
type GormRoundRepository struct {
	db *gorm.DB
}

// NewGormRoundRepository creates a GORM-backed round repository.
func NewGormRoundRepository(db *gorm.DB) *GormRoundRepository {
	return &GormRoundRepository{db: db}
}

// Create stores a round.
func (r *GormRoundRepository) Create(ctx context.Context, round domain.Round) error {
	if err := r.db.WithContext(ctx).Create(&round).Error; err != nil {
		return fmt.Errorf("insert round: %w", err)
	}

	return nil
}

// GetByID fetches a round by id.
func (r *GormRoundRepository) GetByID(ctx context.Context, id string) (domain.Round, error) {
	var round domain.Round
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&round).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Round{}, domain.ErrRoundNotFound
		}
		return domain.Round{}, fmt.Errorf("select round by id: %w", err)
	}

	return round, nil
}

// GetCurrentByRoomID returns the latest round for a room.
func (r *GormRoundRepository) GetCurrentByRoomID(ctx context.Context, roomID string) (domain.Round, error) {
	var round domain.Round
	if err := r.db.WithContext(ctx).
		Where("room_id = ?", roomID).
		Order("round_number desc, started_at desc").
		First(&round).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return domain.Round{}, domain.ErrRoundNotFound
		}
		return domain.Round{}, fmt.Errorf("select current round by room: %w", err)
	}

	return round, nil
}

// Update overwrites a round.
func (r *GormRoundRepository) Update(ctx context.Context, round domain.Round) error {
	result := r.db.WithContext(ctx).Model(&domain.Round{}).Where("id = ?", round.ID).Select("*").Updates(&round)
	if result.Error != nil {
		return fmt.Errorf("update round: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrRoundNotFound
	}

	return nil
}

var _ RoundRepository = (*GormRoundRepository)(nil)
