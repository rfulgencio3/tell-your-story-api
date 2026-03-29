package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/tell-your-story/backend/internal/domain"
	"github.com/tell-your-story/backend/pkg/utils"
	"gorm.io/gorm"
)

// GormRoomScoreRepository stores room scores in PostgreSQL using GORM.
type GormRoomScoreRepository struct {
	db *gorm.DB
}

// NewGormRoomScoreRepository creates a GORM-backed room score repository.
func NewGormRoomScoreRepository(db *gorm.DB) *GormRoomScoreRepository {
	return &GormRoomScoreRepository{db: db}
}

// Increment creates or updates one room score row for the provided room/user pair.
func (r *GormRoomScoreRepository) Increment(ctx context.Context, roomID, userID string, delta int, updatedAt time.Time) (domain.RoomScore, error) {
	var score domain.RoomScore
	err := r.db.WithContext(ctx).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		First(&score).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		scoreID, idErr := utils.GenerateID()
		if idErr != nil {
			return domain.RoomScore{}, fmt.Errorf("generate room score id: %w", idErr)
		}
		score = domain.RoomScore{
			ID:        scoreID,
			RoomID:    roomID,
			UserID:    userID,
			Score:     delta,
			UpdatedAt: updatedAt,
		}
		if createErr := r.db.WithContext(ctx).Create(&score).Error; createErr != nil {
			return domain.RoomScore{}, fmt.Errorf("insert room score: %w", createErr)
		}
		return score, nil
	case err != nil:
		return domain.RoomScore{}, fmt.Errorf("select room score: %w", err)
	}

	score.Score += delta
	score.UpdatedAt = updatedAt
	result := r.db.WithContext(ctx).Model(&domain.RoomScore{}).Where("id = ?", score.ID).Select("*").Updates(&score)
	if result.Error != nil {
		return domain.RoomScore{}, fmt.Errorf("update room score: %w", result.Error)
	}

	return score, nil
}

// ListByRoomID returns all persisted scores for a room.
func (r *GormRoomScoreRepository) ListByRoomID(ctx context.Context, roomID string) ([]domain.RoomScore, error) {
	var scores []domain.RoomScore
	if err := r.db.WithContext(ctx).
		Where("room_id = ?", roomID).
		Order("score desc, updated_at asc").
		Find(&scores).Error; err != nil {
		return nil, fmt.Errorf("list room scores by room: %w", err)
	}

	return scores, nil
}

var _ RoomScoreRepository = (*GormRoomScoreRepository)(nil)
